package chess

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
)

// varmaTables are the FIDE Varma tables (C.05 Annex 2) for 9/10 to 23/24
// players: groups A, B, C, D of Berger numbers. Players of the same group
// meet only in the first rounds, which keeps compatriots apart at the end.
var varmaTables = map[int][4][]int{
	10: {{3, 4, 8}, {5, 7, 9}, {1, 6}, {2, 10}},
	12: {{4, 5, 9, 10}, {1, 2, 7}, {6, 8, 12}, {3, 11}},
	14: {{4, 5, 6, 11, 12}, {1, 2, 8, 9}, {7, 10, 13}, {3, 14}},
	16: {{5, 6, 7, 12, 13, 14}, {1, 2, 3, 9, 10}, {8, 11, 15}, {4, 16}},
	18: {{5, 6, 7, 8, 14, 15, 16}, {1, 2, 3, 10, 11, 12}, {9, 13, 17}, {4, 18}},
	20: {{6, 7, 8, 9, 15, 16, 17, 18}, {1, 2, 3, 11, 12, 13, 14}, {5, 10, 19}, {4, 20}},
	22: {{6, 7, 8, 9, 10, 17, 18, 19, 20}, {1, 2, 3, 4, 12, 13, 14, 15}, {11, 16, 21}, {5, 22}},
	24: {{6, 7, 8, 9, 10, 11, 19, 20, 21, 22}, {1, 2, 3, 4, 13, 14, 15, 16, 17}, {12, 18, 23}, {5, 24}},
}

// VarmaDraw performs the restricted drawing of lots of FIDE C.05 Annex 2:
// federations draw in decreasing order of size (ties: FIDE code), each one
// takes an envelope (group) with enough numbers for all its players, its
// players draw their numbers from it (alphabetical order); unused numbers
// stay available. Players alone in their federation draw last from what is
// left. Returns player id -> Berger number and a description of the draw.
func VarmaDraw(players []*Player, rng *rand.Rand) (map[int]int, []string, error) {
	n := len(players)
	size := n
	if size%2 == 1 {
		size++
	}
	table, ok := varmaTables[size]
	if !ok {
		return nil, nil, fmt.Errorf("les tables de Varma couvrent 9 à 24 joueurs (ici %d)", n)
	}
	groups := make([][]int, 4)
	for g := range table {
		for _, k := range table[g] {
			if k <= n || n == size { // with an odd count, the highest number is the bye
				groups[g] = append(groups[g], k)
			}
		}
	}
	letters := "ABCD"
	byFed := map[string][]*Player{}
	for _, p := range players {
		f := strings.ToUpper(strings.TrimSpace(p.Fed))
		byFed[f] = append(byFed[f], p)
	}
	feds := make([]string, 0, len(byFed))
	for f := range byFed {
		feds = append(feds, f)
	}
	sort.Slice(feds, func(i, j int) bool {
		if len(byFed[feds[i]]) != len(byFed[feds[j]]) {
			return len(byFed[feds[i]]) > len(byFed[feds[j]])
		}
		return feds[i] < feds[j]
	})
	out := map[int]int{}
	var log []string
	take := func(g int) int {
		i := rng.IntN(len(groups[g]))
		k := groups[g][i]
		groups[g] = append(groups[g][:i], groups[g][i+1:]...)
		return k
	}
	var singles []*Player
	for _, f := range feds {
		ps := byFed[f]
		sort.Slice(ps, func(i, j int) bool { return ps[i].FullName() < ps[j].FullName() })
		if len(ps) == 1 {
			singles = append(singles, ps[0])
			continue
		}
		// Envelopes with enough numbers for the whole federation.
		var fit []int
		for g := range groups {
			if len(groups[g]) >= len(ps) {
				fit = append(fit, g)
			}
		}
		if len(fit) == 0 {
			// Too many compatriots for one group: fill the largest group,
			// then put the rest in the smallest group that can hold it, to
			// keep the large groups for the next federations.
			left := ps
			for len(left) > 0 {
				best := -1
				for g := range groups {
					if len(groups[g]) >= len(left) && (best < 0 || len(groups[g]) < len(groups[best])) {
						best = g
					}
				}
				if best < 0 {
					for g := range groups {
						if len(groups[g]) > 0 && (best < 0 || len(groups[g]) > len(groups[best])) {
							best = g
						}
					}
				}
				if best < 0 {
					return nil, nil, errors.New("plus de numéros disponibles")
				}
				var names []string
				for len(left) > 0 && len(groups[best]) > 0 {
					out[left[0].ID] = take(best)
					names = append(names, fmt.Sprintf("%s n°%d", left[0].FullName(), out[left[0].ID]))
					left = left[1:]
				}
				log = append(log, fmt.Sprintf("%s (%d joueurs, plus qu'un groupe) : groupe %c — %s", f, len(ps), letters[best], strings.Join(names, ", ")))
			}
			continue
		}
		g := fit[rng.IntN(len(fit))]
		var names []string
		for _, p := range ps {
			out[p.ID] = take(g)
			names = append(names, fmt.Sprintf("%s n°%d", p.FullName(), out[p.ID]))
		}
		log = append(log, fmt.Sprintf("%s (%d joueurs) : groupe %c — %s", f, len(ps), letters[g], strings.Join(names, ", ")))
	}
	var pool []int
	for g := range groups {
		pool = append(pool, groups[g]...)
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	sort.Slice(singles, func(i, j int) bool { return singles[i].FullName() < singles[j].FullName() })
	var names []string
	for i, p := range singles {
		out[p.ID] = pool[i]
		names = append(names, fmt.Sprintf("%s (%s) n°%d", p.FullName(), p.Fed, pool[i]))
	}
	if len(names) > 0 {
		log = append(log, "Autres joueurs : "+strings.Join(names, ", "))
	}
	return out, log, nil
}

// LastDraw describes the last restricted drawing of lots (Varma).
func (t *Tournament) LastDraw() []string { return t.lastDraw }
