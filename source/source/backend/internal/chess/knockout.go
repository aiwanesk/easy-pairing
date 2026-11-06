package chess

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

// KindKnockout is the cup system (direct elimination).
const KindKnockout = "knockout"

// Stages of a knockout match, in order. The classical games are followed,
// on a tie, by the tie-break stages configured on the tournament.
const (
	StageClassical  = "classical"
	StageRapid      = "rapid"
	StageBlitz      = "blitz"
	StageArmageddon = "armageddon"
)

var stageLabels = map[string]string{StageClassical: "parties", StageRapid: "rapide", StageBlitz: "blitz", StageArmageddon: "Armageddon"}

// Match is one knockout pairing. B = 0 means A qualifies without playing.
type Match struct {
	Number  int       `json:"number"`
	A       int       `json:"a"` // better seed
	B       int       `json:"b"`
	Games   []*KOGame `json:"games"`
	Winner  int       `json:"winner"`  // 0 = not decided
	Decided string    `json:"decided"` // bye | score | tiebreak | arbiter
	Note    string    `json:"note,omitempty"`
}

// KOGame is one game of a match.
type KOGame struct {
	Number int    `json:"number"`
	Stage  string `json:"stage"`
	White  int    `json:"white"`
	Black  int    `json:"black"`
	Result string `json:"result"`
}

// KOSettings configures a knockout.
type KOSettings struct {
	Games     int      `json:"games"`     // classical games per match (1, 2, 4…)
	TieBreaks []string `json:"tieBreaks"` // stages played on a tie, e.g. rapid:2, blitz:2, armageddon
	Seeding   string   `json:"seeding"`   // rating | lots
	Final     int      `json:"final"`     // classical games in the final (0 = same as other rounds)
}

func (k *KOSettings) normalize() {
	if k.Games <= 0 {
		k.Games = 2
	}
	if k.Seeding != "lots" {
		k.Seeding = "rating"
	}
	if k.TieBreaks == nil {
		k.TieBreaks = []string{"rapid:2", "blitz:2", "armageddon"}
	}
}

// parseStage reads "rapid:2" into (rapid, 2); armageddon is always 1 game.
func parseStage(s string) (string, int) {
	name, n := s, 2
	if i := strings.IndexByte(s, ':'); i > 0 {
		name = s[:i]
		fmt.Sscanf(s[i+1:], "%d", &n)
	}
	if name == StageArmageddon {
		n = 1
	}
	if n < 1 {
		n = 1
	}
	return name, n
}

// bracketOrder returns the seeds in bracket order for a power-of-two size
// (1 meets size, and 1 and 2 can only meet in the final).
func bracketOrder(size int) []int {
	order := []int{1}
	for len(order) < size {
		n := len(order) * 2
		next := make([]int, 0, n)
		for _, s := range order {
			next = append(next, s, n+1-s)
		}
		order = next
	}
	return order
}

// KORounds returns the number of rounds of the knockout for n players.
func KORounds(n int) int {
	r := 0
	for size := 1; size < n; size *= 2 {
		r++
	}
	return r
}

// StartKnockout seeds the players and creates the first round (byes for
// the best seeds when the count is not a power of two).
func (t *Tournament) StartKnockout(seed uint64) error {
	if t.Kind != KindKnockout {
		return errors.New("ce tournoi n'est pas une coupe")
	}
	if t.Started() {
		return errors.New("la coupe est déjà commencée")
	}
	t.KO.normalize()
	var players []*Player
	for _, p := range t.Seeding() {
		if p.Withdrawn > 0 || (t.CheckIn && !p.CheckedIn) {
			continue
		}
		players = append(players, p)
	}
	if len(players) < 2 {
		return errors.New("il faut au moins deux joueurs (pointés) pour commencer")
	}
	if t.KO.Seeding == "lots" {
		r := rand.New(rand.NewPCG(seed, seed^0x51ed270b))
		r.Shuffle(len(players), func(i, j int) { players[i], players[j] = players[j], players[i] })
	}
	size := 1
	for size < len(players) {
		size *= 2
	}
	order := bracketOrder(size)
	round := &Round{Number: 1, Boards: []*Board{}, Absences: map[int]string{}, PairedAt: time.Now().UTC(), Published: true}
	for i := 0; i < size; i += 2 {
		a, b := order[i], order[i+1]
		m := &Match{Number: i/2 + 1}
		if a <= len(players) {
			m.A = players[a-1].ID
		}
		if b <= len(players) {
			m.B = players[b-1].ID
		}
		if m.A == 0 {
			m.A, m.B = m.B, 0
		}
		t.initMatch(m, 1)
		round.Matches = append(round.Matches, m)
	}
	for i, p := range players {
		p.BergerNo = i + 1 // seed number
	}
	t.TotalRounds = KORounds(len(players))
	t.Rounds = append(t.Rounds, round)
	return nil
}

func (t *Tournament) classicalGames(round int) int {
	if round == t.TotalRounds && t.KO.Final > 0 {
		return t.KO.Final
	}
	return t.KO.Games
}

func (t *Tournament) initMatch(m *Match, round int) {
	m.Games = []*KOGame{}
	if m.B == 0 {
		m.Winner, m.Decided = m.A, "bye"
		return
	}
	t.addStage(m, StageClassical, t.classicalGames(round))
}

// addStage appends n games of a stage, colours alternating from the
// colour the better seed had in the first game.
func (t *Tournament) addStage(m *Match, stage string, n int) {
	for i := 0; i < n; i++ {
		g := &KOGame{Number: len(m.Games) + 1, Stage: stage, White: m.A, Black: m.B}
		if (len(m.Games)+m.Number)%2 == 0 {
			g.White, g.Black = m.B, m.A
		}
		m.Games = append(m.Games, g)
	}
}

func koPoints(res string, white bool) (float64, bool) {
	switch res {
	case ResultWhiteWins, ResultWhiteForfeit:
		if white {
			return 1, true
		}
		return 0, true
	case ResultBlackWins, ResultBlackForfeit:
		if white {
			return 0, true
		}
		return 1, true
	case ResultDraw:
		return 0.5, true
	case ResultDoubleForfeit:
		return 0, true
	}
	return 0, false
}

// evaluate decides the match from its games, stage by stage (each stage
// is a separate mini-match, FIDE knockout practice). Returns the stage
// still undecided and whether a new tie-break stage is needed.
func (t *Tournament) evaluate(m *Match) (needStage bool) {
	if m.Decided == "arbiter" || m.Decided == "bye" {
		return false
	}
	m.Winner, m.Decided = 0, ""
	var stages []string
	seen := map[string]bool{}
	for _, g := range m.Games {
		if !seen[g.Stage] {
			seen[g.Stage] = true
			stages = append(stages, g.Stage)
		}
	}
	for _, st := range stages {
		var a, b float64
		complete := true
		for _, g := range m.Games {
			if g.Stage != st {
				continue
			}
			pw, ok := koPoints(g.Result, true)
			if !ok {
				complete = false
				continue
			}
			pb, _ := koPoints(g.Result, false)
			if g.White == m.A {
				a, b = a+pw, b+pb
			} else {
				a, b = a+pb, b+pw
			}
			if st == StageArmageddon && g.Result == ResultDraw {
				// Draw in Armageddon: Black qualifies.
				if g.Black == m.A {
					a++
				} else {
					b++
				}
			}
		}
		if !complete {
			return false
		}
		if a != b {
			m.Winner = m.A
			if b > a {
				m.Winner = m.B
			}
			m.Decided = "score"
			if st != StageClassical {
				m.Decided = "tiebreak"
			}
			return false
		}
	}
	return true
}

// KOSetResult records a game result and decides the match if possible.
func (t *Tournament) KOSetResult(round, match, game int, result string) error {
	m, err := t.koMatch(round, match)
	if err != nil {
		return err
	}
	if !validResults[result] {
		return fmt.Errorf("résultat invalide %q", result)
	}
	if round < len(t.Rounds) {
		return errors.New("le tour suivant est déjà créé : ce résultat ne peut plus changer")
	}
	for _, g := range m.Games {
		if g.Number == game {
			g.Result = result
			t.evaluate(m)
			return nil
		}
	}
	return errors.New("partie introuvable")
}

// KOAddTieBreak adds the next tie-break stage of a tied match.
func (t *Tournament) KOAddTieBreak(round, match int) (string, error) {
	m, err := t.koMatch(round, match)
	if err != nil {
		return "", err
	}
	if !t.evaluate(m) {
		if m.Winner != 0 {
			return "", errors.New("le match est déjà décidé")
		}
		return "", errors.New("toutes les parties en cours doivent avoir un résultat")
	}
	used := map[string]int{}
	for _, g := range m.Games {
		used[g.Stage]++
	}
	for _, s := range t.KO.TieBreaks {
		name, n := parseStage(s)
		if used[name] == 0 {
			t.addStage(m, name, n)
			return stageLabels[name], nil
		}
	}
	return "", errors.New("plus de départage prévu : désignez le qualifié (tirage au sort ou décision de l'arbitre)")
}

// KOSetWinner lets the arbiter designate the qualified player (drawing of
// lots, forfeit, exclusion…).
func (t *Tournament) KOSetWinner(round, match, winner int, note string) error {
	m, err := t.koMatch(round, match)
	if err != nil {
		return err
	}
	if round < len(t.Rounds) {
		return errors.New("le tour suivant est déjà créé")
	}
	if winner == 0 {
		m.Decided, m.Note = "", ""
		t.evaluate(m)
		return nil
	}
	if winner != m.A && winner != m.B {
		return errors.New("le qualifié doit être un des deux joueurs du match")
	}
	m.Winner, m.Decided, m.Note = winner, "arbiter", strings.TrimSpace(note)
	return nil
}

func (t *Tournament) koMatch(round, match int) (*Match, error) {
	if t.Kind != KindKnockout {
		return nil, errors.New("ce tournoi n'est pas une coupe")
	}
	if round < 1 || round > len(t.Rounds) {
		return nil, errors.New("tour inexistant")
	}
	for _, m := range t.Rounds[round-1].Matches {
		if m.Number == match {
			return m, nil
		}
	}
	return nil, errors.New("match introuvable")
}

// KOCanAdvance reports whether the next round can be created.
func (t *Tournament) KOCanAdvance() error {
	if len(t.Rounds) == 0 {
		return nil
	}
	cur := t.Rounds[len(t.Rounds)-1]
	if len(cur.Matches) == 1 {
		return errors.New("la finale est le dernier tour")
	}
	var pending []string
	for _, m := range cur.Matches {
		if m.Winner == 0 {
			pending = append(pending, fmt.Sprintf("match %d (%s – %s)", m.Number, t.name(m.A), t.name(m.B)))
		}
	}
	if len(pending) > 0 {
		return fmt.Errorf("%s pas terminé(e)s : %s", strings.ToLower(t.KORoundName(cur.Number)), strings.Join(pending, ", "))
	}
	return nil
}

// KONextRound creates the next round from the winners.
func (t *Tournament) KONextRound() (*Round, error) {
	if len(t.Rounds) == 0 {
		return nil, errors.New("la coupe n'est pas commencée")
	}
	if err := t.KOCanAdvance(); err != nil {
		return nil, err
	}
	cur := t.Rounds[len(t.Rounds)-1]
	n := cur.Number + 1
	round := &Round{Number: n, Boards: []*Board{}, Absences: map[int]string{}, PairedAt: time.Now().UTC(), Published: true}
	for i := 0; i+1 < len(cur.Matches); i += 2 {
		a, b := cur.Matches[i].Winner, cur.Matches[i+1].Winner
		// The better seed is A.
		if t.Player(a) != nil && t.Player(b) != nil && t.Player(b).BergerNo < t.Player(a).BergerNo {
			a, b = b, a
		}
		m := &Match{Number: i/2 + 1, A: a, B: b}
		t.initMatch(m, n)
		round.Matches = append(round.Matches, m)
	}
	t.Rounds = append(t.Rounds, round)
	return round, nil
}

// KOStanding is a player's place in a knockout.
type KOStanding struct {
	Rank       int    `json:"rank"` // shared by players eliminated in the same round
	PlayerID   int    `json:"playerId"`
	Eliminated int    `json:"eliminated"` // round of elimination, 0 = still in / winner
	Label      string `json:"label"`
}

// KOStandings ranks the players by the round they reached.
func (t *Tournament) KOStandings() []KOStanding {
	reached := map[int]int{}
	out := map[int]int{}
	for _, r := range t.Rounds {
		for _, m := range r.Matches {
			for _, id := range []int{m.A, m.B} {
				if id != 0 {
					reached[id] = r.Number
				}
			}
			if m.Winner != 0 && m.B != 0 {
				loser := m.A
				if m.Winner == m.A {
					loser = m.B
				}
				out[loser] = r.Number
			}
		}
	}
	var rows []KOStanding
	for id := range reached {
		rows = append(rows, KOStanding{PlayerID: id, Eliminated: out[id]})
	}
	final := t.TotalRounds
	key := func(s KOStanding) int {
		if s.Eliminated == 0 {
			return 1000 + reached[s.PlayerID]
		}
		return s.Eliminated
	}
	sort.Slice(rows, func(i, j int) bool {
		if key(rows[i]) != key(rows[j]) {
			return key(rows[i]) > key(rows[j])
		}
		return t.Player(rows[i].PlayerID).BergerNo < t.Player(rows[j].PlayerID).BergerNo
	})
	for i := range rows {
		if i > 0 && key(rows[i]) == key(rows[i-1]) {
			rows[i].Rank = rows[i-1].Rank
		} else {
			rows[i].Rank = i + 1
		}
		switch {
		case rows[i].Eliminated == 0 && len(t.Rounds) == final && t.Rounds[final-1].Matches[0].Winner == rows[i].PlayerID:
			rows[i].Label = "vainqueur"
		case rows[i].Eliminated == 0:
			rows[i].Label = "qualifié"
		case rows[i].Eliminated == final:
			rows[i].Label = "finaliste"
		case rows[i].Eliminated == final-1:
			rows[i].Label = "demi-finale"
		case rows[i].Eliminated == final-2:
			rows[i].Label = "quart de finale"
		default:
			rows[i].Label = fmt.Sprintf("éliminé au tour %d", rows[i].Eliminated)
		}
	}
	return rows
}

// KORoundName names a knockout round (finale, demi-finales…).
func (t *Tournament) KORoundName(n int) string {
	switch t.TotalRounds - n {
	case 0:
		return "Finale"
	case 1:
		return "Demi-finales"
	case 2:
		return "Quarts de finale"
	case 3:
		return "Huitièmes de finale"
	}
	return fmt.Sprintf("Tour %d", n)
}
