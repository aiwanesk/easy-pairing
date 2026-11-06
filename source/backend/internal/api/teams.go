package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pairingeasy/internal/bbp"
	"pairingeasy/internal/chess"
)

// setTeams replaces the team list (names, board order, withdrawals).
func (s *Server) setTeams(w http.ResponseWriter, r *http.Request) {
	var in []*chess.Team
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "tournament.teams", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if !t.IsTeam() {
			return 0, "", "", errors.New("ce tournoi n'est pas un tournoi par équipes (Réglages)")
		}
		names := map[string]bool{}
		seen := map[int]string{}
		for i, tm := range in {
			tm.Name = strings.TrimSpace(tm.Name)
			if tm.Name == "" {
				return 0, "", "", fmt.Errorf("équipe n°%d : nom obligatoire", i+1)
			}
			if names[strings.ToLower(tm.Name)] {
				return 0, "", "", fmt.Errorf("deux équipes s'appellent %s", tm.Name)
			}
			names[strings.ToLower(tm.Name)] = true
			for _, pid := range tm.Players {
				if t.Player(pid) == nil {
					return 0, "", "", fmt.Errorf("%s : joueur inconnu", tm.Name)
				}
				if other, ok := seen[pid]; ok {
					return 0, "", "", fmt.Errorf("%s est dans deux équipes (%s et %s)", t.Player(pid).FullName(), other, tm.Name)
				}
				seen[pid] = tm.Name
			}
			if tm.ID == 0 {
				tm.ID = t.NextTeamID
				t.NextTeamID++
			}
			if tm.Players == nil {
				tm.Players = []int{}
			}
		}
		// Teams that already played cannot be removed.
		keep := map[int]bool{}
		for _, tm := range in {
			keep[tm.ID] = true
		}
		for _, rd := range t.Rounds {
			for _, m := range rd.TeamMatches {
				for _, id := range []int{m.Home, m.Away} {
					if id != 0 && !keep[id] {
						return 0, "", "", fmt.Errorf("l'équipe %s a déjà joué : retirez-la plutôt que de la supprimer", t.Team(id).Name)
					}
				}
			}
		}
		t.Teams = in
		t.Normalize()
		return t.ID, t.Name, fmt.Sprintf("%d équipe(s)", len(in)), nil
	})
}

func (s *Server) pairTeams(t *chess.Tournament, r *http.Request, order, by string) (string, error) {
	if t.Kind == chess.KindRoundRobin {
		if err := t.StartTeamRoundRobin(order, uint64(time.Now().UnixNano())); err != nil {
			return "", err
		}
		t.AddLog("pairing", 0, by, "toutes-rondes par équipes : %d rondes créées", len(t.Rounds))
		return fmt.Sprintf("%d rondes", len(t.Rounds)), nil
	}
	if t.Kind != chess.KindSwiss {
		return "", errors.New("la coupe par équipes n'est pas encore disponible")
	}
	if !s.Engine.Status().Ready {
		return "", fail(http.StatusServiceUnavailable, "moteur bbpPairings non installé (voir Système)")
	}
	st, pin, err := t.TeamSwissInput()
	if err != nil {
		return "", err
	}
	out, err := s.Engine.Pair(r.Context(), t.System, pin.TRF)
	if errors.Is(err, bbp.ErrNoPairing) {
		return "", &httpError{status: http.StatusConflict, msg: fmt.Sprintf("Impossible d'apparier la ronde %d : les équipes restantes se sont déjà toutes rencontrées.", len(t.Rounds)+1),
			details: []string{"Avec des équipes, deux équipes ne peuvent pas se rencontrer deux fois.", "→ Réduisez le nombre de rondes (Réglages) ou ajoutez une équipe."}}
	}
	if err != nil {
		return "", err
	}
	round, err := t.ApplyTeamPairing(st, pin, out)
	if err != nil {
		return "", err
	}
	t.AddLog("pairing", round.Number, by, "ronde %d par équipes appariée par bbpPairings (%d matchs)", round.Number, len(round.TeamMatches))
	return fmt.Sprintf("%d matchs", len(round.TeamMatches)), nil
}

func teamParams(r *http.Request) (n, m int, err error) {
	if n, err = pathInt(r, "n"); err != nil {
		return
	}
	m, err = pathInt(r, "m")
	return
}

func (s *Server) teamResult(w http.ResponseWriter, r *http.Request) {
	n, m, err := teamParams(r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	b, err := pathInt(r, "b")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Result string `json:"result"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	onlyResults := s.level(accountFrom(r), r.PathValue("eid")) == levelResults
	s.mutate(w, r, levelResults, "team.result", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if onlyResults {
			if n != len(t.Rounds) && !(t.Kind == chess.KindRoundRobin) {
				return 0, "", "", fail(http.StatusForbidden, "saisie des résultats : seule la ronde en cours peut être saisie")
			}
			for _, tm := range t.Rounds[n-1].TeamMatches {
				for _, g := range tm.Games {
					if tm.Number == m && g.Board == b && g.Result != "" {
						return 0, "", "", fail(http.StatusForbidden, "résultat déjà saisi : seul un arbitre peut le corriger")
					}
				}
			}
		}
		if err := t.SetTeamResult(n, m, b, in.Result); err != nil {
			return 0, "", "", err
		}
		return t.ID, fmt.Sprintf("%s — R%d match %d éch. %d", t.Name, n, m, b), in.Result, nil
	})
}

func (s *Server) teamLineup(w http.ResponseWriter, r *http.Request) {
	n, m, err := teamParams(r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Side    string `json:"side"`
		Players []int  `json:"players"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "team.lineup", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if err := t.SetLineup(n, m, in.Side, in.Players); err != nil {
			return 0, "", "", err
		}
		return t.ID, fmt.Sprintf("%s — R%d match %d", t.Name, n, m), fmt.Sprintf("composition %s %v", in.Side, in.Players), nil
	})
}

func (s *Server) teamForfeit(w http.ResponseWriter, r *http.Request) {
	n, m, err := teamParams(r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Who string `json:"who"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "team.forfeit", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if err := t.SetTeamForfeit(n, m, in.Who); err != nil {
			return 0, "", "", err
		}
		return t.ID, fmt.Sprintf("%s — R%d match %d", t.Name, n, m), "forfait " + in.Who, nil
	})
}
