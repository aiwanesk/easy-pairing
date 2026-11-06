// Package chess holds the event/tournament model and the pure logic around
// it: scores, Swiss pairing input for bbpPairings, round-robin (Berger)
// schedules, manual pairings, tie-breaks, standings, prizes and exports.
// Nothing in this package does I/O.
package chess

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Tournament kinds and pairing systems.
const (
	KindSwiss      = "swiss"
	KindRoundRobin = "roundrobin"

	SystemDutch    = "dutch"
	SystemBurstein = "burstein"

	AccelNone = "none"
	AccelBaku = "baku"
)

// Results stored on a board. Empty string means "not entered yet".
const (
	ResultWhiteWins     = "1-0"
	ResultBlackWins     = "0-1"
	ResultDraw          = "1/2"
	ResultWhiteForfeit  = "+-" // white wins by forfeit
	ResultBlackForfeit  = "-+" // black wins by forfeit
	ResultDoubleForfeit = "--"
	ResultPairingBye    = "PAB"  // Swiss pairing-allocated bye (Black == 0)
	ResultFreeRound     = "FREE" // round-robin free round (Black == 0), no points
)

// Absence codes for players who are not paired in a round.
const (
	ByeHalf = "H" // requested half-point bye
	ByeZero = "Z" // absent / zero-point bye
	ByeFull = "F" // full-point bye granted by the arbiter
)

// Player registration status.
const (
	StatusConfirmed = "confirmed"
	StatusPending   = "pending"
	StatusRefused   = "refused"
)

var validResults = map[string]bool{
	"": true, ResultWhiteWins: true, ResultBlackWins: true, ResultDraw: true,
	ResultWhiteForfeit: true, ResultBlackForfeit: true, ResultDoubleForfeit: true,
}

// IsValidResult reports whether r can be entered on a board.
func IsValidResult(r string) bool { return validResults[r] }

// Event groups tournaments played at the same place and time.
type Event struct {
	ID          string        `json:"id"`
	Slug        string        `json:"slug"`
	Name        string        `json:"name"`
	Location    string        `json:"location"`
	StartDate   string        `json:"startDate"`
	EndDate     string        `json:"endDate"`
	Organizer   string        `json:"organizer"`
	Contact     string        `json:"contact"`
	Public      bool          `json:"public"`
	Message     string        `json:"message"` // banner shown on public pages
	Tournaments []*Tournament `json:"tournaments"`
	NextTID     int           `json:"nextTournamentId"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

// Registration configures online registration for a tournament.
type Registration struct {
	Open        bool    `json:"open"`
	Deadline    string  `json:"deadline"` // RFC3339 or YYYY-MM-DDTHH:MM, empty = none
	MaxPlayers  int     `json:"maxPlayers"`
	MinRating   int     `json:"minRating"`
	MaxRating   int     `json:"maxRating"`
	Fee         float64 `json:"fee"`
	AutoConfirm bool    `json:"autoConfirm"`
	Message     string  `json:"message"`
	ShowList    bool    `json:"showList"` // publish the list of registered players
}

// Prize split modes for players tied on points.
const (
	PrizeSplitPlace  = "place"  // prizes follow the tie-break ranking
	PrizeSplitShared = "shared" // tied players share the money of their places equally
	PrizeSplitHort   = "hort"   // half by tie-break ranking, half shared equally
)

// Prize is one prize of the prize list. Conditions restrict eligibility;
// prizes with the same conditions form a group (general ranking, U1600…)
// inside which money is split between tied players.
type Prize struct {
	ID        int     `json:"id"`
	Label     string  `json:"label"`
	Amount    float64 `json:"amount"`
	Category  string  `json:"category"`  // age categories, comma-separated prefixes (e.g. "Pou,Pup" or "Vet,Sep"), empty = any
	MinRating int     `json:"minRating"` // 0 = any
	MaxRating int     `json:"maxRating"` // 0 = any
	Sex       string  `json:"sex"`       // "f" women only, "m" men only, empty = any
	Fed       string  `json:"fed"`       // federations, comma-separated (e.g. "FRA,MON"), empty = any
	Club      string  `json:"club"`      // exact club name, empty = any
	Unrated   bool    `json:"unrated"`   // only players without an official rating (estimated or none)
	MinPoints float64 `json:"minPoints"` // minimum score, 0 = none
}

// LogEntry records pairing-integrity events and settings changes, as Sharly
// Chess does, and is written as comments in the TRF export.
type LogEntry struct {
	At      time.Time `json:"at"`
	Round   int       `json:"round"`
	Kind    string    `json:"kind"` // pairing | manual | correction | settings | publish
	Message string    `json:"message"`
	By      string    `json:"by"`
}

// Tournament is one tournament of an event.
type Tournament struct {
	ID            int          `json:"id"`
	Name          string       `json:"name"`
	Kind          string       `json:"kind"`   // swiss | roundrobin
	System        string       `json:"system"` // dutch | burstein (swiss)
	Acceleration  string       `json:"acceleration"`
	DoubleRR      bool         `json:"doubleRoundRobin"`
	ReverseLastRR bool         `json:"reverseLastRounds"` // FIDE recommendation for double RR
	TotalRounds   int          `json:"totalRounds"`
	InitialColor  string       `json:"initialColor"` // white | black
	RatingKind    string       `json:"ratingKind"`   // standard | rapid | blitz
	TimeControl   string       `json:"timeControl"`
	PointsWin     float64      `json:"pointsWin"`
	PointsDraw    float64      `json:"pointsDraw"`
	PointsPAB     float64      `json:"pointsPab"`
	TieBreaks     []string     `json:"tieBreaks"`
	UnratedValue  int          `json:"unratedValue"` // rating used for unrated players in ARO/TPR (0 = ignore)
	FIDEMode      bool         `json:"fideMode"`     // refuse actions FIDE regulations forbid
	CheckIn       bool         `json:"checkIn"`      // only checked-in players are paired in round 1
	KioskPIN      string       `json:"kioskPin"`     // PIN of the public check-in / result entry kiosk
	KioskResults  bool         `json:"kioskResults"` // players may enter results at the kiosk
	Registration  Registration `json:"registration"`
	KO            KOSettings   `json:"ko"`   // cup system settings
	TeamCfg       TeamSettings `json:"team"` // team tournament (Boards > 0)
	Teams         []*Team      `json:"teams"`
	NextTeamID    int          `json:"nextTeamId"`
	Prizes        []Prize      `json:"prizes"`
	PrizeSplit    string       `json:"prizeSplit"` // place | shared | hort
	NormsFrom     int          `json:"normsFrom"`  // show players close to a norm from this round on (0 = 5)
	NextPrizeID   int          `json:"nextPrizeId"`
	Players       []*Player    `json:"players"`
	// ForbiddenGroups are sets of players who must not be paired together
	// (same family, club…), sent to bbpPairings as XXP lines.
	ForbiddenGroups [][]int    `json:"forbiddenGroups"`
	Rounds          []*Round   `json:"rounds"`
	NextPlayerID    int        `json:"nextPlayerId"`
	Log             []LogEntry `json:"log"`
	ArbiterNotes    string     `json:"arbiterNotes"`
	lastDraw        []string   // description of the last drawing of lots (not stored)
	ChiefArbiter    string     `json:"chiefArbiter"`
	Federation      string     `json:"federation"`
}

// Player is a tournament participant (or a pending online registration).
type Player struct {
	ID            int            `json:"id"`
	LastName      string         `json:"lastName"`
	FirstName     string         `json:"firstName"`
	Title         string         `json:"title"`
	Rating        int            `json:"rating"`
	RatingType    string         `json:"ratingType"` // F (FIDE), N (national), E (estimated)
	Fed           string         `json:"fed"`
	Sex           string         `json:"sex"`
	BirthDate     string         `json:"birthDate"` // YYYY/MM/DD or YYYY
	Category      string         `json:"category"`
	Club          string         `json:"club"`
	FideID        string         `json:"fideId"`
	FFEID         string         `json:"ffeId"`
	Licence       string         `json:"licence"` // FFE licence type A/B/N
	Email         string         `json:"email"`
	Phone         string         `json:"phone"`
	Status        string         `json:"status"`
	Source        string         `json:"source"` // arbiter | online | bulk | import
	RegisteredAt  time.Time      `json:"registeredAt"`
	Fee           float64        `json:"fee"`
	Paid          float64        `json:"paid"`
	CheckedIn     bool           `json:"checkedIn"`
	CheckedInAt   *time.Time     `json:"checkedInAt,omitempty"`
	ByeIneligible bool           `json:"byeIneligible"` // FIDE C.05:6.7.4
	BergerNo      int            `json:"bergerNo"`      // round-robin table number
	Withdrawn     int            `json:"withdrawnFrom"` // 0 = active; n = absent from round n on
	Byes          map[int]string `json:"byes"`          // round -> H | Z | F requested in advance
	Notes         string         `json:"notes"`
}

// Round holds the boards of one paired round.
type Round struct {
	Number      int            `json:"number"`
	Boards      []*Board       `json:"boards"`
	Absences    map[int]string `json:"absences"` // player id -> H | Z | F, frozen at pairing time
	PairedAt    time.Time      `json:"pairedAt"`
	Published   bool           `json:"published"`
	Manual      bool           `json:"manual"`                // pairings edited by hand
	Checked     string         `json:"engineCheck"`           // result of the engine check of a manual pairing
	Matches     []*Match       `json:"matches,omitempty"`     // knockout (cup) rounds
	TeamMatches []*TeamMatch   `json:"teamMatches,omitempty"` // team tournaments
}

// Board is one game (or the bye / free round when Black == 0).
type Board struct {
	Number  int       `json:"number"`
	White   int       `json:"white"`
	Black   int       `json:"black"`
	Result  string    `json:"result"`
	Source  string    `json:"source,omitempty"` // arbiter | kiosk
	Updated time.Time `json:"updated,omitempty"`
}

// ---------------------------------------------------------------- defaults

// DefaultSwissTieBreaks follows common FFE/FIDE practice for opens.
var DefaultSwissTieBreaks = []string{TBBuchholzCut1, TBBuchholz, TBSonneborn, TBProgressive}

// DefaultRRTieBreaks follows FIDE C.07 recommendations for round robins.
var DefaultRRTieBreaks = []string{TBDirectEncounter, TBSonneborn, TBWins, TBKoya}

// NewEvent returns an event with defaults.
func NewEvent(id, slug, name string) *Event {
	now := time.Now().UTC()
	return &Event{ID: id, Slug: slug, Name: name, Tournaments: []*Tournament{}, NextTID: 1, CreatedAt: now, UpdatedAt: now}
}

// NewTournament returns a Swiss tournament with defaults.
func NewTournament(name string) *Tournament {
	t := &Tournament{Name: name, Kind: KindSwiss, PointsWin: 1, PointsDraw: 0.5, PointsPAB: 1}
	t.Normalize()
	return t
}

// AddTournament appends a tournament and assigns its id.
func (e *Event) AddTournament(t *Tournament) {
	if e.NextTID < 1 {
		e.NextTID = 1
	}
	t.ID = e.NextTID
	e.NextTID++
	t.Normalize()
	e.Tournaments = append(e.Tournaments, t)
}

// Tournament finds a tournament by id.
func (e *Event) Tournament(id int) *Tournament {
	for _, t := range e.Tournaments {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// RemoveTournament deletes a tournament.
func (e *Event) RemoveTournament(id int) error {
	for i, t := range e.Tournaments {
		if t.ID == id {
			e.Tournaments = append(e.Tournaments[:i], e.Tournaments[i+1:]...)
			return nil
		}
	}
	return errors.New("tournoi introuvable")
}

// Normalize fixes zero values after decoding/updates.
func (e *Event) Normalize() {
	if e.Tournaments == nil {
		e.Tournaments = []*Tournament{}
	}
	for _, t := range e.Tournaments {
		t.Normalize()
		if t.ID >= e.NextTID {
			e.NextTID = t.ID + 1
		}
	}
	if e.NextTID < 1 {
		e.NextTID = 1
	}
}

// Normalize fixes zero values and invalid settings.
func (t *Tournament) Normalize() {
	if t.Kind != KindRoundRobin && t.Kind != KindKnockout {
		t.Kind = KindSwiss
	}
	t.KO.normalize()
	t.NormalizeTeams()
	if t.System != SystemBurstein {
		t.System = SystemDutch
	}
	if t.Acceleration != AccelBaku {
		t.Acceleration = AccelNone
	}
	if t.Kind == KindSwiss && t.TotalRounds <= 0 {
		t.TotalRounds = 7
	}
	if t.InitialColor != "black" {
		t.InitialColor = "white"
	}
	switch t.RatingKind {
	case "rapid", "blitz":
	default:
		t.RatingKind = "standard"
	}
	if t.PointsWin <= 0 {
		t.PointsWin = 1
	}
	if t.PointsDraw < 0 || t.PointsDraw > t.PointsWin {
		t.PointsDraw = t.PointsWin / 2
	}
	if t.PointsPAB < 0 || t.PointsPAB > t.PointsWin {
		t.PointsPAB = t.PointsWin
	}
	if t.PointsPAB == 0 && len(t.Rounds) == 0 && t.Kind == KindSwiss {
		t.PointsPAB = t.PointsWin
	}
	if len(t.TieBreaks) == 0 {
		if t.Kind == KindRoundRobin {
			t.TieBreaks = append([]string{}, DefaultRRTieBreaks...)
		} else {
			t.TieBreaks = append([]string{}, DefaultSwissTieBreaks...)
		}
	}
	tbs := t.TieBreaks[:0]
	seen := map[string]bool{}
	for _, tb := range t.TieBreaks {
		if TieBreakLabel(tb) != "" && !seen[tb] {
			seen[tb] = true
			tbs = append(tbs, tb)
		}
	}
	t.TieBreaks = tbs
	if t.Players == nil {
		t.Players = []*Player{}
	}
	if t.Rounds == nil {
		t.Rounds = []*Round{}
	}
	if t.ForbiddenGroups == nil {
		t.ForbiddenGroups = [][]int{}
	}
	if t.Prizes == nil {
		t.Prizes = []Prize{}
	}
	if t.NormsFrom <= 0 {
		t.NormsFrom = 5
	}
	switch t.PrizeSplit {
	case PrizeSplitShared, PrizeSplitHort:
	default:
		t.PrizeSplit = PrizeSplitPlace
	}
	if t.Log == nil {
		t.Log = []LogEntry{}
	}
	if t.NextPlayerID < 1 {
		t.NextPlayerID = 1
	}
	if t.NextPrizeID < 1 {
		t.NextPrizeID = 1
	}
	for _, p := range t.Players {
		if p.Byes == nil {
			p.Byes = map[int]string{}
		}
		if p.Status == "" {
			p.Status = StatusConfirmed
		}
		if p.ID >= t.NextPlayerID {
			t.NextPlayerID = p.ID + 1
		}
	}
	for _, pr := range t.Prizes {
		if pr.ID >= t.NextPrizeID {
			t.NextPrizeID = pr.ID + 1
		}
	}
	for _, r := range t.Rounds {
		if r.Boards == nil {
			r.Boards = []*Board{}
		}
		if r.Absences == nil {
			r.Absences = map[int]string{}
		}
	}
}

// Started reports whether at least one round has been paired.
func (t *Tournament) Started() bool { return len(t.Rounds) > 0 }

// AddLog appends an entry to the tournament log.
func (t *Tournament) AddLog(kind string, round int, by, format string, args ...any) {
	t.Log = append(t.Log, LogEntry{At: time.Now().UTC(), Round: round, Kind: kind, By: by, Message: fmt.Sprintf(format, args...)})
}

// FullName returns "LASTNAME Firstname".
func (p *Player) FullName() string {
	return strings.TrimSpace(strings.ToUpper(p.LastName) + " " + p.FirstName)
}

// Active reports whether a player takes part in pairings (confirmed).
func (p *Player) Active() bool { return p.Status == StatusConfirmed }

// Player looks a participant up by id.
func (t *Tournament) Player(id int) *Player {
	for _, p := range t.Players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// Participants returns confirmed players.
func (t *Tournament) Participants() []*Player {
	out := make([]*Player, 0, len(t.Players))
	for _, p := range t.Players {
		if p.Active() {
			out = append(out, p)
		}
	}
	return out
}

// DuplicateOf returns an existing player with the same FIDE or FFE id.
func (t *Tournament) DuplicateOf(p *Player) *Player {
	for _, q := range t.Players {
		if q.ID == p.ID || q.Status == StatusRefused {
			continue
		}
		if p.FideID != "" && q.FideID == p.FideID {
			return q
		}
		if p.FFEID != "" && strings.EqualFold(q.FFEID, p.FFEID) {
			return q
		}
	}
	return nil
}

// CleanPlayer trims and normalizes user-provided player fields.
func CleanPlayer(p *Player) {
	p.LastName = strings.TrimSpace(p.LastName)
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.Title = strings.ToUpper(strings.TrimSpace(p.Title))
	p.Fed = strings.ToUpper(strings.TrimSpace(p.Fed))
	p.FFEID = strings.ToUpper(strings.TrimSpace(p.FFEID))
	p.FideID = strings.TrimLeft(strings.TrimSpace(p.FideID), "0")
	p.RatingType = strings.ToUpper(strings.TrimSpace(p.RatingType))
	p.Sex = strings.ToLower(strings.TrimSpace(p.Sex))
	p.Email = strings.TrimSpace(p.Email)
	if p.Rating < 0 || p.Rating > 3500 {
		p.Rating = 0
	}
	if p.Byes == nil {
		p.Byes = map[int]string{}
	}
	for k, v := range p.Byes {
		if v != ByeHalf && v != ByeZero && v != ByeFull {
			delete(p.Byes, k)
		}
	}
	switch p.Status {
	case StatusPending, StatusRefused:
	default:
		p.Status = StatusConfirmed
	}
}

// AddPlayer validates and appends a player, assigning its id.
func (t *Tournament) AddPlayer(p *Player) error {
	CleanPlayer(p)
	if p.LastName == "" {
		return errors.New("le nom du joueur est obligatoire")
	}
	if q := t.DuplicateOf(p); q != nil {
		return fmt.Errorf("%s est déjà inscrit", q.FullName())
	}
	if t.Kind == KindRoundRobin && t.Started() && p.Status == StatusConfirmed {
		return errors.New("toutes-rondes déjà commencé : impossible d'ajouter un joueur")
	}
	if p.RegisteredAt.IsZero() {
		p.RegisteredAt = time.Now().UTC()
	}
	p.ID = t.NextPlayerID
	t.NextPlayerID++
	t.Players = append(t.Players, p)
	return nil
}

// HasPlayed reports whether a player appears in any paired round.
func (t *Tournament) HasPlayed(id int) bool {
	for _, r := range t.Rounds {
		if _, ok := r.Absences[id]; ok {
			return true
		}
		for _, b := range r.Boards {
			if b.White == id || b.Black == id {
				return true
			}
		}
	}
	return false
}

// RemovePlayer deletes a player who has never been paired.
func (t *Tournament) RemovePlayer(id int) error {
	if t.HasPlayed(id) {
		return errors.New("ce joueur a déjà été apparié : utilisez le retrait")
	}
	for i, p := range t.Players {
		if p.ID == id {
			t.Players = append(t.Players[:i], t.Players[i+1:]...)
			return nil
		}
	}
	return errors.New("joueur introuvable")
}

// CurrentRound returns the last paired round, or nil.
func (t *Tournament) CurrentRound() *Round {
	if len(t.Rounds) == 0 {
		return nil
	}
	return t.Rounds[len(t.Rounds)-1]
}

// Complete reports whether every game of a round has a result.
func (r *Round) Complete() bool {
	for _, b := range r.Boards {
		if b.Black != 0 && b.White != 0 && b.Result == "" {
			return false
		}
	}
	return true
}

// LastPublished returns the number of the last published round.
func (t *Tournament) LastPublished() int {
	n := 0
	for _, r := range t.Rounds {
		if r.Published {
			n = r.Number
		}
	}
	return n
}

// PlannedRounds is the number of rounds of the tournament.
func (t *Tournament) PlannedRounds() int {
	if t.Kind == KindKnockout {
		if t.Started() {
			return t.TotalRounds
		}
		n := 0
		for _, p := range t.Participants() {
			if p.Withdrawn == 0 && (!t.CheckIn || p.CheckedIn) {
				n++
			}
		}
		return KORounds(n)
	}
	if t.Kind == KindRoundRobin {
		if t.Started() {
			return len(t.Rounds)
		}
		n := len(t.rrPlayers())
		if n < 2 {
			return 0
		}
		if n%2 == 1 {
			n++
		}
		r := n - 1
		if t.DoubleRR {
			r *= 2
		}
		return r
	}
	return t.TotalRounds
}

// SetResult records a result on a board of a round.
func (t *Tournament) SetResult(round, board int, result, source string) error {
	if round < 1 || round > len(t.Rounds) {
		return errors.New("ronde inexistante")
	}
	if !validResults[result] {
		return fmt.Errorf("résultat invalide %q", result)
	}
	for _, b := range t.Rounds[round-1].Boards {
		if b.Number == board {
			if b.Black == 0 || b.White == 0 {
				return errors.New("pas de résultat à saisir pour l'exempt")
			}
			b.Result = result
			b.Source = source
			b.Updated = time.Now().UTC()
			return nil
		}
	}
	return errors.New("échiquier inexistant")
}

// AbsenceFor returns the bye/absence code of a player for a round about to
// be paired: requested bye, withdrawal, or missing check-in before round 1.
func (t *Tournament) AbsenceFor(p *Player, round int) string {
	if p.Withdrawn > 0 && round >= p.Withdrawn {
		return ByeZero
	}
	if t.CheckIn && round == 1 && !p.CheckedIn {
		return ByeZero
	}
	if code, ok := p.Byes[round]; ok {
		switch code {
		case ByeHalf, ByeZero, ByeFull:
			return code
		}
	}
	return ""
}

// titleRank orders titles for seeding (lower is stronger).
func titleRank(title string) int {
	switch strings.ToUpper(strings.TrimSpace(title)) {
	case "GM":
		return 0
	case "IM":
		return 1
	case "WGM":
		return 2
	case "FM":
		return 3
	case "WIM":
		return 4
	case "CM":
		return 5
	case "WFM":
		return 6
	case "WCM":
		return 7
	}
	return 8
}

// Seeding returns confirmed players in initial-ranking order (FIDE C.04.2:
// rating, then title, then alphabetical). Index+1 is the pairing number.
func (t *Tournament) Seeding() []*Player {
	out := t.Participants()
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Rating != b.Rating {
			return a.Rating > b.Rating
		}
		if ta, tb := titleRank(a.Title), titleRank(b.Title); ta != tb {
			return ta < tb
		}
		if na, nb := a.FullName(), b.FullName(); na != nb {
			return na < nb
		}
		return a.ID < b.ID
	})
	return out
}

// PairingNumbers maps player id -> pairing number.
func (t *Tournament) PairingNumbers() map[int]int {
	m := map[int]int{}
	for i, p := range t.Seeding() {
		m[p.ID] = i + 1
	}
	return m
}
