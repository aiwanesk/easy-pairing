package chess

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Title norms, FIDE Title Regulations effective from 1 January 2024
// (Handbook B.01), section 1.4.
type normSpec struct {
	Code      string
	Label     string
	Perf      float64 // 1.4.8 minimum performance before rounding
	MinRa     int     // 1.4.8 minimum average rating of the opponents
	Floor     int     // 1.4.6 adjusted rating floor
	Titled    map[string]bool
	TitledTxt string
	Women     bool
	Excluded  map[string]bool // titles for which this norm is useless
}

var normSpecs = []normSpec{
	{Code: "GM", Label: "GMI", Perf: 2599.5, MinRa: 2380, Floor: 2200, Titled: map[string]bool{"GM": true}, TitledTxt: "GM",
		Excluded: map[string]bool{"GM": true}},
	{Code: "IM", Label: "MI", Perf: 2449.5, MinRa: 2230, Floor: 2050, Titled: map[string]bool{"GM": true, "IM": true}, TitledTxt: "GM ou MI",
		Excluded: map[string]bool{"GM": true, "IM": true}},
	{Code: "WGM", Label: "GMF", Perf: 2399.5, MinRa: 2180, Floor: 2000, Titled: map[string]bool{"GM": true, "IM": true, "WGM": true}, TitledTxt: "GM, MI ou GMF",
		Women: true, Excluded: map[string]bool{"GM": true, "WGM": true}},
	{Code: "WIM", Label: "MIF", Perf: 2249.5, MinRa: 2030, Floor: 1850, Titled: map[string]bool{"GM": true, "IM": true, "WGM": true, "WIM": true}, TitledTxt: "GM, MI, GMF ou MIF",
		Women: true, Excluded: map[string]bool{"GM": true, "IM": true, "WGM": true, "WIM": true}},
}

// titledForNorm: 1.4.5(a) — every title except CM and WCM.
var titledForNorm = map[string]bool{"GM": true, "IM": true, "WGM": true, "WIM": true, "FM": true, "WFM": true}

// NormCondition is one requirement of a norm.
type NormCondition struct {
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// NormStatus is where a player stands for one norm.
type NormStatus struct {
	PlayerID   int             `json:"playerId"`
	Norm       string          `json:"norm"`  // GM | IM | WGM | WIM
	Label      string          `json:"label"` // GMI | MI | GMF | MIF
	Games      int             `json:"games"`
	Score      float64         `json:"score"` // in game points (win = 1)
	Ra         int             `json:"ra"`
	Rp         int             `json:"rp"`
	Achieved   bool            `json:"achieved"`
	Possible   bool            `json:"possible"` // still reachable in the remaining rounds
	Close      bool            `json:"close"`    // few conditions missing
	Missing    int             `json:"missing"`  // number of unmet conditions
	Need       string          `json:"need"`     // what is still needed, in French
	Conditions []NormCondition `json:"conditions"`
}

// NormReport is the norm situation of a tournament after n rounds.
type NormReport struct {
	Round     int          `json:"round"`
	Remaining int          `json:"remaining"`
	Disabled  string       `json:"disabled,omitempty"` // why norms cannot be computed
	Notes     []string     `json:"notes"`
	Players   []NormStatus `json:"players"`
}

type normGame struct {
	opp    *Player
	rating int // FIDE rating, 0 if unrated
	points float64
}

func roundHalfUp(x float64) int { return int(math.Floor(x + 0.5)) }

func fideRating(p *Player) int {
	if p == nil || p.RatingType == "E" || p.RatingType == "N" {
		return 0
	}
	return p.Rating
}

// Norms computes, after n rounds, every norm achieved and those still
// reachable. For a round robin the whole schedule is known: requirements are
// checked on all scheduled opponents and the score needed is given.
func (t *Tournament) Norms(n int) NormReport {
	rep := NormReport{Round: n, Players: []NormStatus{}, Notes: []string{}}
	if t.RatingKind != "standard" {
		rep.Disabled = "les normes de titre ne concernent que les tournois à cadence lente (Elo standard)"
		return rep
	}
	total := t.PlannedRounds()
	if n > len(t.Rounds) {
		n = len(t.Rounds)
	}
	rep.Round = n
	rep.Remaining = total - n
	if total < 9 {
		rep.Disabled = fmt.Sprintf("il faut au moins 9 rondes pour une norme (le tournoi en a %d)", total)
		return rep
	}
	rep.Notes = append(rep.Notes,
		"Calcul selon le règlement FIDE des titres (B.01, en vigueur depuis le 1er janvier 2024), section 1.4.",
		"Seules les parties jouées comptent ; les gains par forfait et les exempts ne comptent pas.",
		"Non pris en compte automatiquement : les points au-delà du minimum comptés comme parties supplémentaires, la possibilité d'ignorer des parties gagnées, l'exemption de fédérations des opens (1.4.3 d), les conditions d'organisation (cadence, 2 rondes par jour au plus, arbitre).")
	rr := t.Kind == KindRoundRobin
	for _, p := range t.Participants() {
		if p.Withdrawn > 0 && p.Withdrawn <= n {
			continue
		}
		games := t.normGames(p, n)
		var future []*Player
		if rr {
			future = t.rrFutureOpponents(p, n)
		}
		for _, spec := range normSpecs {
			if spec.Excluded[p.Title] || spec.Women && !strings.EqualFold(p.Sex, "f") {
				continue
			}
			if st, ok := t.normStatus(p, spec, games, future, rep.Remaining, total); ok {
				rep.Players = append(rep.Players, st)
			}
		}
	}
	sort.SliceStable(rep.Players, func(i, j int) bool {
		a, b := rep.Players[i], rep.Players[j]
		if a.Achieved != b.Achieved {
			return a.Achieved
		}
		if a.Missing != b.Missing {
			return a.Missing < b.Missing
		}
		return a.Rp > b.Rp
	})
	return rep
}

func (t *Tournament) normGames(p *Player, n int) []normGame {
	var out []normGame
	for i := 0; i < n && i < len(t.Rounds); i++ {
		o, ok := t.OutcomeIn(t.Rounds[i], p.ID)
		if !ok || !o.Played {
			continue
		}
		q := t.Player(o.Opponent)
		out = append(out, normGame{opp: q, rating: fideRating(q), points: o.Points / t.PointsWin})
	}
	return out
}

// rrFutureOpponents lists the opponents of the rounds of a round robin not
// played yet.
func (t *Tournament) rrFutureOpponents(p *Player, n int) []*Player {
	var out []*Player
	for i := n; i < len(t.Rounds); i++ {
		for _, b := range t.Rounds[i].Boards {
			if b.White == p.ID && b.Black != 0 {
				out = append(out, t.Player(b.Black))
			} else if b.Black == p.ID && b.White != 0 {
				out = append(out, t.Player(b.White))
			}
		}
	}
	return out
}

// average applies the floor (1.4.6: only the lowest opponent may be raised,
// unrated players count 1400 otherwise) and rounds half up (1.4.7).
func normAverage(ratings []int, floor int) int {
	if len(ratings) == 0 {
		return 0
	}
	rs := append([]int{}, ratings...)
	low := 0
	for i := range rs {
		if rs[i] < rs[low] {
			low = i
		}
	}
	if rs[low] < floor {
		rs[low] = floor
	}
	sum := 0
	for _, r := range rs {
		if r == 0 {
			r = 1400
		}
		sum += r
	}
	return roundHalfUp(float64(sum) / float64(len(rs)))
}

func normPerf(ra int, score float64, games int) (int, int) {
	pct := roundHalfUp(100 * score / float64(games))
	return ra + dp(float64(pct)/100), pct
}

func (t *Tournament) normStatus(p *Player, spec normSpec, games []normGame, future []*Player, remaining, total int) (NormStatus, bool) {
	st := NormStatus{PlayerID: p.ID, Norm: spec.Code, Label: spec.Label, Games: len(games), Conditions: []NormCondition{}}
	// Opponents considered: played ones, plus the scheduled ones in a round robin.
	var opps []*Player
	var ratings []int
	for _, g := range games {
		opps = append(opps, g.opp)
		ratings = append(ratings, g.rating)
		st.Score += g.points
	}
	for _, q := range future {
		opps = append(opps, q)
		ratings = append(ratings, fideRating(q))
	}
	rr := t.Kind == KindRoundRobin
	nGames := len(opps) // games counted for the requirements
	if !rr {
		nGames = len(games)
	}
	if nGames == 0 {
		return st, false
	}
	allRa := normAverage(ratings, spec.Floor)
	playedRa := normAverage(ratings[:len(games)], spec.Floor)
	st.Ra = allRa
	if len(games) > 0 {
		st.Rp, _ = normPerf(playedRa, st.Score, len(games))
		if !rr {
			st.Ra = playedRa
		}
	}

	// Mix of opponents (1.4.3 to 1.4.5).
	feds := map[string]int{}
	own, titled, special := 0, 0, 0
	for _, q := range opps {
		if q == nil {
			continue
		}
		f := strings.ToUpper(strings.TrimSpace(q.Fed))
		feds[f]++
		if f == strings.ToUpper(strings.TrimSpace(p.Fed)) {
			own++
		}
		if titledForNorm[q.Title] {
			titled++
		}
		if spec.Titled[q.Title] {
			special++
		}
	}
	foreign := 0
	maxOne, maxFed := 0, ""
	for f, c := range feds {
		if f != strings.ToUpper(strings.TrimSpace(p.Fed)) && f != "" && f != "FID" {
			foreign++
		}
		if c > maxOne {
			maxOne, maxFed = c, f
		}
	}
	// For a Swiss, requirements are checked on the games already played,
	// with the needs over the final number of games (at least 9).
	final := nGames
	if !rr && final < 9 {
		final = 9
	}
	if final > 13 {
		final = 13
	}
	cond := func(label string, ok bool, detail string) {
		st.Conditions = append(st.Conditions, NormCondition{Label: label, OK: ok, Detail: detail})
		if !ok {
			st.Missing++
		}
	}
	gamesOK := len(games) >= 9 || (len(games) == 8 && total == 9 && t.hadPointWithoutPlaying(p.ID, len(t.Rounds)))
	if rr {
		gamesOK = nGames >= 9
	}
	cond("Parties", gamesOK, fmt.Sprintf("%d partie(s) jouée(s)%s, 9 minimum", len(games), map[bool]string{true: fmt.Sprintf(", %d au calendrier", nGames), false: ""}[rr]))
	cond("Fédérations étrangères", foreign >= 2, fmt.Sprintf("%d fédération(s) autre(s) que %s, 2 minimum", foreign, p.Fed))
	maxOwn := final * 3 / 5
	cond("Adversaires de sa fédération", own <= maxOwn, fmt.Sprintf("%d sur %d, 3/5 au plus (%d)", own, final, maxOwn))
	maxSame := final * 2 / 3
	cond("Adversaires d'une même fédération", maxOne <= maxSame, fmt.Sprintf("%d de %s, 2/3 au plus (%d)", maxOne, maxFed, maxSame))
	needTitled := (final + 1) / 2
	cond("Adversaires titrés", titled >= needTitled, fmt.Sprintf("%d titré(s) (hors CM/WCM), %d au moins (50 %%)", titled, needTitled))
	needSpecial := int(math.Ceil(float64(final) / 3))
	if t.DoubleRR && rr {
		needSpecial = int(math.Ceil(float64(final) / 2))
	}
	if needSpecial < 3 {
		needSpecial = 3
	}
	cond("Titrés "+spec.TitledTxt, special >= needSpecial, fmt.Sprintf("%d, %d au moins (1/3 et au moins 3)", special, needSpecial))
	cond("Moyenne Elo des adversaires", st.Ra >= spec.MinRa, fmt.Sprintf("%d, %d au moins", st.Ra, spec.MinRa))

	perfNeed := int(math.Ceil(spec.Perf))
	if len(games) > 0 {
		pct := roundHalfUp(100 * st.Score / float64(len(games)))
		perfOK := float64(st.Rp) >= spec.Perf && pct >= 35
		cond("Performance", perfOK, fmt.Sprintf("%d (%g/%d, %d %%), %d au moins", st.Rp, st.Score, len(games), pct, perfNeed))
	}
	// Swiss: a title result reached before the last round may ignore the
	// later games (1.4.1); round robin: every scheduled round counts.
	st.Achieved = st.Missing == 0 && (!rr || remaining == 0)

	// What is still needed.
	switch {
	case rr:
		need := neededScore(allRa, nGames, spec.Perf)
		left := float64(len(future))
		switch {
		case need < 0:
			st.Need = "norme impossible même avec le score maximum"
		case need-st.Score > left:
			st.Need = fmt.Sprintf("il faudrait %g/%d : plus atteignable", need, nGames)
		case need <= st.Score:
			st.Need = fmt.Sprintf("score requis %g/%d atteint", need, nGames)
		default:
			st.Need = fmt.Sprintf("il faut %g/%d au total, soit %g dans les %d dernière(s) ronde(s)", need, nGames, need-st.Score, len(future))
		}
		st.Possible = need >= 0 && need-st.Score <= left
		// Mix conditions are fixed by the schedule: only the score can change.
		mixMissing := 0
		for _, c := range st.Conditions {
			if !c.OK && c.Label != "Performance" && c.Label != "Parties" {
				mixMissing++
			}
		}
		if mixMissing > 0 {
			st.Possible = false
		}
	case remaining > 0:
		ra := st.Ra
		if ra < spec.MinRa {
			ra = spec.MinRa
		}
		games9 := len(games) + remaining
		if games9 > 13 {
			games9 = 13
		}
		need := neededScore(ra, games9, spec.Perf)
		left := float64(remaining)
		if need < 0 || need-st.Score > left {
			st.Need = fmt.Sprintf("plus atteignable : il faudrait %g/%d avec une moyenne de %d", math.Max(need, 0), games9, ra)
		} else {
			st.Need = fmt.Sprintf("avec une moyenne de %d : %g/%d, soit %g sur les %d rondes restantes", ra, need, games9, math.Max(need-st.Score, 0), remaining)
			st.Possible = true
		}
		var mix []string
		for _, c := range st.Conditions {
			if !c.OK && c.Label != "Performance" && c.Label != "Parties" && c.Label != "Moyenne Elo des adversaires" {
				mix = append(mix, strings.ToLower(c.Label))
			}
		}
		if len(mix) > 0 {
			st.Need += " ; à compléter : " + strings.Join(mix, ", ")
		}
	default:
		if !st.Achieved {
			st.Need = "non obtenue"
		}
	}
	st.Close = !st.Achieved && st.Possible && st.Missing <= 3
	// Players far from anything are not listed.
	if !st.Achieved && !st.Possible {
		return st, len(games) > 0 && st.Rp >= int(spec.Perf)-100 && st.Missing <= 2
	}
	return st, st.Achieved || st.Possible
}

// neededScore returns the smallest score (in half points) over games games
// giving a performance of at least perf against an average ra, with at
// least 35 %; -1 if even the full score is not enough.
func neededScore(ra, games int, perf float64) float64 {
	for half := 0; half <= 2*games; half++ {
		s := float64(half) / 2
		rp, pct := normPerf(ra, s, games)
		if float64(rp) >= perf && pct >= 35 {
			return s
		}
	}
	return -1
}
