package chess

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

func TestVarmaDraw(t *testing.T) {
	for seed := uint64(1); seed <= 50; seed++ {
		for _, n := range []int{9, 10, 12, 15, 24} {
			var ps []*Player
			feds := []string{"FRA", "FRA", "FRA", "FRA", "GER", "GER", "GER", "SUI", "ITA", "ESP", "UKR", "NED", "BEL", "POL", "CZE", "AUT", "HUN", "ROU", "SRB", "CRO", "SLO", "SVK", "DEN", "SWE"}
			for i := 0; i < n; i++ {
				ps = append(ps, &Player{ID: i + 1, LastName: fmt.Sprintf("P%02d", i), Fed: feds[i]})
			}
			nums, _, err := VarmaDraw(ps, rand.New(rand.NewPCG(seed, 7)))
			if err != nil {
				t.Fatal(err)
			}
			size := n + n%2
			seen := map[int]bool{}
			for _, k := range nums {
				if k < 1 || k > n || seen[k] {
					t.Fatalf("n=%d seed=%d: bad or duplicate number %d (%v)", n, seed, k, nums)
				}
				seen[k] = true
			}
			group := func(k int) int {
				for g, ks := range varmaTables[size] {
					for _, x := range ks {
						if x == k {
							return g
						}
					}
				}
				return -1
			}
			// FRA (4) and GER (3) must each be inside one group when a group is large enough.
			avail := 0
			for _, ks := range varmaTables[size] {
				c := 0
				for _, x := range ks {
					if x <= n {
						c++
					}
				}
				if c > avail {
					avail = c
				}
			}
			for _, ids := range [][]int{{1, 2, 3, 4}, {5, 6, 7}} {
				if len(ids) > avail {
					continue
				}
				g := group(nums[ids[0]])
				for _, id := range ids[1:] {
					if id <= n && group(nums[id]) != g {
						t.Fatalf("n=%d seed=%d: federation split across groups: %v", n, seed, nums)
					}
				}
			}
		}
	}
}

func TestRoundRobinOddVarma(t *testing.T) {
	tr := NewTournament("RR")
	tr.Kind = KindRoundRobin
	tr.Normalize()
	for i, f := range []string{"FRA", "FRA", "FRA", "GER", "GER", "SUI", "ITA", "ESP", "UKR"} {
		tr.AddPlayer(&Player{LastName: fmt.Sprintf("P%d", i), Fed: f, Rating: 2400 - i})
	}
	if err := tr.StartRoundRobinWith("varma", 3, nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range tr.Players {
		if p.BergerNo < 1 || p.BergerNo > 9 {
			t.Fatalf("Berger number %d for %s (10 is the bye)", p.BergerNo, p.LastName)
		}
	}
	if len(tr.Rounds) != 9 || len(tr.LastDraw()) == 0 {
		t.Fatalf("rounds %d draw %v", len(tr.Rounds), tr.LastDraw())
	}
}
