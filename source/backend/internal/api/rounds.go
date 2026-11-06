package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pairingeasy/internal/bbp"
	"pairingeasy/internal/chess"
)

func (s *Server) pairNext(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Order   string      `json:"order"`   // round robin: rating | lots | varma | manual
		Numbers map[int]int `json:"numbers"` // manual: player id -> Berger number
	}
	if r.ContentLength > 0 {
		if err := decode(w, r, &in); err != nil {
			writeAnyErr(w, err)
			return
		}
	}
	s.mutate(w, r, levelArbiter, "round.pair", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if t.IsTeam() {
			details, err := s.pairTeams(t, r, in.Order, by)
			if err != nil {
				return 0, "", "", err
			}
			return t.ID, fmt.Sprintf("%s — ronde %d", t.Name, len(t.Rounds)), details, nil
		}
		if t.Kind == chess.KindKnockout {
			if !t.Started() {
				if err := t.StartKnockout(uint64(time.Now().UnixNano())); err != nil {
					return 0, "", "", err
				}
				t.AddLog("pairing", 1, by, "coupe : tableau de %d joueurs créé (%s, %d tour(s))", len(t.Participants()),
					map[bool]string{true: "placement tiré au sort", false: "têtes de série par Elo"}[t.KO.Seeding == "lots"], t.TotalRounds)
				return t.ID, fmt.Sprintf("%s — %s", t.Name, t.KORoundName(1)), "", nil
			}
			round, err := t.KONextRound()
			if err != nil {
				return 0, "", "", err
			}
			t.AddLog("pairing", round.Number, by, "coupe : %s créé(e)s", strings.ToLower(t.KORoundName(round.Number)))
			return t.ID, fmt.Sprintf("%s — %s", t.Name, t.KORoundName(round.Number)), "", nil
		}
		if t.Kind == chess.KindRoundRobin {
			if err := t.StartRoundRobinWith(in.Order, uint64(time.Now().UnixNano()), in.Numbers); err != nil {
				return 0, "", "", err
			}
			how := map[string]string{"lots": "tirés au sort", "varma": "tirage restreint (tables de Varma, FIDE C.05 annexe 2)", "manual": "saisis par l'arbitre"}[in.Order]
			if how == "" {
				how = "par Elo"
			}
			t.AddLog("pairing", 0, by, "calendrier toutes-rondes créé (%d rondes, numéros %s)", len(t.Rounds), how)
			for _, line := range t.LastDraw() {
				t.AddLog("pairing", 0, by, "tirage Varma : %s", line)
			}
			return t.ID, t.Name, fmt.Sprintf("%d rondes", len(t.Rounds)), nil
		}
		if !s.Engine.Status().Ready {
			return 0, "", "", fail(http.StatusServiceUnavailable, "moteur bbpPairings non installé (voir Système)")
		}
		pin, err := t.PrepareNextSwissRound()
		if err != nil {
			return 0, "", "", err
		}
		out, err := s.Engine.Pair(r.Context(), t.System, pin.TRF)
		if errors.Is(err, bbp.ErrNoPairing) {
			d := t.DiagnoseSwissFailure()
			return 0, "", "", &httpError{status: http.StatusConflict, msg: d.Summary, details: d.Lines()}
		}
		if err != nil {
			return 0, "", "", err
		}
		round, err := t.ApplySwissPairing(pin, out)
		if err != nil {
			return 0, "", "", err
		}
		t.AddLog("pairing", round.Number, by, "ronde %d appariée par bbpPairings (%s%s), %d échiquiers, %d absent(s)",
			round.Number, t.System, map[bool]string{true: ", Baku", false: ""}[t.Acceleration == chess.AccelBaku], len(round.Boards), len(round.Absences))
		return t.ID, fmt.Sprintf("%s — ronde %d", t.Name, round.Number), fmt.Sprintf("%d échiquiers", len(round.Boards)), nil
	})
}

func (s *Server) deleteLastRound(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, levelArbiter, "round.delete", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		n := len(t.Rounds)
		if err := t.DeleteLastRound(); err != nil {
			return 0, "", "", err
		}
		t.AddLog("pairing", n, by, "ronde %d supprimée", n)
		return t.ID, fmt.Sprintf("%s — ronde %d", t.Name, n), "", nil
	})
}

func (s *Server) resetRoundRobin(w http.ResponseWriter, r *http.Request) {
	s.mutate(w, r, levelArbiter, "round.reset", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if t.Kind != chess.KindRoundRobin {
			return 0, "", "", errors.New("réservé aux toutes-rondes")
		}
		if err := t.ResetRoundRobin(); err != nil {
			return 0, "", "", err
		}
		t.AddLog("pairing", 0, by, "calendrier toutes-rondes supprimé")
		return t.ID, t.Name, "", nil
	})
}

func (s *Server) setResult(w http.ResponseWriter, r *http.Request) {
	n, err := pathInt(r, "n")
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
	s.mutate(w, r, levelResults, "result.set", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if onlyResults {
			// Result entry accounts fill in empty boards of the current round
			// only; corrections stay with the arbiters.
			if n != len(t.Rounds) {
				return 0, "", "", fail(http.StatusForbidden, "saisie des résultats : seule la ronde en cours peut être saisie")
			}
			for _, bd := range t.Rounds[n-1].Boards {
				if bd.Number == b && bd.Result != "" && bd.Result != in.Result {
					return 0, "", "", fail(http.StatusForbidden, "résultat déjà saisi : seul un arbitre peut le corriger")
				}
			}
			if !t.Rounds[n-1].Published {
				return 0, "", "", fail(http.StatusForbidden, "la ronde n'est pas encore publiée")
			}
		}
		return applyResult(t, n, b, in.Result, map[bool]string{true: "saisie", false: "arbiter"}[onlyResults], by)
	})
}

// applyResult enforces the correction rules then records a result.
func applyResult(t *chess.Tournament, n, b int, result, source, by string) (int, string, string, error) {
	if n < 1 || n > len(t.Rounds) {
		return 0, "", "", errors.New("ronde inexistante")
	}
	round := t.Rounds[n-1]
	var board *chess.Board
	for _, x := range round.Boards {
		if x.Number == b {
			board = x
		}
	}
	if board == nil {
		return 0, "", "", errors.New("échiquier inexistant")
	}
	old := board.Result
	if old == result {
		return t.ID, "", "", nil
	}
	nextPaired := t.Kind == chess.KindSwiss && n < len(t.Rounds)
	if t.FIDEMode && t.Kind == chess.KindSwiss && n < len(t.Rounds)-1 {
		return 0, "", "", errors.New("mode FIDE : seuls les résultats de la ronde précédente peuvent encore être corrigés")
	}
	if nextPaired && result == "" {
		return 0, "", "", errors.New("impossible d'effacer un résultat d'une ronde déjà suivie d'un appariement")
	}
	if err := t.SetResult(n, b, result, source); err != nil {
		return 0, "", "", err
	}
	target := fmt.Sprintf("%s — R%d éch. %d", t.Name, n, b)
	details := fmt.Sprintf("%s → %s (%s)", orEmpty(old), orEmpty(result), source)
	if old != "" && (nextPaired || round.Published) {
		t.AddLog("correction", n, by, "R%d éch. %d : %s → %s", n, b, orEmpty(old), orEmpty(result))
	}
	return t.ID, target, details, nil
}

func orEmpty(s string) string {
	if s == "" {
		return "(vide)"
	}
	return s
}

func (s *Server) publishRound(w http.ResponseWriter, r *http.Request) {
	n, err := pathInt(r, "n")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Published bool `json:"published"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	action := "round.publish"
	if !in.Published {
		action = "round.unpublish"
	}
	s.mutate(w, r, levelArbiter, action, func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if n < 1 || n > len(t.Rounds) {
			return 0, "", "", errors.New("ronde inexistante")
		}
		round := t.Rounds[n-1]
		if in.Published {
			for _, prev := range t.Rounds[:n-1] {
				if !prev.Complete() || !prev.Published {
					return 0, "", "", fmt.Errorf("la ronde %d doit être terminée et publiée avant", prev.Number)
				}
			}
		} else {
			for _, b := range round.Boards {
				if b.Black != 0 && b.Result != "" {
					return 0, "", "", errors.New("une ronde avec des résultats ne peut plus être dépubliée")
				}
			}
			if t.Kind == chess.KindSwiss && n < len(t.Rounds) {
				return 0, "", "", errors.New("la ronde suivante est déjà appariée")
			}
		}
		round.Published = in.Published
		t.AddLog("publish", n, by, "ronde %d %s", n, map[bool]string{true: "publiée", false: "dépubliée"}[in.Published])
		return t.ID, fmt.Sprintf("%s — ronde %d", t.Name, n), "", nil
	})
}

func (s *Server) manualPairing(w http.ResponseWriter, r *http.Request) {
	n, err := pathInt(r, "n")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Boards []chess.ManualBoard `json:"boards"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "pairing.manual", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		boards, warns, err := t.ValidateManualPairing(n, in.Boards)
		if err != nil {
			return 0, "", "", err
		}
		backup := t.Rounds[n-1].Boards
		t.ApplyManualPairing(n, boards, by, warns)
		if t.Kind == chess.KindSwiss && s.Engine.Status().Ready {
			diffs, err := s.compare(r.Context(), t, n)
			if err == nil {
				round := t.Rounds[n-1]
				if len(diffs) == 0 {
					round.Checked = "conforme au moteur"
				} else {
					if t.FIDEMode {
						round.Boards, round.Manual = backup, false
						return 0, "", "", &httpError{status: http.StatusUnprocessableEntity,
							msg: "mode FIDE : l'appariement manuel diffère de celui du moteur, modification annulée", details: diffs}
					}
					round.Checked = strings.Join(diffs, " ; ")
					t.AddLog("manual", n, by, "écarts avec le moteur : %s", round.Checked)
				}
			}
		}
		return t.ID, fmt.Sprintf("%s — ronde %d", t.Name, n), strings.Join(warns, " ; "), nil
	})
}

func (s *Server) compare(ctx context.Context, t *chess.Tournament, n int) ([]string, error) {
	pin, err := t.EngineInputFor(n)
	if err != nil {
		return nil, err
	}
	out, err := s.Engine.Pair(ctx, t.System, pin.TRF)
	if err != nil {
		return nil, err
	}
	return t.ComparePairing(n, pin, out)
}

func (s *Server) checkRound(w http.ResponseWriter, r *http.Request) {
	n, err := pathInt(r, "n")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	_, t, ok := s.loadTournament(w, r)
	if !ok {
		return
	}
	if t.Kind != chess.KindSwiss {
		writeErr(w, http.StatusUnprocessableEntity, errors.New("contrôle réservé aux suisses"))
		return
	}
	diffs, err := s.compare(r.Context(), t, n)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if diffs == nil {
		diffs = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"round": n, "differences": diffs})
}

// ---------------------------------------------------------------- knockout

func koParams(r *http.Request) (n, m int, err error) {
	if n, err = pathInt(r, "n"); err != nil {
		return
	}
	m, err = pathInt(r, "m")
	return
}

func (s *Server) koResult(w http.ResponseWriter, r *http.Request) {
	n, m, err := koParams(r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	g, err := pathInt(r, "g")
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
	s.mutate(w, r, levelResults, "ko.result", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if onlyResults && n == len(t.Rounds) {
			for _, mt := range t.Rounds[n-1].Matches {
				for _, gm := range mt.Games {
					if mt.Number == m && gm.Number == g && gm.Result != "" {
						return 0, "", "", fail(http.StatusForbidden, "résultat déjà saisi : seul un arbitre peut le corriger")
					}
				}
			}
		} else if onlyResults {
			return 0, "", "", fail(http.StatusForbidden, "saisie des résultats : seul le tour en cours peut être saisi")
		}
		if err := t.KOSetResult(n, m, g, in.Result); err != nil {
			return 0, "", "", err
		}
		return t.ID, fmt.Sprintf("%s — %s, match %d, partie %d", t.Name, t.KORoundName(n), m, g), in.Result, nil
	})
}

func (s *Server) koTieBreak(w http.ResponseWriter, r *http.Request) {
	n, m, err := koParams(r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "ko.tiebreak", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		st, err := t.KOAddTieBreak(n, m)
		if err != nil {
			return 0, "", "", err
		}
		t.AddLog("pairing", n, by, "%s, match %d : départage %s", t.KORoundName(n), m, st)
		return t.ID, fmt.Sprintf("%s — match %d", t.KORoundName(n), m), st, nil
	})
}

func (s *Server) koWinner(w http.ResponseWriter, r *http.Request) {
	n, m, err := koParams(r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Winner int    `json:"winner"`
		Note   string `json:"note"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.mutate(w, r, levelArbiter, "ko.winner", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if err := t.KOSetWinner(n, m, in.Winner, in.Note); err != nil {
			return 0, "", "", err
		}
		if in.Winner != 0 {
			t.AddLog("correction", n, by, "%s, match %d : %s qualifié par décision de l'arbitre (%s)", t.KORoundName(n), m, t.Player(in.Winner).FullName(), in.Note)
		}
		return t.ID, fmt.Sprintf("%s — match %d", t.KORoundName(n), m), in.Note, nil
	})
}
