package chess

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// rrPlayers returns the players taking part in a round robin about to start.
func (t *Tournament) rrPlayers() []*Player {
	var out []*Player
	for _, p := range t.Seeding() {
		if p.Withdrawn > 0 || (t.CheckIn && !p.CheckedIn) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// BergerSchedule returns, for n players (n even), the FIDE Berger tables
// (C.05 Annex 1): rounds[r][b] = {white, black} as 1-based Berger numbers.
func BergerSchedule(n int) [][][2]int {
	if n%2 == 1 {
		n++
	}
	rounds := make([][][2]int, n-1)
	m := n - 1
	for r := 1; r <= n-1; r++ {
		a := ((r - 1) * n / 2) % m
		var boards [][2]int
		first := [2]int{a + 1, n}
		if r%2 == 0 {
			first = [2]int{n, a + 1}
		}
		boards = append(boards, first)
		for k := 1; k < n/2; k++ {
			p := (a+k)%m + 1
			q := ((a-k)%m+m)%m + 1
			boards = append(boards, [2]int{p, q})
		}
		rounds[r-1] = boards
	}
	return rounds
}

// StartRoundRobin assigns Berger numbers and creates every round. order is
// "rating" (initial ranking) or "lots" (random draw).
func (t *Tournament) StartRoundRobin(order string, seed uint64) error {
	return t.StartRoundRobinWith(order, seed, nil)
}

// StartRoundRobinWith also accepts "varma" (restricted drawing of lots,
// FIDE C.05 Annex 2) and "manual" (numbers drawn by the arbiter, given as
// player id -> number).
func (t *Tournament) StartRoundRobinWith(order string, seed uint64, manual map[int]int) error {
	if t.Kind != KindRoundRobin {
		return errors.New("ce tournoi n'est pas un toutes-rondes")
	}
	if t.Started() {
		return errors.New("le toutes-rondes est déjà commencé")
	}
	players := t.rrPlayers()
	if len(players) < 2 {
		return errors.New("il faut au moins deux joueurs (pointés) pour commencer")
	}
	n := len(players)
	ids := make([]int, n+2) // berger number -> player id (0 = free round)
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	switch order {
	case "lots":
		rng.Shuffle(len(players), func(i, j int) { players[i], players[j] = players[j], players[i] })
		for i, p := range players {
			ids[i+1] = p.ID
		}
	case "varma":
		nums, log, err := VarmaDraw(players, rng)
		if err != nil {
			return err
		}
		for id, k := range nums {
			ids[k] = id
		}
		t.lastDraw = log
	case "manual":
		used := map[int]bool{}
		max := n // with an odd count, number n+1 is the free round (bye)
		for _, p := range players {
			k := manual[p.ID]
			if k < 1 || k > max {
				return fmt.Errorf("%s : numéro %d hors de 1 à %d", p.FullName(), k, max)
			}
			if used[k] {
				return fmt.Errorf("le numéro %d est attribué deux fois", k)
			}
			used[k] = true
			ids[k] = p.ID
		}
	default:
		for i, p := range players {
			ids[i+1] = p.ID
		}
	}
	schedule := BergerSchedule(n)
	cycles := [][][][2]int{schedule}
	if t.DoubleRR {
		second := make([][][2]int, len(schedule))
		for r, boards := range schedule {
			rev := make([][2]int, len(boards))
			for b, pr := range boards {
				rev[b] = [2]int{pr[1], pr[0]}
			}
			second[r] = rev
		}
		if t.ReverseLastRR && len(schedule) >= 2 {
			// FIDE recommendation: swap the last two rounds of the first
			// cycle to avoid three games in a row with the same colour.
			k := len(schedule)
			first := append([][][2]int{}, schedule...)
			first[k-1], first[k-2] = first[k-2], first[k-1]
			cycles[0] = first
		}
		cycles = append(cycles, second)
	}
	now := time.Now().UTC()
	number := 0
	for _, cyc := range cycles {
		for _, boards := range cyc {
			number++
			round := &Round{Number: number, Absences: map[int]string{}, PairedAt: now}
			bn := 0
			var free *Board
			for _, pr := range boards {
				w, b := ids[pr[0]], ids[pr[1]]
				if w == 0 || b == 0 {
					id := w + b
					free = &Board{White: id, Result: ResultFreeRound}
					continue
				}
				bn++
				round.Boards = append(round.Boards, &Board{Number: bn, White: w, Black: b})
			}
			if free != nil {
				free.Number = bn + 1
				round.Boards = append(round.Boards, free)
			}
			t.Rounds = append(t.Rounds, round)
		}
	}
	t.TotalRounds = len(t.Rounds)
	berger := map[int]int{}
	for i := 1; i <= n+1; i++ {
		if ids[i] != 0 {
			berger[ids[i]] = i
		}
	}
	for _, p := range t.Players {
		p.BergerNo = berger[p.ID]
	}
	return nil
}

// ResetRoundRobin removes every round (only if no result was entered).
func (t *Tournament) ResetRoundRobin() error {
	for _, r := range t.Rounds {
		for _, b := range r.Boards {
			if b.Black != 0 && b.Result != "" {
				return errors.New("des résultats sont déjà saisis : impossible de réinitialiser")
			}
		}
		for _, m := range r.TeamMatches {
			for _, g := range m.Games {
				if g.White != 0 && g.Black != 0 && g.Result != "" {
					return errors.New("des résultats sont déjà saisis : impossible de réinitialiser")
				}
			}
		}
	}
	t.Rounds = []*Round{}
	for _, p := range t.Players {
		p.BergerNo = 0
	}
	return nil
}
