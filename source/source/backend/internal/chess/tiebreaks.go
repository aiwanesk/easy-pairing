package chess

import (
	"math"
	"sort"
)

// Tie-break identifiers (FIDE C.07 acronyms).
const (
	TBBuchholz        = "BH"
	TBBuchholzCut1    = "BH-C1"
	TBBuchholzCut2    = "BH-C2"
	TBBuchholzMedian1 = "BH-M1"
	TBSonneborn       = "SB"
	TBProgressive     = "PS"
	TBWins            = "WIN"
	TBWonGames        = "WON"
	TBBlackGames      = "BPG"
	TBBlackWins       = "BWG"
	TBDirectEncounter = "DE"
	TBARO             = "ARO"
	TBTPR             = "TPR"
	TBKoya            = "KS"
)

var tieBreakLabels = map[string]string{
	TBBuchholz:        "Buchholz",
	TBBuchholzCut1:    "Buchholz tronqué (-1)",
	TBBuchholzCut2:    "Buchholz tronqué (-2)",
	TBBuchholzMedian1: "Buchholz médian",
	TBSonneborn:       "Sonneborn-Berger",
	TBProgressive:     "Cumulatif",
	TBWins:            "Victoires (y compris non jouées)",
	TBWonGames:        "Parties gagnées",
	TBBlackGames:      "Parties jouées avec les Noirs",
	TBBlackWins:       "Victoires avec les Noirs",
	TBDirectEncounter: "Confrontation directe",
	TBARO:             "Moyenne Elo des adversaires",
	TBTPR:             "Performance",
	TBKoya:            "Système Koya",
}

// TieBreakLabel returns the French label of a tie-break, or "" if unknown.
func TieBreakLabel(id string) string { return tieBreakLabels[id] }

// TieBreakIDs lists every supported tie-break, in display order.
func TieBreakIDs() []string {
	return []string{TBBuchholzCut1, TBBuchholz, TBBuchholzCut2, TBBuchholzMedian1, TBSonneborn, TBProgressive,
		TBDirectEncounter, TBWins, TBWonGames, TBBlackGames, TBBlackWins, TBARO, TBTPR, TBKoya}
}

// Standing is one line of the ranking table.
type Standing struct {
	Rank      int                `json:"rank"`
	PlayerID  int                `json:"playerId"`
	PairingNo int                `json:"pairingNo"`
	Points    float64            `json:"points"`
	Games     int                `json:"games"`
	TB        map[string]float64 `json:"tieBreaks"`
}

type tbContext struct {
	t        *Tournament
	rounds   []*Round
	score    map[int]float64
	adjusted map[int]float64 // opponents' scores with voluntary unplayed rounds as draws
	pno      map[int]int
}

// Standings computes the ranking after the first n rounds with the
// tournament's tie-breaks, using FIDE C.07 (2023) conventions: an
// opponent's voluntarily unplayed rounds count as draws, and a player's own
// unplayed rounds count against a dummy opponent with the player's score.
// Round-robin free rounds are not games and are ignored.
func (t *Tournament) Standings(n int) []Standing {
	if n > len(t.Rounds) {
		n = len(t.Rounds)
	}
	if n < 0 {
		n = 0
	}
	c := &tbContext{t: t, rounds: t.Rounds[:n], score: map[int]float64{}, adjusted: map[int]float64{}, pno: t.PairingNumbers()}
	players := t.Participants()
	for _, p := range players {
		s, adj := 0.0, 0.0
		for _, r := range c.rounds {
			o, ok := t.OutcomeIn(r, p.ID)
			if !ok {
				adj += t.PointsDraw
				continue
			}
			if o.Free {
				continue
			}
			s += o.Points
			if isVoluntaryUnplayed(o) {
				adj += t.PointsDraw
			} else {
				adj += o.Points
			}
		}
		c.score[p.ID], c.adjusted[p.ID] = s, adj
	}

	rows := make(map[int]*Standing, len(players))
	ids := make([]int, 0, len(players))
	for _, p := range players {
		st := &Standing{PlayerID: p.ID, PairingNo: c.pno[p.ID], Points: c.score[p.ID], TB: map[string]float64{}}
		for _, r := range c.rounds {
			if o, ok := t.OutcomeIn(r, p.ID); ok && o.Played {
				st.Games++
			}
		}
		for _, tb := range t.TieBreaks {
			if tb != TBDirectEncounter {
				st.TB[tb] = c.value(tb, p.ID)
			}
		}
		rows[p.ID] = st
		ids = append(ids, p.ID)
	}

	// Sort by points then by the pairing number, then refine groups of equal
	// points criterion by criterion (direct encounter is computed inside
	// each group).
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := rows[ids[i]], rows[ids[j]]
		if a.Points != b.Points {
			return a.Points > b.Points
		}
		return a.PairingNo < b.PairingNo
	})
	groups := splitBy(ids, func(id int) float64 { return rows[id].Points })
	for _, tb := range t.TieBreaks {
		var next [][]int
		for _, g := range groups {
			if len(g) == 1 {
				next = append(next, g)
				continue
			}
			if tb == TBDirectEncounter {
				for _, id := range g {
					rows[id].TB[tb] = c.directEncounter(id, g)
				}
			}
			sort.SliceStable(g, func(i, j int) bool { return rows[g[i]].TB[tb] > rows[g[j]].TB[tb] })
			next = append(next, splitBy(g, func(id int) float64 { return rows[id].TB[tb] })...)
		}
		groups = next
	}

	out := make([]Standing, 0, len(ids))
	rank := 1
	for _, g := range groups {
		sort.SliceStable(g, func(i, j int) bool { return rows[g[i]].PairingNo < rows[g[j]].PairingNo })
		for _, id := range g {
			st := rows[id]
			st.Rank = rank
			for k, v := range st.TB {
				st.TB[k] = math.Round(v*100) / 100
			}
			out = append(out, *st)
		}
		rank += len(g)
	}
	return out
}

func splitBy(ids []int, key func(int) float64) [][]int {
	var groups [][]int
	for i, id := range ids {
		if i == 0 || key(id) != key(ids[i-1]) {
			groups = append(groups, []int{})
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], id)
	}
	return groups
}

func isVoluntaryUnplayed(o Outcome) bool {
	switch o.Code {
	case 'H', 'Z', 'F', '-':
		return !o.Played
	}
	return false
}

// buchholzParts returns the per-round Buchholz contributions of a player.
func (c *tbContext) buchholzParts(id int) []float64 {
	var parts []float64
	for _, r := range c.rounds {
		o, ok := c.t.OutcomeIn(r, id)
		if ok && o.Free {
			continue
		}
		if ok && o.Played {
			parts = append(parts, c.adjusted[o.Opponent])
			continue
		}
		parts = append(parts, c.score[id])
	}
	return parts
}

func (c *tbContext) value(tb string, id int) float64 {
	t := c.t
	switch tb {
	case TBBuchholz, TBBuchholzCut1, TBBuchholzCut2, TBBuchholzMedian1:
		parts := c.buchholzParts(id)
		sort.Float64s(parts)
		lo, hi := 0, 0
		switch tb {
		case TBBuchholzCut1:
			lo = 1
		case TBBuchholzCut2:
			lo = 2
		case TBBuchholzMedian1:
			lo, hi = 1, 1
		}
		sum := 0.0
		for i := lo; i < len(parts)-hi; i++ {
			sum += parts[i]
		}
		return sum
	case TBSonneborn:
		sum := 0.0
		for _, r := range c.rounds {
			o, ok := t.OutcomeIn(r, id)
			if !ok || o.Free {
				continue
			}
			ratio := o.Points / t.PointsWin
			if o.Played {
				sum += c.adjusted[o.Opponent] * ratio
			} else if t.Kind == KindSwiss {
				sum += c.score[id] * ratio
			}
		}
		return sum
	case TBProgressive:
		run, sum := 0.0, 0.0
		for _, r := range c.rounds {
			if o, ok := t.OutcomeIn(r, id); ok {
				run += o.Points
			}
			sum += run
		}
		return sum
	case TBWins, TBWonGames, TBBlackGames, TBBlackWins:
		n := 0
		for _, r := range c.rounds {
			o, ok := t.OutcomeIn(r, id)
			if !ok || o.Free {
				continue
			}
			win := o.Points == t.PointsWin && o.Code != 'H'
			switch tb {
			case TBWins:
				if win {
					n++
				}
			case TBWonGames:
				if win && o.Played {
					n++
				}
			case TBBlackGames:
				if o.Played && o.Color == 'b' {
					n++
				}
			case TBBlackWins:
				if win && o.Played && o.Color == 'b' {
					n++
				}
			}
		}
		return float64(n)
	case TBARO, TBTPR:
		sum, cnt, pts := 0.0, 0, 0.0
		for _, r := range c.rounds {
			o, ok := t.OutcomeIn(r, id)
			if !ok || !o.Played {
				continue
			}
			rating := 0
			if q := t.Player(o.Opponent); q != nil {
				rating = q.Rating
			}
			if rating == 0 {
				rating = t.UnratedValue
			}
			if rating == 0 {
				continue
			}
			sum += float64(rating)
			cnt++
			pts += o.Points / t.PointsWin
		}
		if cnt == 0 {
			return 0
		}
		aro := math.Round(sum / float64(cnt))
		if tb == TBARO {
			return aro
		}
		return aro + float64(dp(pts/float64(cnt)))
	case TBKoya:
		// points against opponents who scored at least 50% of the maximum
		maxPts := float64(c.gamesPossible()) * t.PointsWin
		sum := 0.0
		for _, r := range c.rounds {
			o, ok := t.OutcomeIn(r, id)
			if !ok || !o.Played {
				continue
			}
			if c.score[o.Opponent] >= maxPts/2 {
				sum += o.Points
			}
		}
		return sum
	}
	return 0
}

// gamesPossible is the number of games a player can play (free rounds of a
// round robin excluded).
func (c *tbContext) gamesPossible() int {
	if c.t.Kind == KindRoundRobin {
		n := 0
		for _, p := range c.t.Players {
			if p.BergerNo > 0 {
				n++
			}
		}
		g := n - 1
		if c.t.DoubleRR {
			g *= 2
		}
		return g
	}
	return len(c.rounds)
}

// directEncounter returns the points a player scored against the other
// members of the group, when every member of the group has met every other
// one; otherwise 0 for all (the criterion does not separate them).
func (c *tbContext) directEncounter(id int, group []int) float64 {
	in := map[int]bool{}
	for _, g := range group {
		in[g] = true
	}
	for _, a := range group {
		for _, b := range group {
			if a < b && !c.t.Met(a, b, len(c.rounds)) {
				return 0
			}
		}
	}
	sum := 0.0
	for _, r := range c.rounds {
		o, ok := c.t.OutcomeIn(r, id)
		if ok && o.Played && in[o.Opponent] {
			sum += o.Points
		}
	}
	return sum
}

// dpTable is FIDE rating regulations table 8.1(a): index = round(p*100)-50.
var dpTable = [51]int{0, 7, 14, 21, 29, 36, 43, 50, 57, 65, 72, 80, 87, 95, 102, 110, 117, 125, 133, 141,
	149, 158, 166, 175, 184, 193, 202, 211, 220, 230, 240, 251, 262, 273, 284, 296, 309, 322, 336, 351,
	366, 383, 401, 422, 444, 470, 501, 538, 589, 677, 800}

func dp(p float64) int {
	pc := int(math.Round(p * 100))
	if pc >= 50 {
		if pc > 100 {
			pc = 100
		}
		return dpTable[pc-50]
	}
	if pc < 0 {
		pc = 0
	}
	return -dpTable[50-pc]
}
