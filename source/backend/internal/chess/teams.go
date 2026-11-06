package chess

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

// Team is a team of a team tournament; Players are in board order.
type Team struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Club      string `json:"club"`
	Fed       string `json:"fed"`
	Captain   string `json:"captain"`
	Players   []int  `json:"players"`
	Withdrawn int    `json:"withdrawnFrom"` // 0 = active; n = from round n on
}

// TeamPoints are the match points of a team match.
type TeamPoints struct {
	Win     float64 `json:"win"`
	Draw    float64 `json:"draw"`
	Loss    float64 `json:"loss"`
	Forfeit float64 `json:"forfeit"` // whole team absent
}

// TeamSettings configures a team tournament (Boards = 0: individual).
type TeamSettings struct {
	Boards int        `json:"boards"` // games per team match
	Points TeamPoints `json:"points"`
	// Order of the standings: match points then game points (FIDE), or
	// game points first.
	GamePointsFirst bool `json:"gamePointsFirst"`
	// FirstWhiteOdd: the team named first has White on odd boards (FIDE);
	// otherwise on even boards.
	FirstWhiteEven bool `json:"firstWhiteEven"`
}

// TeamMatch is one match between two teams. Away = 0: bye.
type TeamMatch struct {
	Number  int         `json:"number"`
	Home    int         `json:"home"`
	Away    int         `json:"away"`
	Games   []*TeamGame `json:"games"`
	Forfeit string      `json:"forfeit,omitempty"` // home | away | both: whole team absent
}

// TeamGame is one board of a team match; a 0 player is an empty board
// (lost by forfeit).
type TeamGame struct {
	Board  int    `json:"board"`
	White  int    `json:"white"`
	Black  int    `json:"black"`
	Result string `json:"result"`
}

// IsTeam reports whether the tournament is a team tournament.
func (t *Tournament) IsTeam() bool { return t.TeamCfg.Boards > 0 }

// Team returns a team by id.
func (t *Tournament) Team(id int) *Team {
	for _, tm := range t.Teams {
		if tm.ID == id {
			return tm
		}
	}
	return nil
}

func (t *Tournament) teamName(id int) string {
	if tm := t.Team(id); tm != nil {
		return tm.Name
	}
	return fmt.Sprintf("équipe %d", id)
}

// TeamRating is the average rating of the first Boards players.
func (t *Tournament) TeamRating(tm *Team) int {
	sum, n := 0, 0
	for _, id := range tm.Players {
		if n == t.TeamCfg.Boards {
			break
		}
		if p := t.Player(id); p != nil && p.Active() {
			sum += p.Rating
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return (sum + n/2) / n
}

func (t *Tournament) activeTeams(round int) []*Team {
	var out []*Team
	for _, tm := range t.Teams {
		if tm.Withdrawn > 0 && tm.Withdrawn <= round {
			continue
		}
		out = append(out, tm)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := t.TeamRating(out[i]), t.TeamRating(out[j])
		if ri != rj {
			return ri > rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// lineup returns the players fielded by a team in a round: the first
// Boards players present (no absence declared for that round).
func (t *Tournament) lineup(tm *Team, round int) []int {
	var out []int
	for _, id := range tm.Players {
		if len(out) == t.TeamCfg.Boards {
			break
		}
		p := t.Player(id)
		if p == nil || !p.Active() || p.Withdrawn > 0 && p.Withdrawn <= round || p.Byes[round] != "" {
			continue
		}
		out = append(out, id)
	}
	for len(out) < t.TeamCfg.Boards {
		out = append(out, 0)
	}
	return out
}

// newTeamMatch sets up the boards with the default lineups and colours.
func (t *Tournament) newTeamMatch(number, home, away, round int) *TeamMatch {
	m := &TeamMatch{Number: number, Home: home, Away: away, Games: []*TeamGame{}}
	if away == 0 {
		return m
	}
	hl, al := t.lineup(t.Team(home), round), t.lineup(t.Team(away), round)
	for b := 1; b <= t.TeamCfg.Boards; b++ {
		g := &TeamGame{Board: b, White: hl[b-1], Black: al[b-1]}
		homeWhite := b%2 == 1
		if t.TeamCfg.FirstWhiteEven {
			homeWhite = !homeWhite
		}
		if !homeWhite {
			g.White, g.Black = g.Black, g.White
		}
		if g.White == 0 || g.Black == 0 {
			switch {
			case g.White == 0 && g.Black == 0:
				g.Result = ResultDoubleForfeit
			case g.White == 0:
				g.Result = ResultBlackForfeit
			default:
				g.Result = ResultWhiteForfeit
			}
		}
		m.Games = append(m.Games, g)
	}
	return m
}

// homeSide reports whether the home team plays White on a board.
func (t *Tournament) homeWhite(board int) bool {
	w := board%2 == 1
	if t.TeamCfg.FirstWhiteEven {
		w = !w
	}
	return w
}

// MatchScore returns the game points of both teams and whether every
// board has a result.
func (t *Tournament) MatchScore(m *TeamMatch) (home, away float64, complete bool) {
	if m.Away == 0 {
		return 0, 0, true
	}
	switch m.Forfeit {
	case "home":
		return 0, float64(t.TeamCfg.Boards), true
	case "away":
		return float64(t.TeamCfg.Boards), 0, true
	case "both":
		return 0, 0, true
	}
	complete = true
	for _, g := range m.Games {
		pw, ok := koPoints(g.Result, true)
		if !ok {
			complete = false
			continue
		}
		pb, _ := koPoints(g.Result, false)
		if t.homeWhite(g.Board) {
			home, away = home+pw, away+pb
		} else {
			home, away = home+pb, away+pw
		}
	}
	return home, away, complete
}

// MatchPoints returns the match points of both teams.
func (t *Tournament) MatchPoints(m *TeamMatch) (float64, float64, bool) {
	pts := t.TeamCfg.Points
	if m.Away == 0 {
		return pts.Win, 0, true // pairing-allocated bye
	}
	switch m.Forfeit {
	case "home":
		return pts.Forfeit, pts.Win, true
	case "away":
		return pts.Win, pts.Forfeit, true
	case "both":
		return pts.Forfeit, pts.Forfeit, true
	}
	h, a, ok := t.MatchScore(m)
	if !ok {
		return 0, 0, false
	}
	switch {
	case h > a:
		return pts.Win, pts.Loss, true
	case a > h:
		return pts.Loss, pts.Win, true
	}
	return pts.Draw, pts.Draw, true
}

// TeamStanding is a team row of the standings.
type TeamStanding struct {
	Rank   int     `json:"rank"`
	TeamID int     `json:"teamId"`
	MP     float64 `json:"mp"`
	GP     float64 `json:"gp"`
	Played int     `json:"played"`
	Won    int     `json:"won"`
	Drawn  int     `json:"drawn"`
	Lost   int     `json:"lost"`
	BH     float64 `json:"bh"` // sum of the opponents' match points
}

// TeamStandings ranks the teams after n rounds: match points, game points
// (or the reverse), then the sum of the opponents' match points.
func (t *Tournament) TeamStandings(n int) []TeamStanding {
	if n > len(t.Rounds) {
		n = len(t.Rounds)
	}
	rows := map[int]*TeamStanding{}
	for _, tm := range t.Teams {
		rows[tm.ID] = &TeamStanding{TeamID: tm.ID}
	}
	opps := map[int][]int{}
	for _, r := range t.Rounds[:n] {
		for _, m := range r.TeamMatches {
			hm, am, ok := t.MatchPoints(m)
			if !ok {
				continue
			}
			hg, ag, _ := t.MatchScore(m)
			h := rows[m.Home]
			if h == nil {
				continue
			}
			h.MP += hm
			h.GP += hg
			if m.Away == 0 {
				continue
			}
			a := rows[m.Away]
			if a == nil {
				continue
			}
			a.MP += am
			a.GP += ag
			h.Played++
			a.Played++
			switch {
			case hm > am:
				h.Won++
				a.Lost++
			case am > hm:
				a.Won++
				h.Lost++
			default:
				h.Drawn++
				a.Drawn++
			}
			opps[m.Home] = append(opps[m.Home], m.Away)
			opps[m.Away] = append(opps[m.Away], m.Home)
		}
	}
	var out []TeamStanding
	for id, r := range rows {
		for _, o := range opps[id] {
			r.BH += rows[o].MP
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		k1a, k1b, k2a, k2b := a.MP, b.MP, a.GP, b.GP
		if t.TeamCfg.GamePointsFirst {
			k1a, k1b, k2a, k2b = a.GP, b.GP, a.MP, b.MP
		}
		if k1a != k1b {
			return k1a > k1b
		}
		if k2a != k2b {
			return k2a > k2b
		}
		if a.BH != b.BH {
			return a.BH > b.BH
		}
		return t.teamName(a.TeamID) < t.teamName(b.TeamID)
	})
	for i := range out {
		if i > 0 && out[i].MP == out[i-1].MP && out[i].GP == out[i-1].GP && out[i].BH == out[i-1].BH {
			out[i].Rank = out[i-1].Rank
		} else {
			out[i].Rank = i + 1
		}
	}
	return out
}

// teamsMet reports whether two teams already met.
func (t *Tournament) teamsMet(a, b int) bool {
	for _, r := range t.Rounds {
		for _, m := range r.TeamMatches {
			if m.Home == a && m.Away == b || m.Home == b && m.Away == a {
				return true
			}
		}
	}
	return false
}

// CanPairTeams checks that the previous round is finished.
func (t *Tournament) CanPairTeams() error {
	if len(t.Teams) < 2 {
		return errors.New("il faut au moins deux équipes")
	}
	for _, tm := range t.Teams {
		if len(tm.Players) < t.TeamCfg.Boards {
			return fmt.Errorf("l'équipe %s n'a que %d joueur(s) pour %d échiquiers", tm.Name, len(tm.Players), t.TeamCfg.Boards)
		}
	}
	if t.Kind == KindRoundRobin && t.Started() {
		return errors.New("toutes les rondes du toutes-rondes sont créées")
	}
	if t.Kind == KindSwiss && len(t.Rounds) >= t.TotalRounds {
		return fmt.Errorf("toutes les rondes (%d) ont déjà été appariées", t.TotalRounds)
	}
	if cur := t.CurrentRound(); cur != nil {
		for _, m := range cur.TeamMatches {
			if _, _, ok := t.MatchPoints(m); !ok {
				return fmt.Errorf("ronde %d pas terminée : %s – %s", cur.Number, t.teamName(m.Home), t.teamName(m.Away))
			}
		}
	}
	return nil
}

// StartTeamRoundRobin creates every round of a team round robin (Berger
// tables over the teams, ordered by rating or drawn by lots).
func (t *Tournament) StartTeamRoundRobin(order string, seed uint64) error {
	if err := t.CanPairTeams(); err != nil {
		return err
	}
	teams := t.activeTeams(1)
	if order == "lots" {
		r := rand.New(rand.NewPCG(seed, seed^0xa5a5))
		r.Shuffle(len(teams), func(i, j int) { teams[i], teams[j] = teams[j], teams[i] })
	}
	n := len(teams)
	ids := make([]int, n+2)
	for i, tm := range teams {
		ids[i+1] = tm.ID
	}
	sched := BergerSchedule(n)
	cycles := [][][][2]int{sched}
	if t.DoubleRR {
		second := make([][][2]int, len(sched))
		for r, bs := range sched {
			for _, pr := range bs {
				second[r] = append(second[r], [2]int{pr[1], pr[0]})
			}
		}
		cycles = append(cycles, second)
	}
	num := 0
	for _, cyc := range cycles {
		for _, bs := range cyc {
			num++
			round := &Round{Number: num, Boards: []*Board{}, Absences: map[int]string{}, PairedAt: time.Now().UTC(), Published: true}
			mn := 0
			var bye *TeamMatch
			for _, pr := range bs {
				h, a := ids[pr[0]], ids[pr[1]]
				if h == 0 || a == 0 {
					bye = &TeamMatch{Home: h + a, Games: []*TeamGame{}}
					continue
				}
				mn++
				round.TeamMatches = append(round.TeamMatches, t.newTeamMatch(mn, h, a, num))
			}
			if bye != nil {
				bye.Number = mn + 1
				round.TeamMatches = append(round.TeamMatches, bye)
			}
			t.Rounds = append(t.Rounds, round)
		}
	}
	t.TotalRounds = len(t.Rounds)
	return nil
}

// TeamSwissInput builds the engine input of the next team Swiss round: the
// teams play the role of players (rating = average of the boards), match
// results the role of game results.
func (t *Tournament) TeamSwissInput() (*Tournament, *PairingInput, error) {
	if err := t.CanPairTeams(); err != nil {
		return nil, nil, err
	}
	next := len(t.Rounds) + 1
	st := NewTournament(t.Name)
	st.Kind, st.System, st.Acceleration, st.TotalRounds, st.InitialColor = KindSwiss, t.System, t.Acceleration, t.TotalRounds, t.InitialColor
	st.PointsWin, st.PointsDraw, st.PointsPAB = 2, 1, 2
	for _, tm := range t.Teams {
		p := &Player{ID: tm.ID, LastName: tm.Name, Rating: t.TeamRating(tm), RatingType: "F", Fed: tm.Fed, Status: StatusConfirmed, Byes: map[int]string{}}
		if tm.Withdrawn > 0 {
			p.Withdrawn = tm.Withdrawn
		}
		st.Players = append(st.Players, p)
	}
	for _, r := range t.Rounds {
		sr := &Round{Number: r.Number, Absences: map[int]string{}, Published: true}
		played := map[int]bool{}
		for i, m := range r.TeamMatches {
			b := &Board{Number: i + 1, White: m.Home, Black: m.Away}
			played[m.Home] = true
			if m.Away == 0 {
				b.Result = ResultPairingBye
			} else {
				played[m.Away] = true
				hm, am, _ := t.MatchPoints(m)
				switch {
				case hm > am:
					b.Result = ResultWhiteWins
				case am > hm:
					b.Result = ResultBlackWins
				default:
					b.Result = ResultDraw
				}
			}
			sr.Boards = append(sr.Boards, b)
		}
		for _, tm := range t.Teams {
			if !played[tm.ID] {
				sr.Absences[tm.ID] = ByeZero
			}
		}
		st.Rounds = append(st.Rounds, sr)
	}
	st.Normalize()
	pin, err := st.PrepareNextSwissRound()
	if err != nil {
		return nil, nil, err
	}
	pin.Round = next
	return st, pin, nil
}

// ApplyTeamPairing turns the engine output into team matches.
func (t *Tournament) ApplyTeamPairing(st *Tournament, in *PairingInput, output string) (*Round, error) {
	boards, err := parsePairs(output, in.ByPNo)
	if err != nil {
		return nil, err
	}
	next := len(t.Rounds) + 1
	round := &Round{Number: next, Boards: []*Board{}, Absences: map[int]string{}, PairedAt: time.Now().UTC(), Published: true}
	// Strongest matches first, the bye last.
	sort.SliceStable(boards, func(i, j int) bool {
		if (boards[i].Black == 0) != (boards[j].Black == 0) {
			return boards[j].Black == 0
		}
		return st.Score(boards[i].White, next-1)+st.Score(boards[i].Black, next-1) > st.Score(boards[j].White, next-1)+st.Score(boards[j].Black, next-1)
	})
	for i, b := range boards {
		round.TeamMatches = append(round.TeamMatches, t.newTeamMatch(i+1, b.White, b.Black, next))
	}
	for id, code := range in.Absent {
		round.Absences[id] = code
	}
	t.Rounds = append(t.Rounds, round)
	return round, nil
}

// SetTeamResult records the result of one board.
func (t *Tournament) SetTeamResult(round, match, board int, result string) error {
	m, err := t.teamMatch(round, match)
	if err != nil {
		return err
	}
	if !validResults[result] {
		return fmt.Errorf("résultat invalide %q", result)
	}
	for _, g := range m.Games {
		if g.Board == board {
			if g.White == 0 || g.Black == 0 {
				return errors.New("échiquier sans joueur : perdu par forfait (modifiez la composition)")
			}
			g.Result = result
			return nil
		}
	}
	return errors.New("échiquier introuvable")
}

// SetLineup changes the players of a team in a match (before any result
// on that match). players are in board order, 0 = empty board.
func (t *Tournament) SetLineup(round, match int, side string, players []int) error {
	m, err := t.teamMatch(round, match)
	if err != nil {
		return err
	}
	for _, g := range m.Games {
		if g.Result != "" && g.White != 0 && g.Black != 0 {
			return errors.New("des résultats sont déjà saisis sur ce match")
		}
	}
	teamID := m.Home
	if side == "away" {
		teamID = m.Away
	}
	tm := t.Team(teamID)
	if tm == nil {
		return errors.New("équipe introuvable")
	}
	if len(players) != t.TeamCfg.Boards {
		return fmt.Errorf("il faut %d joueurs (0 pour un échiquier vide)", t.TeamCfg.Boards)
	}
	member := map[int]bool{}
	for _, id := range tm.Players {
		member[id] = true
	}
	seen := map[int]bool{}
	for _, id := range players {
		if id == 0 {
			continue
		}
		if !member[id] {
			return fmt.Errorf("%s n'est pas dans l'équipe %s", t.name(id), tm.Name)
		}
		if seen[id] {
			return fmt.Errorf("%s est aligné deux fois", t.name(id))
		}
		seen[id] = true
	}
	for i, g := range m.Games {
		homeWhite := t.homeWhite(g.Board)
		isWhite := (side == "home") == homeWhite
		if isWhite {
			g.White = players[i]
		} else {
			g.Black = players[i]
		}
		g.Result = ""
		switch {
		case g.White == 0 && g.Black == 0:
			g.Result = ResultDoubleForfeit
		case g.White == 0:
			g.Result = ResultBlackForfeit
		case g.Black == 0:
			g.Result = ResultWhiteForfeit
		}
	}
	return nil
}

// SetTeamForfeit records a whole team absent ("home", "away", "both", "" to cancel).
func (t *Tournament) SetTeamForfeit(round, match int, who string) error {
	m, err := t.teamMatch(round, match)
	if err != nil {
		return err
	}
	switch who {
	case "", "home", "away", "both":
		m.Forfeit = who
		return nil
	}
	return errors.New("valeur de forfait inconnue")
}

func (t *Tournament) teamMatch(round, match int) (*TeamMatch, error) {
	if !t.IsTeam() {
		return nil, errors.New("ce n'est pas un tournoi par équipes")
	}
	if round < 1 || round > len(t.Rounds) {
		return nil, errors.New("ronde inexistante")
	}
	for _, m := range t.Rounds[round-1].TeamMatches {
		if m.Number == match {
			return m, nil
		}
	}
	return nil, errors.New("match introuvable")
}

// NormalizeTeams cleans team data: names, unknown players, defaults.
func (t *Tournament) NormalizeTeams() {
	if t.TeamCfg.Points == (TeamPoints{}) {
		t.TeamCfg.Points = TeamPoints{Win: 2, Draw: 1, Loss: 0, Forfeit: 0}
	}
	if t.Teams == nil {
		t.Teams = []*Team{}
	}
	for _, tm := range t.Teams {
		tm.Name = strings.TrimSpace(tm.Name)
		if tm.Players == nil {
			tm.Players = []int{}
		}
		if tm.ID >= t.NextTeamID {
			t.NextTeamID = tm.ID + 1
		}
	}
	if t.NextTeamID < 1 {
		t.NextTeamID = 1
	}
}

// TeamOf returns the team of a player (nil if none).
func (t *Tournament) TeamOf(pid int) *Team {
	for _, tm := range t.Teams {
		for _, id := range tm.Players {
			if id == pid {
				return tm
			}
		}
	}
	return nil
}
