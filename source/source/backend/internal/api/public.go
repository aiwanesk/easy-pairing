package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pairingeasy/internal/chess"
	"pairingeasy/internal/lookup"
	"pairingeasy/internal/store"
)

// PublicPlayer exposes no contact data.
type PublicPlayer struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	LastName   string  `json:"lastName"`
	FirstName  string  `json:"firstName"`
	Rating     int     `json:"rating"`
	RatingType string  `json:"ratingType"`
	Fed        string  `json:"fed"`
	Club       string  `json:"club"`
	Category   string  `json:"category"`
	Status     string  `json:"status"`
	Withdrawn  int     `json:"withdrawnFrom"`
	CheckedIn  bool    `json:"checkedIn"`
	PairingNo  int     `json:"pairingNo"`
	BergerNo   int     `json:"bergerNo"`
	Points     float64 `json:"points"`
}

// PublicRegistration describes online registration for visitors.
type PublicRegistration struct {
	chess.Registration
	Count        int    `json:"count"`
	Available    bool   `json:"available"`
	ClosedReason string `json:"closedReason,omitempty"`
}

// PublicTournament is what visitors and screens see.
type PublicTournament struct {
	ID            int                  `json:"id"`
	Name          string               `json:"name"`
	Kind          string               `json:"kind"`
	System        string               `json:"system"`
	PlannedRounds int                  `json:"plannedRounds"`
	TimeControl   string               `json:"timeControl"`
	PointsWin     float64              `json:"pointsWin"`
	PointsDraw    float64              `json:"pointsDraw"`
	TieBreaks     []string             `json:"tieBreaks"`
	CheckIn       bool                 `json:"checkIn"`
	Started       bool                 `json:"started"`
	LastPublished int                  `json:"lastPublished"`
	Kiosk         bool                 `json:"kiosk"`
	KioskResults  bool                 `json:"kioskResults"`
	Registration  PublicRegistration   `json:"registration"`
	Players       []PublicPlayer       `json:"players"`
	Rounds        []*chess.Round       `json:"rounds"`
	Standings     []chess.Standing     `json:"standings"`
	KO            []chess.KOStanding   `json:"ko,omitempty"`
	Teams         []*chess.Team        `json:"teams,omitempty"`
	TeamCfg       *chess.TeamSettings  `json:"team,omitempty"`
	TeamStandings []chess.TeamStanding `json:"teamStandings,omitempty"`
	TotalRounds   int                  `json:"totalRounds"`
}

// PublicEvent is the public view of an event.
type PublicEvent struct {
	Slug        string             `json:"slug"`
	Name        string             `json:"name"`
	Location    string             `json:"location"`
	StartDate   string             `json:"startDate"`
	EndDate     string             `json:"endDate"`
	Organizer   string             `json:"organizer"`
	Contact     string             `json:"contact"`
	Message     string             `json:"message"`
	Tournaments []PublicTournament `json:"tournaments"`
}

func registrationState(t *chess.Tournament) PublicRegistration {
	reg := PublicRegistration{Registration: t.Registration}
	for _, p := range t.Players {
		if p.Status != chess.StatusRefused {
			reg.Count++
		}
	}
	switch {
	case !t.Registration.Open:
		reg.ClosedReason = "inscriptions en ligne fermées"
	case t.Kind == chess.KindRoundRobin && t.Started():
		reg.ClosedReason = "tournoi commencé"
	case t.Registration.MaxPlayers > 0 && reg.Count >= t.Registration.MaxPlayers:
		reg.ClosedReason = "tournoi complet"
	case deadlinePassed(t.Registration.Deadline):
		reg.ClosedReason = "date limite dépassée"
	case t.Kind == chess.KindSwiss && len(t.Rounds) >= t.TotalRounds:
		reg.ClosedReason = "tournoi terminé"
	default:
		reg.Available = true
	}
	return reg
}

func deadlinePassed(d string) bool {
	if d == "" {
		return false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, d, time.Local); err == nil {
			if layout == "2006-01-02" {
				t = t.Add(24 * time.Hour)
			}
			return time.Now().After(t)
		}
	}
	return false
}

func publicTournament(t *chess.Tournament) PublicTournament {
	last := t.LastPublished()
	pno := t.PairingNumbers()
	scores := t.Scores(last)
	pt := PublicTournament{ID: t.ID, Name: t.Name, Kind: t.Kind, System: t.System, PlannedRounds: t.PlannedRounds(),
		TimeControl: t.TimeControl, PointsWin: t.PointsWin, PointsDraw: t.PointsDraw, TieBreaks: t.TieBreaks,
		CheckIn: t.CheckIn, Started: t.Started(), LastPublished: last, Kiosk: t.KioskPIN != "", KioskResults: t.KioskResults,
		Registration: registrationState(t), Players: []PublicPlayer{}, Rounds: []*chess.Round{}}
	for _, p := range t.Players {
		if p.Status == chess.StatusRefused || (p.Status == chess.StatusPending && !t.Registration.ShowList) {
			continue
		}
		pt.Players = append(pt.Players, PublicPlayer{ID: p.ID, Title: p.Title, LastName: p.LastName, FirstName: p.FirstName,
			Rating: p.Rating, RatingType: p.RatingType, Fed: p.Fed, Club: p.Club, Category: p.Category, Status: p.Status,
			Withdrawn: p.Withdrawn, CheckedIn: p.CheckedIn, PairingNo: pno[p.ID], BergerNo: p.BergerNo, Points: scores[p.ID]})
	}
	for _, r := range t.Rounds {
		if r.Published {
			pt.Rounds = append(pt.Rounds, r)
		}
	}
	pt.TotalRounds = t.TotalRounds
	if t.IsTeam() {
		pt.Teams, pt.TeamCfg = t.Teams, &t.TeamCfg
		pt.TeamStandings = t.TeamStandings(len(t.Rounds))
		pt.Standings = []chess.Standing{}
		return pt
	}
	if t.Kind == chess.KindKnockout {
		pt.Standings = []chess.Standing{}
		pt.KO = t.KOStandings()
		return pt
	}
	pt.Standings = t.Standings(last)
	return pt
}

func (s *Server) publicEventBySlug(w http.ResponseWriter, r *http.Request) (*chess.Event, bool) {
	e, err := s.Store.EventBySlug(r.PathValue("slug"))
	if err != nil || !e.Public {
		writeErr(w, http.StatusNotFound, errors.New("événement introuvable ou non public"))
		return nil, false
	}
	return e, true
}

func publicTournamentOf(e *chess.Event, r *http.Request) (*chess.Tournament, error) {
	t, err := tournamentOf(e, r)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Server) publicEvents(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.PublicEvents()
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if list == nil {
		list = []store.EventSummary{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) publicEvent(w http.ResponseWriter, r *http.Request) {
	e, ok := s.publicEventBySlug(w, r)
	if !ok {
		return
	}
	pe := PublicEvent{Slug: e.Slug, Name: e.Name, Location: e.Location, StartDate: e.StartDate, EndDate: e.EndDate,
		Organizer: e.Organizer, Contact: e.Contact, Message: e.Message, Tournaments: []PublicTournament{}}
	for _, t := range e.Tournaments {
		pe.Tournaments = append(pe.Tournaments, publicTournament(t))
	}
	writeJSON(w, http.StatusOK, pe)
}

func (s *Server) publicStandings(w http.ResponseWriter, r *http.Request) {
	e, ok := s.publicEventBySlug(w, r)
	if !ok {
		return
	}
	t, err := publicTournamentOf(e, r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	n := roundParam(r, t.LastPublished())
	writeJSON(w, http.StatusOK, map[string]any{"round": n, "standings": t.Standings(n)})
}

func (s *Server) publicLookupFFE(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow("lookup:"+clientIP(r), 60, 10*time.Minute, true) {
		writeErr(w, http.StatusTooManyRequests, errors.New("trop de recherches, patientez quelques minutes"))
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	out := []map[string]any{}
	if !s.FFE.Ready() || len(q) < 3 {
		writeJSON(w, http.StatusOK, out)
		return
	}
	for _, p := range s.FFE.Search(q, 10) {
		out = append(out, map[string]any{"ffeId": p.FFEID, "lastName": p.LastName, "firstName": p.FirstName, "club": p.Club,
			"category": p.Category, "standard": p.Standard, "rapid": p.Rapid, "blitz": p.Blitz})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------- registration

type registrationInput struct {
	LastName  string `json:"lastName"`
	FirstName string `json:"firstName"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	BirthDate string `json:"birthDate"`
	Sex       string `json:"sex"`
	Club      string `json:"club"`
	Fed       string `json:"fed"`
	FFEID     string `json:"ffeId"`
	FideID    string `json:"fideId"`
	Rating    int    `json:"rating"`
	Website   string `json:"website"` // honeypot, must stay empty
}

func (s *Server) publicRegister(w http.ResponseWriter, r *http.Request) {
	var in registrationInput
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	if in.Website != "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "pending"})
		return
	}
	if !s.limiter.allow("register:"+clientIP(r), 10, time.Hour, true) {
		writeErr(w, http.StatusTooManyRequests, errors.New("trop d'inscriptions depuis cette adresse, réessayez plus tard"))
		return
	}
	pub, err := s.Store.EventBySlug(r.PathValue("slug"))
	if err != nil || !pub.Public {
		writeErr(w, http.StatusNotFound, errors.New("événement introuvable"))
		return
	}
	in.LastName, in.FirstName, in.Email = strings.TrimSpace(in.LastName), strings.TrimSpace(in.FirstName), strings.TrimSpace(in.Email)
	if in.Email != "" && (!strings.Contains(in.Email, "@") || len(in.Email) > 200) {
		writeErr(w, http.StatusUnprocessableEntity, errors.New("adresse e-mail invalide"))
		return
	}
	// Official data from the FFE base when a licence number is given.
	var official *lookup.Player
	if in.FFEID != "" && s.FFE.Ready() {
		p, err := s.FFE.ByFFE(in.FFEID)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Errorf("licence FFE %s inconnue", strings.ToUpper(in.FFEID)))
			return
		}
		official = p
	}
	var status string
	var name string
	tid := 0
	e, err := s.Store.UpdateEvent(r.Context(), pub.ID, func(e *chess.Event) error {
		t, err := tournamentOf(e, r)
		if err != nil {
			return err
		}
		tid = t.ID
		reg := registrationState(t)
		if !reg.Available {
			return fail(http.StatusConflict, "inscription impossible : %s", reg.ClosedReason)
		}
		var p *chess.Player
		if official != nil {
			p = fromLookup(official, t.RatingKind)
		} else {
			if in.LastName == "" || in.FirstName == "" {
				return errors.New("nom et prénom obligatoires")
			}
			p = &chess.Player{LastName: in.LastName, FirstName: in.FirstName, FideID: in.FideID, FFEID: in.FFEID,
				Club: strings.TrimSpace(in.Club), Fed: in.Fed, BirthDate: in.BirthDate, Sex: in.Sex}
			if in.Rating > 0 {
				p.Rating, p.RatingType = in.Rating, "E"
			}
			if p.Fed == "" {
				p.Fed = "FRA"
			}
		}
		p.Email, p.Phone = in.Email, strings.TrimSpace(in.Phone)
		if t.Registration.MinRating > 0 && p.Rating < t.Registration.MinRating {
			return fmt.Errorf("Elo minimum : %d", t.Registration.MinRating)
		}
		if t.Registration.MaxRating > 0 && p.Rating > t.Registration.MaxRating {
			return fmt.Errorf("Elo maximum : %d", t.Registration.MaxRating)
		}
		p.Source, p.Fee = "online", t.Registration.Fee
		p.Status = chess.StatusPending
		if t.Registration.AutoConfirm {
			p.Status = chess.StatusConfirmed
		}
		if err := t.AddPlayer(p); err != nil {
			return err
		}
		status, name = p.Status, p.FullName()
		return nil
	})
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, e.ID, tid, "registration.online", name, fmt.Sprintf("statut %s, e-mail %s", status, in.Email))
	s.hub.publish(e.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"status": status, "player": name})
}

// ---------------------------------------------------------------- kiosk

func (s *Server) kioskToken(eventID string, t *chess.Tournament) string {
	m := hmac.New(sha256.New, s.kioskKey)
	fmt.Fprintf(m, "%s|%d|%s", eventID, t.ID, t.KioskPIN)
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Server) kioskTournament(w http.ResponseWriter, r *http.Request) (*chess.Event, *chess.Tournament, bool) {
	e, ok := s.publicEventBySlug(w, r)
	if !ok {
		return nil, nil, false
	}
	t, err := publicTournamentOf(e, r)
	if err != nil {
		writeAnyErr(w, err)
		return nil, nil, false
	}
	if t.KioskPIN == "" {
		writeErr(w, http.StatusNotFound, errors.New("kiosque désactivé pour ce tournoi"))
		return nil, nil, false
	}
	return e, t, true
}

func (s *Server) kioskAuth(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PIN string `json:"pin"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	if !s.limiter.allow("kiosk:"+clientIP(r), 10, 10*time.Minute, false) {
		writeErr(w, http.StatusTooManyRequests, errors.New("trop d'essais, patientez"))
		return
	}
	e, t, ok := s.kioskTournament(w, r)
	if !ok {
		return
	}
	if subtle.ConstantTimeCompare([]byte(in.PIN), []byte(t.KioskPIN)) != 1 {
		s.limiter.allow("kiosk:"+clientIP(r), 10, 10*time.Minute, true)
		writeErr(w, http.StatusUnauthorized, errors.New("code incorrect"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": s.kioskToken(e.ID, t)})
}

func (s *Server) kioskAllowed(w http.ResponseWriter, r *http.Request, e *chess.Event, t *chess.Tournament) bool {
	tok := r.Header.Get("X-Kiosk-Token")
	if subtle.ConstantTimeCompare([]byte(tok), []byte(s.kioskToken(e.ID, t))) != 1 {
		writeErr(w, http.StatusUnauthorized, errors.New("kiosque verrouillé : saisissez le code"))
		return false
	}
	return true
}

func (s *Server) kioskData(w http.ResponseWriter, r *http.Request) {
	e, t, ok := s.kioskTournament(w, r)
	if !ok || !s.kioskAllowed(w, r, e, t) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": e.Name, "tournament": publicTournament(t)})
}

func (s *Server) kioskCheckIn(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PlayerID int `json:"playerId"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	e, t, ok := s.kioskTournament(w, r)
	if !ok || !s.kioskAllowed(w, r, e, t) {
		return
	}
	var name string
	_, err := s.Store.UpdateEvent(r.Context(), e.ID, func(e *chess.Event) error {
		t := e.Tournament(t.ID)
		if !t.CheckIn {
			return errors.New("le pointage n'est pas activé")
		}
		if t.Started() {
			return errors.New("le pointage est clos : la première ronde est appariée")
		}
		p := t.Player(in.PlayerID)
		if p == nil || !p.Active() {
			return errors.New("joueur introuvable")
		}
		setCheckIn(p, true)
		name = p.FullName()
		return nil
	})
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, e.ID, t.ID, "kiosk.checkin", name, "")
	s.hub.publish(e.ID)
	writeJSON(w, http.StatusOK, map[string]string{"player": name})
}

func (s *Server) kioskResult(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Round  int    `json:"round"`
		Board  int    `json:"board"`
		Result string `json:"result"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	e, t, ok := s.kioskTournament(w, r)
	if !ok || !s.kioskAllowed(w, r, e, t) {
		return
	}
	switch in.Result {
	case chess.ResultWhiteWins, chess.ResultBlackWins, chess.ResultDraw:
	default:
		writeErr(w, http.StatusUnprocessableEntity, errors.New("résultat invalide"))
		return
	}
	var target string
	_, err := s.Store.UpdateEvent(r.Context(), e.ID, func(e *chess.Event) error {
		t := e.Tournament(t.ID)
		if !t.KioskResults {
			return errors.New("la saisie des résultats par les joueurs est désactivée")
		}
		last := t.LastPublished()
		if in.Round != last || last == 0 {
			return errors.New("seule la ronde en cours peut être saisie")
		}
		for _, b := range t.Rounds[last-1].Boards {
			if b.Number == in.Board && b.Result != "" {
				return errors.New("résultat déjà saisi : adressez-vous à l'arbitre")
			}
		}
		_, tg, _, err := applyResult(t, in.Round, in.Board, in.Result, "kiosk", "kiosque")
		target = tg
		return err
	})
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, e.ID, t.ID, "kiosk.result", target, in.Result)
	s.hub.publish(e.ID)
	w.WriteHeader(http.StatusNoContent)
}
