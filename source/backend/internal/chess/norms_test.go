package chess

import (
	"strings"
	"testing"
)

func TestNormHelpers(t *testing.T) {
	// 1.4.6: only the lowest opponent is raised to the floor; unrated count 1400.
	// The unrated opponent is the lowest: raised to 2050; 1900 stays. 8850/4 = 2212.5 → 2213.
	if got := normAverage([]int{2500, 2400, 1900, 0}, 2050); got != 2213 {
		t.Fatalf("average %d", got)
	}
	// Two unrated: one raised to the floor, the other counts 1400.
	if got := normAverage([]int{2400, 0, 0}, 2050); got != (2400+2050+1400+1)/3 {
		t.Fatalf("average with two unrated %d", got)
	}
	// 9 games against Ra 2380: 6.5 (72 % → +166 = 2546) is not enough, 7 (78 % → +220 = 2600) is.
	if s := neededScore(2380, 9, 2599.5); s != 7 {
		t.Fatalf("GM need at 2380/9 = %v, want 7 (78 %% → +220)", s)
	}
	if s := neededScore(2230, 9, 2449.5); s != 7 {
		t.Fatalf("IM need at 2230/9 = %v, want 7", s)
	}
	if s := neededScore(2450, 9, 2449.5); s != 4.5 {
		t.Fatalf("IM need at 2450/9 = %v, want 4.5", s)
	}
}

func TestRoundRobinIMNorm(t *testing.T) {
	tr := NewTournament("Fermé MI")
	tr.Kind = KindRoundRobin
	tr.Normalize()
	field := []struct {
		name, title, fed string
		rating           int
	}{
		{"Seeker", "FM", "FRA", 2300}, {"G1", "GM", "UKR", 2520}, {"G2", "GM", "GER", 2500}, {"G3", "GM", "ESP", 2480},
		{"I1", "IM", "FRA", 2420}, {"I2", "IM", "SUI", 2400}, {"F1", "FM", "FRA", 2350}, {"F2", "FM", "ITA", 2330},
		{"P1", "", "FRA", 2250}, {"P2", "", "FRA", 2200},
	}
	for _, f := range field {
		tr.AddPlayer(&Player{LastName: f.name, Title: f.title, Fed: f.fed, Rating: f.rating, RatingType: "F"})
	}
	if err := tr.StartRoundRobin("rating", 1); err != nil {
		t.Fatal(err)
	}
	seeker := 1
	rep := tr.Norms(0)
	var im *NormStatus
	for i := range rep.Players {
		if rep.Players[i].PlayerID == seeker && rep.Players[i].Norm == "IM" {
			im = &rep.Players[i]
		}
	}
	if im == nil {
		t.Fatalf("no IM status for the seeker: %+v", rep)
	}
	// Ra = (2520+2500+2480+2420+2400+2350+2330+2250+2200)/9 = 2383.3 → 2383; 6/9 (67 % → +125) = 2508, 5.5 (61 % → +80) = 2463, 5 (56 % → +43) = 2426.
	if im.Ra != 2383 || !im.Possible || !strings.Contains(im.Need, "5.5/9") {
		t.Fatalf("IM status: %+v", *im)
	}
	for _, c := range im.Conditions {
		if c.Label != "Performance" && !c.OK {
			t.Fatalf("condition %s should hold: %s", c.Label, c.Detail)
		}
	}
	// Play everything: seeker draws the three GMs and wins the rest (7.5/9), the others draw.
	for _, r := range tr.Rounds {
		for _, b := range r.Boards {
			switch {
			case b.Black == 0 || b.White == 0:
			case b.White == seeker && b.Black <= 4 || b.Black == seeker && b.White <= 4:
				b.Result = ResultDraw
			case b.White == seeker:
				b.Result = ResultWhiteWins
			case b.Black == seeker:
				b.Result = ResultBlackWins
			default:
				b.Result = ResultDraw
			}
		}
	}
	rep = tr.Norms(len(tr.Rounds))
	got := false
	for _, s := range rep.Players {
		if s.PlayerID == seeker && s.Norm == "IM" {
			got = s.Achieved
			if s.Score != 7.5 {
				t.Fatalf("score %v", s.Score)
			}
		}
	}
	if !got {
		t.Fatalf("IM norm not achieved: %+v", rep.Players)
	}
}
