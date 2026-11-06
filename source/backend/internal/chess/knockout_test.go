package chess

import (
	"fmt"
	"testing"
)

func TestKnockout(t *testing.T) {
	if got := fmt.Sprint(bracketOrder(8)); got != "[1 8 4 5 2 7 3 6]" {
		t.Fatalf("bracket order %s", got)
	}
	tr := NewTournament("Coupe")
	tr.Kind = KindKnockout
	tr.KO = KOSettings{Games: 2, TieBreaks: []string{"rapid:2", "armageddon"}}
	tr.Normalize()
	for i := 0; i < 6; i++ {
		tr.AddPlayer(&Player{LastName: fmt.Sprintf("P%d", i+1), Rating: 2000 - 10*i})
	}
	if err := tr.StartKnockout(1); err != nil {
		t.Fatal(err)
	}
	if tr.TotalRounds != 3 || tr.PlannedRounds() != 3 {
		t.Fatalf("rounds %d", tr.TotalRounds)
	}
	r1 := tr.Rounds[0]
	byes := 0
	for _, m := range r1.Matches {
		if m.B == 0 {
			byes++
			if m.Winner != m.A || (m.A != 1 && m.A != 2) {
				t.Fatalf("bye for %d", m.A)
			}
		}
	}
	if byes != 2 || len(r1.Matches) != 4 {
		t.Fatalf("byes %d matches %d", byes, len(r1.Matches))
	}
	if err := tr.KOCanAdvance(); err == nil {
		t.Fatal("round 1 not finished")
	}
	for _, m := range r1.Matches {
		if m.B == 0 {
			continue
		}
		// Game 1 drawn, game 2 drawn: tie -> rapid -> A wins the rapid.
		tr.KOSetResult(1, m.Number, 1, ResultDraw)
		tr.KOSetResult(1, m.Number, 2, ResultDraw)
		if m.Winner != 0 {
			t.Fatal("tie should not decide")
		}
		if st, err := tr.KOAddTieBreak(1, m.Number); err != nil || st != "rapide" || len(m.Games) != 4 {
			t.Fatalf("tie-break %q %v %d", st, err, len(m.Games))
		}
		for _, g := range m.Games[2:] {
			if g.White == m.A {
				tr.KOSetResult(1, m.Number, g.Number, ResultWhiteWins)
			} else {
				tr.KOSetResult(1, m.Number, g.Number, ResultDraw)
			}
		}
		if m.Winner != m.A || m.Decided != "tiebreak" {
			t.Fatalf("winner %d (%s)", m.Winner, m.Decided)
		}
	}
	r2, err := tr.KONextRound()
	if err != nil || len(r2.Matches) != 2 {
		t.Fatalf("round 2: %v", err)
	}
	// Armageddon: draw -> Black qualifies.
	m := r2.Matches[0]
	tr.KOSetResult(2, m.Number, 1, ResultDraw)
	tr.KOSetResult(2, m.Number, 2, ResultDraw)
	tr.KOAddTieBreak(2, m.Number)
	for _, g := range m.Games[2:4] {
		tr.KOSetResult(2, m.Number, g.Number, ResultDraw)
	}
	if st, _ := tr.KOAddTieBreak(2, m.Number); st != "Armageddon" {
		t.Fatalf("expected Armageddon, got %q", st)
	}
	arm := m.Games[len(m.Games)-1]
	tr.KOSetResult(2, m.Number, arm.Number, ResultDraw)
	if m.Winner != arm.Black {
		t.Fatalf("Armageddon draw: winner %d, black %d", m.Winner, arm.Black)
	}
	if err := tr.KOSetWinner(2, 2, r2.Matches[1].B, "tirage au sort"); err != nil {
		t.Fatal(err)
	}
	final, err := tr.KONextRound()
	if err != nil || len(final.Matches) != 1 || tr.KORoundName(3) != "Finale" {
		t.Fatalf("final: %v", err)
	}
	tr.KOSetResult(3, 1, 1, ResultWhiteWins)
	tr.KOSetResult(3, 1, 2, ResultDraw)
	st := tr.KOStandings()
	if st[0].Label != "vainqueur" || st[1].Label != "finaliste" || st[2].Rank != 3 || st[3].Rank != 3 || len(st) != 6 {
		t.Fatalf("standings %+v", st)
	}
}
