package chess

import (
	"errors"
	"fmt"
	"sort"
)

// ManualBoard is one board of a hand-edited pairing.
type ManualBoard struct {
	White int `json:"white"`
	Black int `json:"black"` // 0 = pairing-allocated bye / free round
}

// ValidateManualPairing checks a hand-made pairing for round number (which
// must be the last paired round, without results) and returns the boards
// in final order plus non-blocking warnings (repeated games, forbidden
// pairs, colour rules, bye eligibility).
func (t *Tournament) ValidateManualPairing(number int, in []ManualBoard) ([]*Board, []string, error) {
	if number != len(t.Rounds) || number == 0 {
		return nil, nil, errors.New("seule la dernière ronde appariée peut être modifiée à la main")
	}
	r := t.Rounds[number-1]
	for _, b := range r.Boards {
		if b.Black != 0 && b.White != 0 && b.Result != "" {
			return nil, nil, errors.New("des résultats sont déjà saisis pour cette ronde")
		}
	}
	expected := map[int]bool{}
	for _, b := range r.Boards {
		for _, id := range []int{b.White, b.Black} {
			if id != 0 {
				expected[id] = true
			}
		}
	}
	seen := map[int]bool{}
	byes := 0
	var boards []*Board
	var warnings []string
	forbidden := t.forbidden()
	for _, mb := range in {
		for _, id := range []int{mb.White, mb.Black} {
			if id == 0 {
				continue
			}
			if !expected[id] {
				return nil, nil, fmt.Errorf("le joueur #%d n'est pas à apparier dans cette ronde", id)
			}
			if seen[id] {
				return nil, nil, fmt.Errorf("%s apparaît deux fois", t.name(id))
			}
			seen[id] = true
		}
		if mb.White == 0 {
			if mb.Black == 0 {
				continue
			}
			mb.White, mb.Black = mb.Black, 0
		}
		b := &Board{White: mb.White, Black: mb.Black}
		if b.Black == 0 {
			byes++
			if t.Kind == KindRoundRobin {
				b.Result = ResultFreeRound
			} else {
				b.Result = ResultPairingBye
				if t.hadPointWithoutPlaying(b.White, number-1) {
					warnings = append(warnings, fmt.Sprintf("%s a déjà reçu un point sans jouer (exempt ou forfait)", t.name(b.White)))
				}
			}
		} else {
			if t.Met(b.White, b.Black, number-1) {
				warnings = append(warnings, fmt.Sprintf("%s et %s se sont déjà rencontrés", t.name(b.White), t.name(b.Black)))
			}
			k := [2]int{b.White, b.Black}
			if k[0] > k[1] {
				k[0], k[1] = k[1], k[0]
			}
			if forbidden[k] {
				warnings = append(warnings, fmt.Sprintf("%s et %s ne doivent pas être appariés (appariement interdit)", t.name(b.White), t.name(b.Black)))
			}
			warnings = append(warnings, t.colourWarnings(b.White, 'w', number-1)...)
			warnings = append(warnings, t.colourWarnings(b.Black, 'b', number-1)...)
		}
		boards = append(boards, b)
	}
	for id := range expected {
		if !seen[id] {
			return nil, nil, fmt.Errorf("%s n'est pas apparié", t.name(id))
		}
	}
	if byes > 1 {
		return nil, nil, errors.New("un seul exempt par ronde")
	}
	if t.Kind == KindSwiss {
		t.orderBoards(boards, number-1)
	} else {
		sort.SliceStable(boards, func(i, j int) bool { return boards[j].Black == 0 && boards[i].Black != 0 })
		for i, b := range boards {
			b.Number = i + 1
		}
	}
	return boards, warnings, nil
}

// ApplyManualPairing replaces the boards of the last round.
func (t *Tournament) ApplyManualPairing(number int, boards []*Board, by string, warnings []string) {
	r := t.Rounds[number-1]
	r.Boards = boards
	r.Manual = true
	r.Checked = ""
	msg := "appariement modifié à la main"
	if len(warnings) > 0 {
		msg += fmt.Sprintf(" (%d avertissement(s) : %v)", len(warnings), warnings)
	}
	t.AddLog("manual", number, by, "%s", msg)
}

func (t *Tournament) name(id int) string {
	if p := t.Player(id); p != nil {
		return p.FullName()
	}
	return fmt.Sprintf("#%d", id)
}

func (t *Tournament) hadPointWithoutPlaying(id, n int) bool {
	for i := 0; i < n && i < len(t.Rounds); i++ {
		if o, ok := t.OutcomeIn(t.Rounds[i], id); ok && !o.Played && o.Points == t.PointsWin {
			return true
		}
	}
	return false
}

// colourWarnings applies the absolute colour criteria of the FIDE Swiss
// rules: no colour difference beyond ±2, no three times the same colour in
// a row.
func (t *Tournament) colourWarnings(id int, c byte, n int) []string {
	if t.Kind != KindSwiss {
		return nil
	}
	var seq []byte
	for i := 0; i < n && i < len(t.Rounds); i++ {
		if o, ok := t.OutcomeIn(t.Rounds[i], id); ok && o.Played {
			seq = append(seq, o.Color)
		}
	}
	seq = append(seq, c)
	diff := 0
	for _, x := range seq {
		if x == 'w' {
			diff++
		} else {
			diff--
		}
	}
	var w []string
	if diff > 2 || diff < -2 {
		w = append(w, fmt.Sprintf("%s : différence de couleurs de %+d", t.name(id), diff))
	}
	if k := len(seq); k >= 3 && seq[k-1] == seq[k-2] && seq[k-2] == seq[k-3] {
		w = append(w, fmt.Sprintf("%s : trois fois de suite la même couleur", t.name(id)))
	}
	return w
}
