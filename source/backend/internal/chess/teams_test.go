package chess

import (
	"fmt"
	"testing"
)

func teamTournament(nTeams, boards int) *Tournament {
	tr := NewTournament("Équipes")
	tr.TeamCfg = TeamSettings{Boards: boards, Points: TeamPoints{Win: 3, Draw: 2, Loss: 1}}
	tr.Normalize()
	for i := 0; i < nTeams; i++ {
		tm := &Team{ID: i + 1, Name: fmt.Sprintf("Club %c", 'A'+i)}
		for b := 0; b < boards+1; b++ {
			p := &Player{LastName: fmt.Sprintf("T%dB%d", i+1, b+1), Rating: 2200 - 100*i - 10*b}
			tr.AddPlayer(p)
			tm.Players = append(tm.Players, p.ID)
		}
		tr.Teams = append(tr.Teams, tm)
	}
	tr.Normalize()
	return tr
}

func TestTeamRoundRobin(t *testing.T) {
	tr := teamTournament(4, 4)
	tr.Kind = KindRoundRobin
	if err := tr.StartTeamRoundRobin("rating", 1); err != nil {
		t.Fatal(err)
	}
	if len(tr.Rounds) != 3 {
		t.Fatalf("rounds %d", len(tr.Rounds))
	}
	m := tr.Rounds[0].TeamMatches[0]
	// Home team has White on boards 1 and 3.
	home := tr.Team(m.Home)
	if m.Games[0].White != home.Players[0] || m.Games[1].Black != home.Players[1] {
		t.Fatalf("colours: %+v", m.Games)
	}
	// Absent player: the reserve plays board 4.
	tr.Player(home.Players[1]).Byes[2] = ByeZero
	m2 := tr.Rounds[1].TeamMatches[0]
	_ = m2
	if l := tr.lineup(home, 2); l[1] != home.Players[2] || l[3] != home.Players[4] {
		t.Fatalf("lineup %v", l)
	}
	// 2.5-1.5 for home.
	for i, r := range []string{ResultWhiteWins, ResultDraw, ResultDraw, ResultBlackWins} {
		if err := tr.SetTeamResult(1, m.Number, i+1, r); err != nil {
			t.Fatal(err)
		}
	}
	h, a, ok := tr.MatchScore(m)
	// Boards: 1 home White wins (1), 2 home Black draw (.5), 3 home White draw (.5), 4 home Black, Black wins (1) -> 3-1.
	if !ok || h != 3 || a != 1 {
		t.Fatalf("score %v-%v", h, a)
	}
	hm, am, _ := tr.MatchPoints(m)
	if hm != 3 || am != 1 {
		t.Fatalf("match points %v-%v", hm, am)
	}
	st := tr.TeamStandings(1)
	if st[0].TeamID != m.Home || st[0].MP != 3 {
		t.Fatalf("standings %+v", st)
	}
	if err := tr.SetLineup(1, m.Number, "home", []int{0, 0, 0, 0}); err == nil {
		t.Fatal("lineup change after results must be refused")
	}
}

func TestTeamSwissInput(t *testing.T) {
	tr := teamTournament(5, 2)
	tr.TotalRounds = 3
	st, pin, err := tr.TeamSwissInput()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Players) != 5 || pin.Round != 1 || len(pin.TRF) == 0 {
		t.Fatalf("input %d %d", len(st.Players), pin.Round)
	}
	// Fake engine output: 1-2, 3-4, 5 bye (pairing numbers by rating = team ids here).
	r, err := tr.ApplyTeamPairing(st, pin, "3 1 2 3 4 5 0")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.TeamMatches) != 3 || r.TeamMatches[2].Away != 0 {
		t.Fatalf("matches %+v", r.TeamMatches)
	}
	if err := tr.CanPairTeams(); err == nil {
		t.Fatal("round 1 not finished")
	}
}
