package chess_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"pairingeasy/internal/bbp"
	"pairingeasy/internal/chess"
)

func engine(t *testing.T) *bbp.Engine {
	t.Helper()
	path := os.Getenv("BBP_PAIRINGS")
	if path == "" {
		path = "../../bin/bbpPairings"
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("bbpPairings non disponible (BBP_PAIRINGS)")
	}
	e := &bbp.Engine{}
	e.Set(path, "test", "")
	return e
}

func pairNext(t *testing.T, e *bbp.Engine, tr *chess.Tournament) *chess.Round {
	t.Helper()
	in, err := tr.PrepareNextSwissRound()
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Pair(context.Background(), tr.System, in.TRF)
	if err != nil {
		t.Fatalf("round %d: %v\n%s", in.Round, err, in.TRF)
	}
	r, err := tr.ApplySwissPairing(in, out)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func play(rng *rand.Rand, r *chess.Round) {
	for _, b := range r.Boards {
		if b.Black == 0 {
			continue
		}
		switch x := rng.Intn(20); {
		case x == 0:
			b.Result = chess.ResultWhiteForfeit
		case x < 8:
			b.Result = chess.ResultWhiteWins
		case x < 14:
			b.Result = chess.ResultBlackWins
		default:
			b.Result = chess.ResultDraw
		}
	}
	r.Published = true
}

func newSwiss(n int, rng *rand.Rand) *chess.Tournament {
	tr := chess.NewTournament("Open test")
	tr.TotalRounds = 9
	for i := 0; i < n; i++ {
		p := &chess.Player{LastName: fmt.Sprintf("Joueur%02d", i), FirstName: "Élodie", Rating: 1400 + rng.Intn(1000), Fed: "FRA", Club: fmt.Sprintf("Club %d", i%4)}
		if err := tr.AddPlayer(p); err != nil {
			panic(err)
		}
	}
	return tr
}

// TestSwissFull simulates 9-round opens (Dutch, Burstein, Baku) with byes,
// forfeits, withdrawal, late entry, pending registrations, check-in and
// forbidden pairs, then has bbpPairings re-check every round (-c).
func TestSwissFull(t *testing.T) {
	for _, c := range []struct{ system, accel string }{
		{chess.SystemDutch, chess.AccelNone}, {chess.SystemBurstein, chess.AccelNone}, {chess.SystemDutch, chess.AccelBaku},
	} {
		t.Run(c.system+"-"+c.accel, func(t *testing.T) {
			e := engine(t)
			rng := rand.New(rand.NewSource(42))
			tr := newSwiss(23, rng)
			tr.System, tr.Acceleration = c.system, c.accel
			tr.CheckIn = true
			for _, p := range tr.Players {
				p.CheckedIn = p.ID != 2 // player 2 forgot to check in: absent in round 1
			}
			tr.Players[3].Byes[2] = chess.ByeHalf
			tr.Players[5].Byes[4] = chess.ByeZero
			tr.Players[7].Withdrawn = 6
			tr.ForbiddenGroups = [][]int{{tr.Players[0].ID, tr.Players[1].ID, tr.Players[4].ID}}
			pending := &chess.Player{LastName: "EnAttente", Rating: 2000, Status: chess.StatusPending}
			if err := tr.AddPlayer(pending); err != nil {
				t.Fatal(err)
			}
			forbidden := map[[2]int]bool{}
			for _, a := range tr.ForbiddenGroups[0] {
				for _, b := range tr.ForbiddenGroups[0] {
					forbidden[[2]int{a, b}] = true
				}
			}
			met := map[[2]int]bool{}
			for round := 1; round <= tr.TotalRounds; round++ {
				if round == 5 {
					if err := tr.AddPlayer(&chess.Player{LastName: "Retardataire", Rating: 1800}); err != nil {
						t.Fatal(err)
					}
				}
				r := pairNext(t, e, tr)
				seen := map[int]bool{}
				for _, b := range r.Boards {
					for _, id := range []int{b.White, b.Black} {
						if id == 0 {
							continue
						}
						if id == pending.ID {
							t.Fatal("pending registration was paired")
						}
						if seen[id] {
							t.Fatalf("round %d: player %d paired twice", round, id)
						}
						seen[id] = true
					}
					if b.Black == 0 {
						continue
					}
					k := [2]int{min(b.White, b.Black), max(b.White, b.Black)}
					if met[k] {
						t.Fatalf("round %d: %v already met", round, k)
					}
					if forbidden[k] {
						t.Fatalf("round %d: forbidden pair %v", round, k)
					}
					met[k] = true
				}
				if round == 1 && r.Absences[2] != chess.ByeZero {
					t.Fatalf("unchecked player not absent in round 1: %v", r.Absences)
				}
				if round == 2 && r.Absences[tr.Players[3].ID] != chess.ByeHalf {
					t.Fatal("half bye not applied")
				}
				if _, err := tr.PrepareNextSwissRound(); err == nil {
					t.Fatal("pairing allowed with pending results")
				}
				play(rng, r)
				for _, b := range r.Boards {
					if b.Result == chess.ResultWhiteForfeit {
						delete(met, [2]int{min(b.White, b.Black), max(b.White, b.Black)})
					}
				}
			}
			out, err := e.Check(context.Background(), tr.System, tr.ExportTRF(nil))
			if err != nil || out != "" {
				t.Fatalf("bbpPairings -c: %v\n%s\n%s", err, out, tr.ExportTRF(nil))
			}
			st := tr.Standings(len(tr.Rounds))
			if len(st) != len(tr.Participants()) {
				t.Fatalf("standings size %d", len(st))
			}
			for i := 1; i < len(st); i++ {
				if st[i].Points > st[i-1].Points || st[i].Rank < st[i-1].Rank {
					t.Fatal("standings not sorted")
				}
			}
		})
	}
}

func TestBakuAccelerationLines(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := newSwiss(10, rng)
	tr.Acceleration = chess.AccelBaku
	in, err := tr.PrepareNextSwissRound()
	if err != nil {
		t.Fatal(err)
	}
	// 10 players: GA = 2*ceil(10/4) = 6; 9 rounds: 5 accelerated, 3 at 1 VP.
	if !strings.Contains(in.TRF, "XXA    1  1.0  1.0  1.0  0.5  0.5  0.0  0.0  0.0  0.0") {
		t.Fatalf("XXA line for player 1:\n%s", in.TRF)
	}
	if !strings.Contains(in.TRF, "XXA    7  0.0  0.0") {
		t.Fatalf("XXA line for player 7:\n%s", in.TRF)
	}
}

func TestBergerTables(t *testing.T) {
	want6 := [][][2]int{
		{{1, 6}, {2, 5}, {3, 4}},
		{{6, 4}, {5, 3}, {1, 2}},
		{{2, 6}, {3, 1}, {4, 5}},
		{{6, 5}, {1, 4}, {2, 3}},
		{{3, 6}, {4, 2}, {5, 1}},
	}
	got := chess.BergerSchedule(6)
	if fmt.Sprint(got) != fmt.Sprint(want6) {
		t.Fatalf("Berger 6:\n got %v\nwant %v", got, want6)
	}
	for _, n := range []int{4, 8, 10, 12, 14, 16} {
		s := chess.BergerSchedule(n)
		met := map[[2]int]int{}
		for _, r := range s {
			seen := map[int]bool{}
			for _, b := range r {
				if seen[b[0]] || seen[b[1]] {
					t.Fatalf("n=%d: player twice in a round", n)
				}
				seen[b[0]], seen[b[1]] = true, true
				met[[2]int{min(b[0], b[1]), max(b[0], b[1])}]++
			}
		}
		if len(met) != n*(n-1)/2 {
			t.Fatalf("n=%d: %d distinct pairs", n, len(met))
		}
	}
}

func TestRoundRobin(t *testing.T) {
	tr := chess.NewTournament("Fermé")
	tr.Kind = chess.KindRoundRobin
	tr.DoubleRR, tr.ReverseLastRR = true, true
	tr.TieBreaks = nil
	tr.Normalize()
	for i := 0; i < 5; i++ {
		tr.AddPlayer(&chess.Player{LastName: fmt.Sprintf("P%d", i), Rating: 2400 - 10*i})
	}
	if tr.PlannedRounds() != 10 {
		t.Fatalf("planned rounds %d", tr.PlannedRounds())
	}
	if err := tr.StartRoundRobin("rating", 0); err != nil {
		t.Fatal(err)
	}
	if len(tr.Rounds) != 10 {
		t.Fatalf("rounds %d", len(tr.Rounds))
	}
	games := map[[2]int]int{}
	for _, r := range tr.Rounds {
		free := 0
		for _, b := range r.Boards {
			if b.Black == 0 {
				free++
				continue
			}
			games[[2]int{min(b.White, b.Black), max(b.White, b.Black)}]++
			b.Result = chess.ResultDraw
		}
		if free != 1 {
			t.Fatalf("round %d: %d free", r.Number, free)
		}
	}
	for k, n := range games {
		if n != 2 {
			t.Fatalf("pair %v met %d times", k, n)
		}
	}
	st := tr.Standings(len(tr.Rounds))
	for _, s := range st {
		if s.Points != 4 || s.Games != 8 {
			t.Fatalf("all draws: %+v", s)
		}
	}
	if err := tr.ResetRoundRobin(); err == nil {
		t.Fatal("reset allowed with results")
	}
}

func TestDirectEncounterAndPrizes(t *testing.T) {
	tr := chess.NewTournament("DE")
	tr.Kind = chess.KindRoundRobin
	tr.TieBreaks = []string{chess.TBDirectEncounter, chess.TBSonneborn}
	tr.Normalize()
	for i, cat := range []string{"SenM", "PouF", "SenM", "BenM"} {
		tr.AddPlayer(&chess.Player{LastName: fmt.Sprintf("P%d", i), Rating: 2000 - i*100, Category: cat, Sex: map[bool]string{true: "f", false: "m"}[i == 1]})
	}
	tr.StartRoundRobin("rating", 0)
	// P0 and P1 both win against P2,P3; P1 beats P0 -> P1 first by DE? no: P1 has 3/3.
	res := map[[2]int]string{}
	set := func(a, b int, r string) { res[[2]int{a, b}] = r }
	set(1, 2, "0-1") // P1 beats P0 (ids 1,2)
	set(1, 3, "1-0")
	set(1, 4, "1-0")
	set(2, 3, "0-1") // P1 loses to P2
	set(2, 4, "1-0")
	set(3, 4, "0-1")
	for _, r := range tr.Rounds {
		for _, b := range r.Boards {
			if b.Black == 0 {
				continue
			}
			if v, ok := res[[2]int{b.White, b.Black}]; ok {
				b.Result = v
			} else if v, ok := res[[2]int{b.Black, b.White}]; ok {
				b.Result = map[string]string{"1-0": "0-1", "0-1": "1-0"}[v]
			}
		}
	}
	st := tr.Standings(len(tr.Rounds))
	// ids 1,2 have 2 points each; id 2 beat id 1 -> id 2 ranks first.
	if st[0].PlayerID != 2 || st[1].PlayerID != 1 || st[0].TB[chess.TBDirectEncounter] != 1 {
		t.Fatalf("direct encounter: %+v", st)
	}
	tr.Prizes = []chess.Prize{
		{ID: 1, Label: "1er", Amount: 100},
		{ID: 2, Label: "1re féminine", Amount: 50, Sex: "f"},
		{ID: 3, Label: "2e", Amount: 60},
	}
	awards := tr.AllocatePrizes(len(tr.Rounds)).Awards
	if awards[0].PlayerID != 2 || awards[2].PlayerID != 1 {
		t.Fatalf("prizes: %+v", awards)
	}
	if awards[1].PlayerID != 0 {
		t.Fatalf("women prize should be unawarded (only woman already has a prize): %+v", awards[1])
	}
}

func TestPrizeSplitHortAndShared(t *testing.T) {
	tr := chess.NewTournament("Hort")
	tr.Kind = chess.KindRoundRobin
	tr.TieBreaks = []string{chess.TBDirectEncounter, chess.TBSonneborn}
	tr.Normalize()
	for i := 0; i < 4; i++ {
		tr.AddPlayer(&chess.Player{LastName: fmt.Sprintf("P%d", i), Rating: 2000 - i*100, Category: "SenM", Fed: "FRA"})
	}
	tr.StartRoundRobin("rating", 0)
	// ids 1 and 2 on 2 points (2 ahead by direct encounter), ids 3 and 4 on 1 point.
	res := map[[2]int]string{{1, 2}: "0-1", {1, 3}: "1-0", {1, 4}: "1-0", {2, 3}: "0-1", {2, 4}: "1-0", {3, 4}: "0-1"}
	for _, r := range tr.Rounds {
		for _, b := range r.Boards {
			if v, ok := res[[2]int{b.White, b.Black}]; ok {
				b.Result = v
			} else if v, ok := res[[2]int{b.Black, b.White}]; ok {
				b.Result = map[string]string{"1-0": "0-1", "0-1": "1-0"}[v]
			}
		}
	}
	tr.Prizes = []chess.Prize{{ID: 1, Label: "1er", Amount: 100}, {ID: 2, Label: "2e", Amount: 60}, {ID: 3, Label: "3e", Amount: 25}}
	amounts := func(split string) map[int]float64 {
		tr.PrizeSplit = split
		out := map[int]float64{}
		r := tr.AllocatePrizes(len(tr.Rounds))
		total := 0.0
		for _, p := range r.Payouts {
			out[p.PlayerID] = p.Amount
			total += p.Amount
		}
		if total != 185 || r.Total != 185 {
			t.Fatalf("%s: total %v / %v, want 185", split, total, r.Total)
		}
		return out
	}
	if got := amounts(chess.PrizeSplitPlace); got[2] != 100 || got[1] != 60 || len(got) != 3 {
		t.Fatalf("place: %v", got)
	}
	// Shared: 1st+2nd split by ids 2 and 1; 3rd (25) split by ids 3 and 4 with the cent to the best ranked.
	got := amounts(chess.PrizeSplitShared)
	if got[2] != 80 || got[1] != 80 || got[3]+got[4] != 25 || len(got) != 4 {
		t.Fatalf("shared: %v", got)
	}
	// Hort: 2 gets 50 + (50+30)/2 = 90, 1 gets 30 + 40 = 70; 3e: winner 12.50 + 6.25, other 6.25.
	got = amounts(chess.PrizeSplitHort)
	if got[2] != 90 || got[1] != 70 {
		t.Fatalf("hort: %v", got)
	}
	st := tr.Standings(len(tr.Rounds))
	third, fourth := st[2].PlayerID, st[3].PlayerID
	if got[third] != 18.75 || got[fourth] != 6.25 {
		t.Fatalf("hort 3rd place: %v (third %d)", got, third)
	}
	// Conditions.
	p := tr.Player(1)
	if !(chess.Prize{Category: "Sen,Vet", MaxRating: 2000, Fed: "fra"}).Eligible(p) || (chess.Prize{MinRating: 2100}).Eligible(p) ||
		(chess.Prize{Sex: "f"}).Eligible(p) || !(chess.Prize{Sex: "m"}).Eligible(p) || (chess.Prize{Unrated: true}).Eligible(p) ||
		(chess.Prize{Fed: "SUI,MON"}).Eligible(p) || (chess.Prize{MinPoints: 3}).EligibleWith(p, 2) {
		t.Fatal("conditions")
	}
}

func TestManualPairing(t *testing.T) {
	e := engine(t)
	rng := rand.New(rand.NewSource(7))
	tr := newSwiss(6, rng)
	r1 := pairNext(t, e, tr)
	play(rng, r1)
	r2 := pairNext(t, e, tr)
	// Re-pair round 2 with the round-1 pairs: must warn about repeats.
	var mb []chess.ManualBoard
	for _, b := range r1.Boards {
		mb = append(mb, chess.ManualBoard{White: b.Black, Black: b.White})
	}
	boards, warnings, err := tr.ValidateManualPairing(2, mb)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "|"), "déjà rencontrés") {
		t.Fatalf("expected repeat warnings, got %v", warnings)
	}
	tr.ApplyManualPairing(2, boards, "test", warnings)
	if !r2.Manual {
		t.Fatal("round not marked manual")
	}
	in, err := tr.EngineInputFor(2)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Pair(context.Background(), tr.System, in.TRF)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := tr.ComparePairing(2, in, out)
	if err != nil || len(diffs) == 0 {
		t.Fatalf("engine comparison should report the manual difference: %v %v", diffs, err)
	}
	if _, _, err := tr.ValidateManualPairing(2, mb[:1]); err == nil {
		t.Fatal("incomplete manual pairing accepted")
	}
}
