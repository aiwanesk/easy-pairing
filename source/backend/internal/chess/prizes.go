package chess

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// PrizeAward is the allocation of one prize by the tie-break ranking.
type PrizeAward struct {
	Prize    Prize `json:"prize"`
	PlayerID int   `json:"playerId"` // 0 = not awarded (no eligible player)
	// Tied lists eligible players without money who have the same points
	// as the winner (Sharly Chess issue #1061), so the arbiter can see who
	// was separated by tie-breaks.
	Tied []int `json:"tied"`
}

// Payout is the money one player receives.
type Payout struct {
	PlayerID int     `json:"playerId"`
	Rank     int     `json:"rank"`
	Points   float64 `json:"points"`
	PrizeID  int     `json:"prizeId"` // prize won by tie-break (0 = none: share of a tie only)
	Nominal  float64 `json:"nominal"` // amount of that prize
	Amount   float64 `json:"amount"`  // amount actually paid after the split
	// SharedWith lists the other players of the tie the money was split
	// with (empty in "place" mode or without a tie).
	SharedWith []int  `json:"sharedWith"`
	Detail     string `json:"detail"`
}

// PrizeResult is the full prize allocation after n rounds.
type PrizeResult struct {
	Split   string       `json:"split"`
	Round   int          `json:"round"`
	Awards  []PrizeAward `json:"awards"`
	Payouts []Payout     `json:"payouts"`
	Total   float64      `json:"total"`
}

func csvSet(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.ToLower(strings.TrimSpace(x)); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// Eligible reports whether a player meets the conditions of a prize
// (score excepted: see EligibleWith).
func (pr Prize) Eligible(p *Player) bool {
	if cats := csvSet(pr.Category); len(cats) > 0 {
		pc := strings.ToLower(p.Category)
		ok := false
		for _, c := range cats {
			if strings.HasPrefix(pc, c) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if pr.MinRating > 0 && p.Rating < pr.MinRating {
		return false
	}
	if pr.MaxRating > 0 && p.Rating > pr.MaxRating {
		return false
	}
	switch pr.Sex {
	case "f":
		if !strings.EqualFold(p.Sex, "f") {
			return false
		}
	case "m":
		if strings.EqualFold(p.Sex, "f") {
			return false
		}
	}
	if feds := csvSet(pr.Fed); len(feds) > 0 {
		pf := strings.ToLower(strings.TrimSpace(p.Fed))
		ok := false
		for _, f := range feds {
			if f == pf {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if pr.Club != "" && !strings.EqualFold(strings.TrimSpace(p.Club), strings.TrimSpace(pr.Club)) {
		return false
	}
	if pr.Unrated && p.Rating > 0 && p.RatingType != "E" {
		return false
	}
	return true
}

// EligibleWith also checks the minimum score.
func (pr Prize) EligibleWith(p *Player, points float64) bool {
	return pr.Eligible(p) && (pr.MinPoints <= 0 || points >= pr.MinPoints-1e-9)
}

// GroupKey identifies prizes sharing the same conditions.
func (pr Prize) GroupKey() string {
	norm := func(s string) string {
		xs := csvSet(s)
		sort.Strings(xs)
		return strings.Join(xs, ",")
	}
	return fmt.Sprintf("%s|%d|%d|%s|%s|%s|%t|%g", norm(pr.Category), pr.MinRating, pr.MaxRating, pr.Sex, norm(pr.Fed),
		strings.ToLower(strings.TrimSpace(pr.Club)), pr.Unrated, pr.MinPoints)
}

// Conditions describes the conditions of a prize in French.
func (pr Prize) Conditions() string {
	var c []string
	if pr.Category != "" {
		c = append(c, "catégories "+strings.Join(csvSet(pr.Category), ", "))
	}
	switch {
	case pr.MinRating > 0 && pr.MaxRating > 0:
		c = append(c, fmt.Sprintf("Elo %d à %d", pr.MinRating, pr.MaxRating))
	case pr.MinRating > 0:
		c = append(c, fmt.Sprintf("Elo ≥ %d", pr.MinRating))
	case pr.MaxRating > 0:
		c = append(c, fmt.Sprintf("Elo ≤ %d", pr.MaxRating))
	}
	switch pr.Sex {
	case "f":
		c = append(c, "féminines")
	case "m":
		c = append(c, "masculins")
	}
	if pr.Fed != "" {
		c = append(c, "fédération "+strings.ToUpper(strings.Join(csvSet(pr.Fed), ", ")))
	}
	if pr.Club != "" {
		c = append(c, "club "+pr.Club)
	}
	if pr.Unrated {
		c = append(c, "non classés")
	}
	if pr.MinPoints > 0 {
		c = append(c, fmt.Sprintf("au moins %g pt", pr.MinPoints))
	}
	if len(c) == 0 {
		return "classement général"
	}
	return strings.Join(c, ", ")
}

func cents(x float64) int64 { return int64(math.Round(x * 100)) }

// AllocatePrizes distributes prizes on the standings after n rounds.
//
// Winners: prizes are handled from the highest amount to the lowest (list
// order breaks ties); each goes to the best-ranked eligible player who has
// no prize yet, so a player never receives two prizes and gets the highest
// one he can.
//
// Money, by t.PrizeSplit:
//   - place: each winner receives his prize;
//   - shared: inside each group of prizes with the same conditions,
//     players tied on points share equally the money of the places they
//     occupy (tied players beyond the last paid place included);
//   - hort: same tie, half of each prize goes to its winner by tie-break,
//     the other half of the money is shared equally among the tied players.
//
// Amounts are computed in cents; rounding remainders go to the best ranked.
func (t *Tournament) AllocatePrizes(n int) PrizeResult {
	st := t.Standings(n)
	res := PrizeResult{Split: t.PrizeSplit, Round: n, Awards: make([]PrizeAward, len(t.Prizes)), Payouts: []Payout{}}
	if res.Split == "" {
		res.Split = PrizeSplitPlace
	}
	rank := map[int]int{}
	points := map[int]float64{}
	for i, s := range st {
		rank[s.PlayerID] = i
		points[s.PlayerID] = s.Points
	}
	active := func(p *Player) bool { return p != nil && !(p.Withdrawn > 0 && p.Withdrawn <= n) }

	order := make([]int, len(t.Prizes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return t.Prizes[order[i]].Amount > t.Prizes[order[j]].Amount })
	winnerOf := map[int]int{} // player -> prize index
	for _, idx := range order {
		pr := t.Prizes[idx]
		award := PrizeAward{Prize: pr, Tied: []int{}}
		for _, s := range st {
			p := t.Player(s.PlayerID)
			if !active(p) {
				continue
			}
			if _, has := winnerOf[p.ID]; has || !pr.EligibleWith(p, s.Points) {
				continue
			}
			award.PlayerID = p.ID
			winnerOf[p.ID] = idx
			break
		}
		res.Awards[idx] = award
	}

	// Money per player, in cents.
	pay := map[int]int64{}
	shared := map[int][]int{}
	detail := map[int]string{}
	for pid, idx := range winnerOf {
		pay[pid] = cents(t.Prizes[idx].Amount)
	}

	if res.Split != PrizeSplitPlace {
		// Groups of prizes with identical conditions, by decreasing top amount.
		type group struct {
			key  string
			idxs []int
			top  float64
		}
		var groups []*group
		byKey := map[string]*group{}
		for _, idx := range order {
			k := t.Prizes[idx].GroupKey()
			g := byKey[k]
			if g == nil {
				g = &group{key: k, top: t.Prizes[idx].Amount}
				byKey[k] = g
				groups = append(groups, g)
			}
			g.idxs = append(g.idxs, idx)
		}
		consumed := map[int]bool{} // tied players without a prize already sharing in a group
		for _, g := range groups {
			cond := t.Prizes[g.idxs[0]]
			var winners []int
			for _, idx := range g.idxs {
				if pid := res.Awards[idx].PlayerID; pid != 0 {
					winners = append(winners, pid)
				}
			}
			if len(winners) == 0 {
				continue
			}
			winnerSet := map[int]bool{}
			for _, w := range winners {
				winnerSet[w] = true
			}
			// Clusters of equal points among winners, plus tied eligible
			// players without any prize.
			byPts := map[float64][]int{}
			for _, w := range winners {
				byPts[points[w]] = append(byPts[points[w]], w)
			}
			for pts, members := range byPts {
				for _, s := range st {
					if s.Points != pts || winnerSet[s.PlayerID] || consumed[s.PlayerID] {
						continue
					}
					p := t.Player(s.PlayerID)
					if _, has := winnerOf[s.PlayerID]; has || !active(p) || !cond.EligibleWith(p, s.Points) {
						continue
					}
					members = append(members, s.PlayerID)
				}
				if len(members) < 2 {
					continue
				}
				sort.Slice(members, func(i, j int) bool { return rank[members[i]] < rank[members[j]] })
				var pool int64
				for _, m := range members {
					pool += pay[m]
				}
				k := int64(len(members))
				newPay := map[int]int64{}
				var common int64
				if res.Split == PrizeSplitShared {
					common = pool
				} else {
					for _, m := range members {
						half := pay[m] / 2
						newPay[m] = half
						common += pay[m] - half
					}
				}
				each, rest := common/k, common%k
				for i, m := range members {
					newPay[m] += each
					if int64(i) < rest {
						newPay[m]++
					}
				}
				for _, m := range members {
					pay[m] = newPay[m]
					var others []int
					for _, o := range members {
						if o != m {
							others = append(others, o)
						}
					}
					shared[m] = others
					if !winnerSet[m] {
						consumed[m] = true
					}
					mode := "partage égal"
					if res.Split == PrizeSplitHort {
						mode = "système Hort"
					}
					detail[m] = fmt.Sprintf("%s entre %d ex aequo à %g pt (%s)", mode, len(members), pts, cond.Conditions())
				}
			}
		}
	}

	paid := map[int]bool{}
	for pid, c := range pay {
		po := Payout{PlayerID: pid, Rank: rank[pid] + 1, Points: points[pid], Amount: float64(c) / 100, SharedWith: shared[pid], Detail: detail[pid]}
		if po.SharedWith == nil {
			po.SharedWith = []int{}
		}
		if idx, has := winnerOf[pid]; has {
			po.PrizeID = t.Prizes[idx].ID
			po.Nominal = t.Prizes[idx].Amount
		}
		res.Payouts = append(res.Payouts, po)
		res.Total += po.Amount
		paid[pid] = true
	}
	sort.Slice(res.Payouts, func(i, j int) bool { return res.Payouts[i].Rank < res.Payouts[j].Rank })
	res.Total = math.Round(res.Total*100) / 100

	// Ties without money, for each awarded prize.
	for i := range res.Awards {
		a := &res.Awards[i]
		if a.PlayerID == 0 {
			continue
		}
		for _, s := range st {
			p := t.Player(s.PlayerID)
			if active(p) && !paid[p.ID] && s.Points == points[a.PlayerID] && a.Prize.EligibleWith(p, s.Points) {
				a.Tied = append(a.Tied, p.ID)
			}
		}
	}
	return res
}
