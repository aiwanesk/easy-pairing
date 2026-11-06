package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"pairingeasy/internal/auth"
	"pairingeasy/internal/chess"
	"pairingeasy/internal/store"
)

// Permission levels.
const (
	levelNone = iota
	levelViewer
	levelResults // read-only, may enter results
	levelArbiter
	levelOwner
)

func levelValue(l string) int {
	switch l {
	case "owner", "admin":
		return levelOwner
	case "arbiter":
		return levelArbiter
	case "results":
		return levelResults
	case "viewer":
		return levelViewer
	}
	return levelNone
}

func (s *Server) level(a *store.Account, eventID string) int {
	if a.Role == "admin" {
		return levelOwner
	}
	return levelValue(s.Store.Level(a.ID, eventID))
}

func (s *Server) require(r *http.Request, eventID string, need int) error {
	l := s.level(accountFrom(r), eventID)
	if l == levelNone {
		return fail(http.StatusNotFound, "événement introuvable")
	}
	if l < need {
		return fail(http.StatusForbidden, "droits insuffisants sur cet événement")
	}
	return nil
}

// TournamentDerived carries computed data the UI needs for a tournament.
type TournamentDerived struct {
	PairingNumbers map[int]int     `json:"pairingNumbers"`
	Scores         map[int]float64 `json:"scores"`
	PlannedRounds  int             `json:"plannedRounds"`
	LastPublished  int             `json:"lastPublished"`
	CanPair        bool            `json:"canPair"`
	CanPairReason  string          `json:"canPairReason,omitempty"`
	Pending        int             `json:"pendingRegistrations"`
	CheckedIn      int             `json:"checkedIn"`
}

// EventView is the admin view of an event.
type EventView struct {
	*chess.Event
	Level   string                     `json:"level"`
	Derived map[int]*TournamentDerived `json:"derived"`
}

func derive(t *chess.Tournament) *TournamentDerived {
	d := &TournamentDerived{PairingNumbers: t.PairingNumbers(), Scores: t.Scores(len(t.Rounds)),
		PlannedRounds: t.PlannedRounds(), LastPublished: t.LastPublished()}
	for _, p := range t.Players {
		if p.Status == chess.StatusPending {
			d.Pending++
		}
		if p.Active() && p.CheckedIn {
			d.CheckedIn++
		}
	}
	if t.IsTeam() {
		if err := t.CanPairTeams(); err != nil {
			d.CanPairReason = err.Error()
		} else {
			d.CanPair = true
		}
		d.PlannedRounds = t.PlannedRounds()
		if t.Kind == chess.KindRoundRobin && !t.Started() {
			n := len(t.Teams)
			if n%2 == 1 {
				n++
			}
			d.PlannedRounds = n - 1
			if t.DoubleRR {
				d.PlannedRounds *= 2
			}
		}
		return d
	}
	if t.Kind == chess.KindKnockout {
		switch {
		case !t.Started() && t.PlannedRounds() == 0:
			d.CanPairReason = "il faut au moins deux joueurs (pointés) pour commencer"
		case !t.Started():
			d.CanPair = true
		default:
			if err := t.KOCanAdvance(); err != nil {
				d.CanPairReason = err.Error()
			} else {
				d.CanPair = true
			}
		}
		return d
	}
	if t.Kind == chess.KindSwiss {
		if _, err := t.PrepareNextSwissRound(); err != nil {
			d.CanPairReason = err.Error()
		} else {
			d.CanPair = true
		}
	} else if !t.Started() {
		if t.PlannedRounds() == 0 {
			d.CanPairReason = "il faut au moins deux joueurs (pointés) pour commencer"
		} else {
			d.CanPair = true
		}
	} else {
		d.CanPairReason = "toutes les rondes du toutes-rondes sont créées"
	}
	return d
}

func (s *Server) view(r *http.Request, e *chess.Event) EventView {
	a := accountFrom(r)
	lvl := "admin"
	if a.Role != "admin" {
		lvl = s.Store.Level(a.ID, e.ID)
	}
	v := EventView{Event: e, Level: lvl, Derived: map[int]*TournamentDerived{}}
	for _, t := range e.Tournaments {
		v.Derived[t.ID] = derive(t)
	}
	return v
}

// mutate applies fn to an event (with the lock held), records the audit
// entry, notifies live screens and returns the new admin view.
func (s *Server) mutate(w http.ResponseWriter, r *http.Request, need int, action string,
	fn func(e *chess.Event, by string) (tid int, target, details string, err error)) {
	eid := r.PathValue("eid")
	if err := s.require(r, eid, need); err != nil {
		writeAnyErr(w, err)
		return
	}
	var tid int
	var target, details string
	by := accountFrom(r).Name
	e, err := s.Store.UpdateEvent(r.Context(), eid, func(e *chess.Event) error {
		var err error
		tid, target, details, err = fn(e, by)
		return err
	})
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, eid, tid, action, target, details)
	s.hub.publish(eid)
	writeJSON(w, http.StatusOK, s.view(r, e))
}

func tournamentOf(e *chess.Event, r *http.Request) (*chess.Tournament, error) {
	tid, err := pathInt(r, "tid")
	if err != nil {
		return nil, err
	}
	t := e.Tournament(tid)
	if t == nil {
		return nil, fail(http.StatusNotFound, "tournoi introuvable")
	}
	return t, nil
}

// ---------------------------------------------------------------- events

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,60}$`)

var accents = map[rune]rune{
	'à': 'a', 'â': 'a', 'ä': 'a', 'á': 'a', 'ã': 'a', 'å': 'a', 'ç': 'c', 'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'î': 'i', 'ï': 'i', 'í': 'i', 'ì': 'i', 'ô': 'o', 'ö': 'o', 'ó': 'o', 'ò': 'o', 'õ': 'o', 'ø': 'o',
	'ù': 'u', 'û': 'u', 'ü': 'u', 'ú': 'u', 'ÿ': 'y', 'ý': 'y', 'ñ': 'n', 'œ': 'o', 'æ': 'a',
}

// Slugify turns a name into a URL-safe slug.
func Slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if f, ok := accents[r]; ok {
			r = f
		}
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 50 {
		out = strings.Trim(out[:50], "-")
	}
	if len(out) < 2 {
		out = "evenement-" + auth.RandomID(3)
	}
	return out
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	a := accountFrom(r)
	list, err := s.Store.Events(a.ID, a.Role == "admin")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if list == nil {
		list = []store.EventSummary{}
	}
	writeJSON(w, http.StatusOK, list)
}

type eventInput struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Location  string `json:"location"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Organizer string `json:"organizer"`
	Contact   string `json:"contact"`
	Public    bool   `json:"public"`
	Message   string `json:"message"`
}

func (in *eventInput) apply(e *chess.Event) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return errors.New("le nom de l'événement est obligatoire")
	}
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	if in.Slug == "" {
		in.Slug = Slugify(in.Name)
	}
	if !slugRe.MatchString(in.Slug) {
		return errors.New("adresse publique invalide : lettres minuscules, chiffres et tirets")
	}
	e.Name, e.Slug, e.Location, e.StartDate, e.EndDate = in.Name, in.Slug, strings.TrimSpace(in.Location), in.StartDate, in.EndDate
	e.Organizer, e.Contact, e.Public, e.Message = strings.TrimSpace(in.Organizer), strings.TrimSpace(in.Contact), in.Public, strings.TrimSpace(in.Message)
	return nil
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	if a := accountFrom(r); a.Role != "admin" && a.Role != "manager" {
		writeErr(w, http.StatusForbidden, errors.New("seuls les administrateurs et les gestionnaires peuvent créer un événement"))
		return
	}
	var in eventInput
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	e := chess.NewEvent(auth.RandomID(6), "", "")
	if err := in.apply(e); err != nil {
		writeAnyErr(w, err)
		return
	}
	if err := s.Store.CreateEvent(e, accountFrom(r).ID); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, e.ID, 0, "event.create", e.Name, e.Slug)
	writeJSON(w, http.StatusCreated, s.view(r, e))
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request) {
	eid := r.PathValue("eid")
	if err := s.require(r, eid, levelViewer); err != nil {
		writeAnyErr(w, err)
		return
	}
	e, err := s.Store.Event(eid)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.view(r, e))
}

func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request) {
	var in eventInput
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelOwner, "event.update", func(e *chess.Event, by string) (int, string, string, error) {
		return 0, in.Name, fmt.Sprintf("public=%v slug=%s", in.Public, in.Slug), in.apply(e)
	})
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request) {
	eid := r.PathValue("eid")
	if err := s.require(r, eid, levelOwner); err != nil {
		writeAnyErr(w, err)
		return
	}
	e, err := s.Store.Event(eid)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if err := s.Store.DeleteEvent(eid); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, eid, 0, "event.delete", e.Name, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listPermissions(w http.ResponseWriter, r *http.Request) {
	eid := r.PathValue("eid")
	if err := s.require(r, eid, levelViewer); err != nil {
		writeAnyErr(w, err)
		return
	}
	list, err := s.Store.Permissions(eid)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) setPermission(w http.ResponseWriter, r *http.Request) {
	eid := r.PathValue("eid")
	if err := s.require(r, eid, levelOwner); err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		AccountID int    `json:"accountId"`
		Level     string `json:"level"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	target, err := s.Store.Account(in.AccountID)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	cur := s.Store.Level(in.AccountID, eid)
	if cur == "owner" && in.Level != "owner" && s.Store.CountOwners(eid) <= 1 {
		writeErr(w, http.StatusUnprocessableEntity, errors.New("l'événement doit garder au moins un propriétaire"))
		return
	}
	if err := s.Store.SetPermission(in.AccountID, eid, in.Level); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, eid, 0, "event.permission", target.Login, fmt.Sprintf("%s → %s", orDash(cur), orDash(in.Level)))
	list, _ := s.Store.Permissions(eid)
	writeJSON(w, http.StatusOK, list)
}

func orDash(s string) string {
	if s == "" {
		return "aucun accès"
	}
	return s
}

// ---------------------------------------------------------------- tournaments

type tournamentInput struct {
	Name          string              `json:"name"`
	Kind          string              `json:"kind"`
	System        string              `json:"system"`
	Acceleration  string              `json:"acceleration"`
	DoubleRR      bool                `json:"doubleRoundRobin"`
	ReverseLastRR bool                `json:"reverseLastRounds"`
	TotalRounds   int                 `json:"totalRounds"`
	InitialColor  string              `json:"initialColor"`
	RatingKind    string              `json:"ratingKind"`
	TimeControl   string              `json:"timeControl"`
	PointsWin     float64             `json:"pointsWin"`
	PointsDraw    float64             `json:"pointsDraw"`
	PointsPAB     float64             `json:"pointsPab"`
	TieBreaks     []string            `json:"tieBreaks"`
	UnratedValue  int                 `json:"unratedValue"`
	FIDEMode      bool                `json:"fideMode"`
	CheckIn       bool                `json:"checkIn"`
	KioskPIN      string              `json:"kioskPin"`
	KioskResults  bool                `json:"kioskResults"`
	Registration  chess.Registration  `json:"registration"`
	ChiefArbiter  string              `json:"chiefArbiter"`
	Federation    string              `json:"federation"`
	ArbiterNotes  string              `json:"arbiterNotes"`
	PrizeSplit    string              `json:"prizeSplit"`
	NormsFrom     int                 `json:"normsFrom"`
	KO            *chess.KOSettings   `json:"ko"`
	Team          *chess.TeamSettings `json:"team"`
}

var pinRe = regexp.MustCompile(`^[0-9]{4,8}$`)

// apply validates and applies settings; changes FIDE regulations forbid
// once started are refused, others are logged in the tournament log.
func (in *tournamentInput) apply(t *chess.Tournament, by string) (string, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return "", errors.New("le nom du tournoi est obligatoire")
	}
	if in.KioskPIN != "" && !pinRe.MatchString(in.KioskPIN) {
		return "", errors.New("le code du kiosque doit faire 4 à 8 chiffres")
	}
	started := t.Started()
	var changes []string
	if started {
		if in.Kind != t.Kind {
			return "", errors.New("le type de tournoi ne peut plus changer")
		}
		if in.PointsWin != t.PointsWin || in.PointsDraw != t.PointsDraw || in.PointsPAB != t.PointsPAB {
			return "", errors.New("le barème ne peut plus changer une fois le tournoi commencé")
		}
		if in.InitialColor != t.InitialColor {
			return "", errors.New("la couleur initiale ne peut plus changer")
		}
		if t.Kind == chess.KindSwiss && in.TotalRounds < len(t.Rounds) {
			return "", fmt.Errorf("%d rondes sont déjà appariées", len(t.Rounds))
		}
		if t.Kind == chess.KindRoundRobin && (in.DoubleRR != t.DoubleRR || in.ReverseLastRR != t.ReverseLastRR) {
			return "", errors.New("le calendrier du toutes-rondes est déjà créé")
		}
		if t.FIDEMode && !in.FIDEMode {
			return "", errors.New("le mode FIDE est définitif une fois la première ronde appariée")
		}
		if in.System != t.System || in.Acceleration != t.Acceleration {
			if t.FIDEMode {
				return "", errors.New("mode FIDE : système et accélération sont fixés")
			}
			changes = append(changes, fmt.Sprintf("système %s/%s → %s/%s", t.System, t.Acceleration, in.System, in.Acceleration))
		}
		if in.TotalRounds != t.TotalRounds && t.Kind == chess.KindSwiss {
			changes = append(changes, fmt.Sprintf("nombre de rondes %d → %d", t.TotalRounds, in.TotalRounds))
		}
		if strings.Join(in.TieBreaks, ",") != strings.Join(t.TieBreaks, ",") {
			changes = append(changes, fmt.Sprintf("départages %v → %v", t.TieBreaks, in.TieBreaks))
		}
	}
	t.Name, t.Kind, t.System, t.Acceleration = in.Name, in.Kind, in.System, in.Acceleration
	t.DoubleRR, t.ReverseLastRR = in.DoubleRR, in.ReverseLastRR
	if t.Kind == chess.KindSwiss {
		t.TotalRounds = in.TotalRounds
	}
	t.InitialColor, t.RatingKind, t.TimeControl = in.InitialColor, in.RatingKind, strings.TrimSpace(in.TimeControl)
	t.PointsWin, t.PointsDraw, t.PointsPAB = in.PointsWin, in.PointsDraw, in.PointsPAB
	t.TieBreaks, t.UnratedValue, t.FIDEMode, t.CheckIn = in.TieBreaks, in.UnratedValue, in.FIDEMode, in.CheckIn
	t.KioskPIN, t.KioskResults, t.Registration = in.KioskPIN, in.KioskResults, in.Registration
	t.ChiefArbiter, t.Federation, t.ArbiterNotes = strings.TrimSpace(in.ChiefArbiter), strings.ToUpper(strings.TrimSpace(in.Federation)), in.ArbiterNotes
	if in.Team != nil {
		if started && in.Team.Boards != t.TeamCfg.Boards {
			return "", errors.New("tournoi commencé : le nombre d'échiquiers par match ne peut plus changer")
		}
		if in.Team.Boards < 0 || in.Team.Boards > 20 {
			return "", errors.New("nombre d'échiquiers par équipe invalide (1 à 20)")
		}
		if in.Team.Boards > 0 && in.Kind == chess.KindKnockout {
			return "", errors.New("la coupe par équipes n'est pas encore disponible : choisissez suisse ou toutes-rondes")
		}
		t.TeamCfg = *in.Team
	}
	if in.KO != nil {
		if started && (in.KO.Games != t.KO.Games || in.KO.Seeding != t.KO.Seeding) {
			return "", errors.New("coupe commencée : le nombre de parties par match et le placement ne peuvent plus changer")
		}
		t.KO = *in.KO
	}
	if in.NormsFrom > 0 {
		t.NormsFrom = in.NormsFrom
	}
	if in.PrizeSplit != "" {
		if started && in.PrizeSplit != t.PrizeSplit {
			changes = append(changes, fmt.Sprintf("répartition des prix %s → %s", t.PrizeSplit, in.PrizeSplit))
		}
		t.PrizeSplit = in.PrizeSplit
	}
	t.Normalize()
	if len(changes) > 0 {
		t.AddLog("settings", len(t.Rounds), by, "%s", strings.Join(changes, " ; "))
	}
	return strings.Join(changes, " ; "), nil
}

func (s *Server) createTournament(w http.ResponseWriter, r *http.Request) {
	var in tournamentInput
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "tournament.create", func(e *chess.Event, by string) (int, string, string, error) {
		t := chess.NewTournament(in.Name)
		if in.PointsWin == 0 {
			in.PointsWin, in.PointsDraw, in.PointsPAB = 1, 0.5, 1
		}
		if len(in.TieBreaks) == 0 {
			if in.Kind == chess.KindRoundRobin {
				in.TieBreaks = chess.DefaultRRTieBreaks
			} else {
				in.TieBreaks = chess.DefaultSwissTieBreaks
			}
		}
		if _, err := in.apply(t, by); err != nil {
			return 0, "", "", err
		}
		e.AddTournament(t)
		return t.ID, t.Name, t.Kind, nil
	})
}

func (s *Server) updateTournament(w http.ResponseWriter, r *http.Request) {
	var in tournamentInput
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	// Before round 1, a change of rating kind (lent / rapide / blitz)
	// reloads every player's rating of that kind.
	var src ratingSources
	if e, err := s.Store.Event(r.PathValue("eid")); err == nil {
		if t, err := tournamentOf(e, r); err == nil && !t.Started() && in.RatingKind != "" && in.RatingKind != t.RatingKind {
			src = s.collectRatingSources(r.Context(), t.Players)
		}
	}
	s.mutate(w, r, levelArbiter, "tournament.update", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		oldKind := t.RatingKind
		changes, err := in.apply(t, by)
		if err != nil {
			return 0, "", "", err
		}
		if src != nil && t.RatingKind != oldKind && !t.Started() {
			changed, unknown, lines := applyRatings(t, t.RatingKind, src)
			msg := fmt.Sprintf("Elo utilisé %s → %s : %d Elo mis à jour, %d joueur(s) introuvable(s)",
				ratingKindLabel[oldKind], ratingKindLabel[t.RatingKind], changed, unknown)
			if len(lines) > 0 {
				msg += " — " + strings.Join(lines, " ; ")
			}
			t.AddLog("settings", 0, by, "%s", msg)
			if changes != "" {
				changes += " ; "
			}
			changes += msg
		}
		return t.ID, t.Name, changes, nil
	})
}

func (s *Server) deleteTournament(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, levelOwner, "tournament.delete", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		return t.ID, t.Name, "", e.RemoveTournament(t.ID)
	})
}

// ---------------------------------------------------------------- players

func (s *Server) addPlayer(w http.ResponseWriter, r *http.Request) {
	var p chess.Player
	if err := decode(w, r, &p); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "player.add", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		p.Source = "arbiter"
		p.Status = chess.StatusConfirmed
		p.CheckedIn, p.CheckedInAt = false, nil
		if err := t.AddPlayer(&p); err != nil {
			return 0, "", "", err
		}
		return t.ID, p.FullName(), fmt.Sprintf("Elo %d, FFE %s, FIDE %s", p.Rating, p.FFEID, p.FideID), nil
	})
}

func (s *Server) updatePlayer(w http.ResponseWriter, r *http.Request) {
	pid, err := pathInt(r, "pid")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in chess.Player
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "player.update", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		p := t.Player(pid)
		if p == nil {
			return 0, "", "", errors.New("joueur introuvable")
		}
		chess.CleanPlayer(&in)
		if in.LastName == "" {
			return 0, "", "", errors.New("le nom du joueur est obligatoire")
		}
		paired := len(t.Rounds)
		for round, code := range in.Byes {
			if round <= paired && code != p.Byes[round] {
				return 0, "", "", fmt.Errorf("la ronde %d est déjà appariée", round)
			}
			if code == chess.ByeHalf && in.ByeIneligible {
				return 0, "", "", errors.New("ce joueur n'a pas droit aux byes (FIDE C.05:6.7.4)")
			}
		}
		if in.Withdrawn > 0 && in.Withdrawn <= paired && in.Withdrawn != p.Withdrawn {
			in.Withdrawn = paired + 1
		}
		if t.Started() && in.Rating != p.Rating {
			t.AddLog("correction", paired, by, "Elo de %s : %d → %d (l'ordre initial peut changer)", p.FullName(), p.Rating, in.Rating)
		}
		// fields managed elsewhere
		in.ID, in.Status, in.Source, in.RegisteredAt = p.ID, p.Status, p.Source, p.RegisteredAt
		in.CheckedIn, in.CheckedInAt, in.BergerNo = p.CheckedIn, p.CheckedInAt, p.BergerNo
		if q := t.DuplicateOf(&in); q != nil {
			return 0, "", "", fmt.Errorf("%s porte déjà ce code FFE/FIDE", q.FullName())
		}
		*p = in
		return t.ID, p.FullName(), fmt.Sprintf("retrait=%d byes=%v", p.Withdrawn, p.Byes), nil
	})
}

// setBye sets or clears the absence of a player for one round not paired
// yet: H (bye ½), F (bye 1), Z (absent, 0), "" (present). Withdraw marks the
// player absent from that round on.
func (s *Server) setBye(w http.ResponseWriter, r *http.Request) {
	pid, err := pathInt(r, "pid")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Round    int    `json:"round"`
		Code     string `json:"code"`
		Withdraw bool   `json:"withdraw"`
		Return   bool   `json:"return"` // cancel a withdrawal
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "player.bye", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		p := t.Player(pid)
		if p == nil {
			return 0, "", "", errors.New("joueur introuvable")
		}
		paired := len(t.Rounds)
		if in.Round <= paired {
			return 0, "", "", fmt.Errorf("la ronde %d est déjà appariée : supprimez son appariement pour changer les présents", in.Round)
		}
		if t.Kind == chess.KindSwiss && in.Round > t.TotalRounds {
			return 0, "", "", fmt.Errorf("le tournoi n'a que %d rondes", t.TotalRounds)
		}
		switch {
		case in.Return:
			if p.Withdrawn == 0 {
				return t.ID, p.FullName(), "", nil
			}
			p.Withdrawn = 0
			t.AddLog("settings", paired, by, "%s réintégré à partir de la ronde %d", p.FullName(), paired+1)
			return t.ID, p.FullName(), "réintégré", nil
		case in.Withdraw:
			p.Withdrawn = in.Round
			t.AddLog("settings", paired, by, "%s retiré à partir de la ronde %d", p.FullName(), in.Round)
			return t.ID, p.FullName(), fmt.Sprintf("retiré à partir de la ronde %d", in.Round), nil
		}
		switch in.Code {
		case "":
			delete(p.Byes, in.Round)
		case chess.ByeHalf, chess.ByeZero, chess.ByeFull:
			if in.Code == chess.ByeHalf && p.ByeIneligible {
				return 0, "", "", errors.New("ce joueur n'a pas droit aux byes ½ (FIDE C.05:6.7.4)")
			}
			if p.Byes == nil {
				p.Byes = map[int]string{}
			}
			p.Byes[in.Round] = in.Code
		default:
			return 0, "", "", fmt.Errorf("code d'absence inconnu %q", in.Code)
		}
		return t.ID, p.FullName(), fmt.Sprintf("ronde %d : %s", in.Round, map[string]string{"": "présent", "H": "bye ½", "F": "bye 1", "Z": "absent"}[in.Code]), nil
	})
}

func (s *Server) deletePlayer(w http.ResponseWriter, r *http.Request) {
	pid, err := pathInt(r, "pid")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "player.delete", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		p := t.Player(pid)
		if p == nil {
			return 0, "", "", errors.New("joueur introuvable")
		}
		return t.ID, p.FullName(), "", t.RemovePlayer(pid)
	})
}

func (s *Server) checkIn(w http.ResponseWriter, r *http.Request) {
	pid, err := pathInt(r, "pid")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		CheckedIn bool `json:"checkedIn"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "player.checkin", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		p := t.Player(pid)
		if p == nil {
			return 0, "", "", errors.New("joueur introuvable")
		}
		setCheckIn(p, in.CheckedIn)
		return t.ID, p.FullName(), fmt.Sprintf("pointé=%v", in.CheckedIn), nil
	})
}

func setCheckIn(p *chess.Player, v bool) {
	p.CheckedIn = v
	if v {
		now := time.Now().UTC()
		p.CheckedInAt = &now
	} else {
		p.CheckedInAt = nil
	}
}

func (s *Server) checkInAll(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CheckedIn bool `json:"checkedIn"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "player.checkin.all", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		for _, p := range t.Participants() {
			setCheckIn(p, in.CheckedIn)
		}
		return t.ID, t.Name, fmt.Sprintf("tous pointés=%v", in.CheckedIn), nil
	})
}

func (s *Server) setPlayerStatus(w http.ResponseWriter, r *http.Request) {
	pid, err := pathInt(r, "pid")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "registration."+in.Status, func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		p := t.Player(pid)
		if p == nil {
			return 0, "", "", errors.New("joueur introuvable")
		}
		switch in.Status {
		case chess.StatusConfirmed, chess.StatusPending, chess.StatusRefused:
		default:
			return 0, "", "", errors.New("statut invalide")
		}
		if p.Status == chess.StatusConfirmed && in.Status != chess.StatusConfirmed && t.HasPlayed(p.ID) {
			return 0, "", "", errors.New("ce joueur a déjà été apparié")
		}
		if t.Kind == chess.KindRoundRobin && t.Started() && in.Status == chess.StatusConfirmed && p.Status != chess.StatusConfirmed {
			return 0, "", "", errors.New("toutes-rondes déjà commencé")
		}
		p.Status = in.Status
		return t.ID, p.FullName(), in.Status, nil
	})
}

func (s *Server) setForbidden(w http.ResponseWriter, r *http.Request) {
	var in [][]int
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "tournament.forbidden", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if t.FIDEMode && len(t.Rounds) >= 1 {
			return 0, "", "", errors.New("mode FIDE : les appariements interdits sont fixés après la première ronde")
		}
		var groups [][]int
		for _, g := range in {
			var ids []int
			seen := map[int]bool{}
			for _, id := range g {
				if t.Player(id) != nil && !seen[id] {
					ids = append(ids, id)
					seen[id] = true
				}
			}
			if len(ids) >= 2 {
				groups = append(groups, ids)
			}
		}
		t.ForbiddenGroups = groups
		t.AddLog("settings", len(t.Rounds), by, "appariements interdits : %d groupe(s)", len(groups))
		return t.ID, t.Name, fmt.Sprint(groups), nil
	})
}

// prizesInput is the prize list and the split mode. A bare array (former
// format) is still accepted.
type prizesInput struct {
	Split  string        `json:"split"`
	Prizes []chess.Prize `json:"prizes"`
}

func (s *Server) setPrizes(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if err := decode(w, r, &raw); err != nil {
		writeAnyErr(w, err)
		return
	}
	var in prizesInput
	if t := bytes.TrimSpace(raw); len(t) > 0 && t[0] == '[' {
		if err := json.Unmarshal(raw, &in.Prizes); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	} else if err := json.Unmarshal(raw, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mutate(w, r, levelArbiter, "tournament.prizes", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		for i := range in.Prizes {
			pr := &in.Prizes[i]
			pr.Label = strings.TrimSpace(pr.Label)
			pr.Club = strings.TrimSpace(pr.Club)
			pr.Category = strings.Join(splitList(pr.Category), ",")
			pr.Fed = strings.ToUpper(strings.Join(splitList(pr.Fed), ","))
			if pr.Label == "" {
				return 0, "", "", fmt.Errorf("prix n°%d : libellé obligatoire", i+1)
			}
			if pr.Amount < 0 {
				return 0, "", "", fmt.Errorf("prix « %s » : montant négatif", pr.Label)
			}
			if pr.MinRating > 0 && pr.MaxRating > 0 && pr.MinRating > pr.MaxRating {
				return 0, "", "", fmt.Errorf("prix « %s » : Elo minimum %d supérieur à l'Elo maximum %d", pr.Label, pr.MinRating, pr.MaxRating)
			}
			if pr.Sex != "f" && pr.Sex != "m" {
				pr.Sex = ""
			}
			if pr.ID == 0 {
				pr.ID = t.NextPrizeID
				t.NextPrizeID++
			}
		}
		if in.Prizes == nil {
			in.Prizes = []chess.Prize{}
		}
		old := t.PrizeSplit
		t.Prizes = in.Prizes
		if in.Split != "" {
			t.PrizeSplit = in.Split
		}
		t.Normalize()
		details := fmt.Sprintf("%d prix, répartition %s", len(in.Prizes), t.PrizeSplit)
		if old != t.PrizeSplit && t.Started() {
			t.AddLog("settings", len(t.Rounds), by, "répartition des prix %s → %s", old, t.PrizeSplit)
		}
		return t.ID, t.Name, details, nil
	})
}

func splitList(s string) []string {
	var out []string
	for _, x := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func (s *Server) loadTournament(w http.ResponseWriter, r *http.Request) (*chess.Event, *chess.Tournament, bool) {
	eid := r.PathValue("eid")
	if err := s.require(r, eid, levelViewer); err != nil {
		writeAnyErr(w, err)
		return nil, nil, false
	}
	e, err := s.Store.Event(eid)
	if err != nil {
		writeAnyErr(w, err)
		return nil, nil, false
	}
	t, err := tournamentOf(e, r)
	if err != nil {
		writeAnyErr(w, err)
		return nil, nil, false
	}
	return e, t, true
}

func (s *Server) getPrizes(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.loadTournament(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, t.AllocatePrizes(roundParam(r, len(t.Rounds))))
}

func (s *Server) norms(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.loadTournament(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, t.Norms(roundParam(r, len(t.Rounds))))
}

func roundParam(r *http.Request, max int) int {
	n := max
	if q := r.URL.Query().Get("round"); q != "" {
		var v int
		if _, err := fmt.Sscanf(q, "%d", &v); err == nil && v >= 0 && v < max {
			n = v
		}
	}
	return n
}

func (s *Server) standings(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.loadTournament(w, r)
	if !ok {
		return
	}
	n := roundParam(r, len(t.Rounds))
	if t.Kind == chess.KindKnockout {
		writeJSON(w, http.StatusOK, map[string]any{"round": n, "standings": []chess.Standing{}, "ko": t.KOStandings()})
		return
	}
	if t.IsTeam() {
		writeJSON(w, http.StatusOK, map[string]any{"round": n, "standings": []chess.Standing{}, "teams": t.TeamStandings(n)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"round": n, "standings": t.Standings(n)})
}
