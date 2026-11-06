package chess

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PairingInput is the TRF handed to the engine plus the id mapping needed to
// read its answer back.
type PairingInput struct {
	Round  int
	TRF    string
	ByPNo  map[int]int // pairing number -> player id
	Absent map[int]string
}

// PrepareNextSwissRound validates that the next round can be paired and
// builds the TRF(bx) for bbpPairings.
func (t *Tournament) PrepareNextSwissRound() (*PairingInput, error) {
	if t.Kind != KindSwiss {
		return nil, errors.New("ce tournoi n'est pas un suisse")
	}
	next := len(t.Rounds) + 1
	if next > t.TotalRounds {
		return nil, fmt.Errorf("toutes les rondes (%d) ont déjà été appariées", t.TotalRounds)
	}
	if cur := t.CurrentRound(); cur != nil {
		if !cur.Complete() {
			return nil, fmt.Errorf("la ronde %d n'est pas terminée : saisissez tous les résultats", cur.Number)
		}
		if !cur.Published {
			return nil, fmt.Errorf("publiez la ronde %d avant d'apparier la suivante", cur.Number)
		}
	}
	absent := map[int]string{}
	active := 0
	for _, p := range t.Participants() {
		if code := t.AbsenceFor(p, next); code != "" {
			absent[p.ID] = code
		} else {
			active++
		}
	}
	if active < 2 {
		return nil, errors.New("il faut au moins deux joueurs présents pour apparier")
	}
	trf, byPNo := t.buildTRF(next, absent)
	return &PairingInput{Round: next, TRF: trf, ByPNo: byPNo, Absent: absent}, nil
}

// EngineInputFor rebuilds the engine input of an already paired round
// (rounds before it, absences frozen at pairing time), to compare a
// hand-made pairing with what bbpPairings would have produced.
func (t *Tournament) EngineInputFor(number int) (*PairingInput, error) {
	if t.Kind != KindSwiss || number < 1 || number > len(t.Rounds) {
		return nil, errors.New("ronde suisse inexistante")
	}
	absent := map[int]string{}
	for id, code := range t.Rounds[number-1].Absences {
		absent[id] = code
	}
	trf, byPNo := t.buildTRF(number, absent)
	return &PairingInput{Round: number, TRF: trf, ByPNo: byPNo, Absent: absent}, nil
}

// ComparePairing lists the differences between the boards of a round and
// the engine output for the same round.
func (t *Tournament) ComparePairing(number int, in *PairingInput, engineOutput string) ([]string, error) {
	want, err := parsePairs(engineOutput, in.ByPNo)
	if err != nil {
		return nil, err
	}
	key := func(b *Board) string { return fmt.Sprintf("%d-%d", b.White, b.Black) }
	have := map[string]bool{}
	for _, b := range t.Rounds[number-1].Boards {
		have[key(b)] = true
	}
	var diffs []string
	for _, b := range want {
		if !have[key(b)] {
			if b.Black == 0 {
				diffs = append(diffs, fmt.Sprintf("le moteur donne l'exempt à %s", t.name(b.White)))
			} else {
				diffs = append(diffs, fmt.Sprintf("le moteur apparie %s (Blancs) contre %s (Noirs)", t.name(b.White), t.name(b.Black)))
			}
		}
	}
	return diffs, nil
}

// ApplySwissPairing parses the engine output (number of pairs, then
// "white black" lines, 0 meaning bye) and appends the new round.
func (t *Tournament) ApplySwissPairing(in *PairingInput, output string) (*Round, error) {
	boards, err := parsePairs(output, in.ByPNo)
	if err != nil {
		return nil, err
	}
	for _, b := range boards {
		if b.Black == 0 {
			b.Result = ResultPairingBye
		}
	}
	t.orderBoards(boards, in.Round-1)
	r := &Round{Number: in.Round, Boards: boards, Absences: in.Absent, PairedAt: time.Now().UTC()}
	t.Rounds = append(t.Rounds, r)
	return r, nil
}

func parsePairs(output string, byPNo map[int]int) ([]*Board, error) {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return nil, errors.New("sortie du moteur vide")
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || len(fields) != 1+2*n {
		return nil, fmt.Errorf("sortie du moteur illisible: %q", output)
	}
	seen := map[int]bool{}
	var boards []*Board
	for i := 0; i < n; i++ {
		w, err1 := strconv.Atoi(fields[1+2*i])
		b, err2 := strconv.Atoi(fields[2+2*i])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("paire illisible dans la sortie du moteur: %q", output)
		}
		wid, ok := byPNo[w]
		if !ok {
			return nil, fmt.Errorf("numéro d'appariement inconnu %d", w)
		}
		bid := 0
		if b != 0 {
			if bid, ok = byPNo[b]; !ok {
				return nil, fmt.Errorf("numéro d'appariement inconnu %d", b)
			}
		}
		if seen[wid] || (bid != 0 && seen[bid]) {
			return nil, errors.New("joueur apparié deux fois par le moteur")
		}
		seen[wid], seen[bid] = true, true
		boards = append(boards, &Board{White: wid, Black: bid})
	}
	return boards, nil
}

// orderBoards sorts boards: higher score of the pair first, then higher sum
// of scores, then best pairing number; the bye goes last.
func (t *Tournament) orderBoards(boards []*Board, played int) {
	pno := t.PairingNumbers()
	scores := t.Scores(played)
	type key struct {
		max, sum float64
		best     int
	}
	k := func(b *Board) key {
		if b.Black == 0 {
			return key{-1, -1, pno[b.White]}
		}
		sw, sb := scores[b.White], scores[b.Black]
		best := pno[b.White]
		if pno[b.Black] < best {
			best = pno[b.Black]
		}
		return key{math.Max(sw, sb), sw + sb, best}
	}
	sort.SliceStable(boards, func(i, j int) bool {
		a, b := k(boards[i]), k(boards[j])
		if a.max != b.max {
			return a.max > b.max
		}
		if a.sum != b.sum {
			return a.sum > b.sum
		}
		return a.best < b.best
	})
	for i, b := range boards {
		b.Number = i + 1
	}
}

// DeleteLastRound removes the last paired round (Swiss only).
func (t *Tournament) DeleteLastRound() error {
	if t.Kind != KindSwiss {
		return errors.New("un toutes-rondes se réinitialise en entier")
	}
	cur := t.CurrentRound()
	if cur == nil {
		return errors.New("aucune ronde à supprimer")
	}
	if t.FIDEMode && cur.Published {
		for _, b := range cur.Boards {
			if b.Black != 0 && b.Result != "" {
				return errors.New("mode FIDE : une ronde publiée avec des résultats ne peut pas être supprimée")
			}
		}
	}
	t.Rounds = t.Rounds[:len(t.Rounds)-1]
	return nil
}

// bakuAcceleration returns the Baku virtual points of a player before round
// r (1-based), per FIDE C.04.5 (Baku): group A is the first half of the players rounded up
// to an even number, i.e. 2*ceil(N/4)
// of the initial ranking; during the first half of the rounds (rounded up)
// they receive one virtual point in the first half of that period (rounded
// up) and half a point in the remaining accelerated rounds.
func bakuAcceleration(pairingNo, players, totalRounds, round int) float64 {
	groupA := 2 * int(math.Ceil(float64(players)/4))
	if pairingNo > groupA {
		return 0
	}
	accelerated := int(math.Ceil(float64(totalRounds) / 2))
	full := int(math.Ceil(float64(accelerated) / 2))
	switch {
	case round <= full:
		return 1
	case round <= accelerated:
		return 0.5
	}
	return 0
}
