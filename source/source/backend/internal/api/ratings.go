package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"pairingeasy/internal/chess"
	"pairingeasy/internal/lookup"
)

// ratingSources holds the official data found for the players of a
// tournament: FFE base first (French players, with their FIDE ratings),
// then ratings.fide.com for the others.
type ratingSources map[int]*lookup.Player

// collectRatingSources looks every player up. FIDE cards are fetched in
// parallel within a global time limit; players not found keep their rating.
func (s *Server) collectRatingSources(ctx context.Context, players []*chess.Player) ratingSources {
	out := ratingSources{}
	var fide []*chess.Player
	for _, p := range players {
		if s.FFE != nil && s.FFE.Ready() {
			if p.FFEID != "" {
				if l, err := s.FFE.ByFFE(p.FFEID); err == nil {
					out[p.ID] = l
					continue
				}
			}
			if p.FideID != "" {
				if l, err := s.FFE.ByFIDE(p.FideID); err == nil {
					out[p.ID] = l
					continue
				}
			}
		}
		if p.FideID != "" && s.FIDE != nil {
			fide = append(fide, p)
		}
	}
	if len(fide) == 0 {
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for _, p := range fide {
		wg.Add(1)
		go func(id int, fideID string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if l, err := s.FIDE.Get(ctx, fideID); err == nil {
				mu.Lock()
				out[id] = l
				mu.Unlock()
			}
		}(p.ID, p.FideID)
	}
	wg.Wait()
	return out
}

// applyRatings sets each player's rating to the one of the given kind
// (standard, rapid, blitz) and returns a summary.
func applyRatings(t *chess.Tournament, kind string, src ratingSources) (changed, unknown int, lines []string) {
	for _, p := range t.Players {
		l := src[p.ID]
		if l == nil {
			if p.FFEID != "" || p.FideID != "" {
				unknown++
			}
			continue
		}
		ref := fromLookup(l, kind)
		if ref.Rating != p.Rating || ref.RatingType != p.RatingType {
			lines = append(lines, fmt.Sprintf("%s %d%s → %d%s", p.FullName(), p.Rating, p.RatingType, ref.Rating, ref.RatingType))
			p.Rating, p.RatingType = ref.Rating, ref.RatingType
			changed++
		}
		if p.Title == "" && ref.Title != "" {
			p.Title = ref.Title
		}
	}
	return changed, unknown, lines
}

var ratingKindLabel = map[string]string{"standard": "lent", "rapid": "rapide", "blitz": "blitz"}

// refreshRatings reloads the ratings of the kind used by the tournament
// (button « Mettre à jour les Elo »). Refused once round 1 is paired:
// pairing numbers and ratings are then frozen.
func (s *Server) refreshRatings(w http.ResponseWriter, r *http.Request) {
	_, t, ok := s.loadTournament(w, r)
	if !ok {
		return
	}
	src := s.collectRatingSources(r.Context(), t.Players)
	var summary string
	s.mutate(w, r, levelArbiter, "tournament.ratings", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		if t.Started() {
			return 0, "", "", fail(http.StatusConflict, "la ronde 1 est appariée : les Elo sont figés pour ce tournoi")
		}
		changed, unknown, lines := applyRatings(t, t.RatingKind, src)
		summary = fmt.Sprintf("Elo %s : %d modifié(s), %d introuvable(s)", ratingKindLabel[t.RatingKind], changed, unknown)
		if len(lines) > 0 {
			summary += " — " + strings.Join(lines, " ; ")
		}
		t.AddLog("settings", 0, by, "%s", summary)
		return t.ID, t.Name, summary, nil
	})
}
