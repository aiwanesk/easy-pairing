package chess

// Outcome describes what a player got in one round.
type Outcome struct {
	Opponent int     // 0 if none
	Color    byte    // 'w', 'b' or '-'
	Code     byte    // TRF result code: 1 0 = + - H F U Z (or 'P' pending)
	Points   float64 // points earned
	Played   bool    // over-the-board game
	Pending  bool    // paired but no result yet
	Free     bool    // round-robin free round
}

// OutcomeIn computes a player's outcome in a paired round; ok is false when
// the player does not appear in the round at all.
func (t *Tournament) OutcomeIn(r *Round, id int) (Outcome, bool) {
	for _, b := range r.Boards {
		if b.White != id && b.Black != id {
			continue
		}
		if b.Black == 0 || b.White == 0 {
			if t.Kind == KindRoundRobin || b.Result == ResultFreeRound {
				return Outcome{Color: '-', Code: 'Z', Free: true}, true
			}
			return Outcome{Color: '-', Code: 'U', Points: t.PointsPAB}, true
		}
		white := b.White == id
		o := Outcome{Opponent: b.Black, Color: 'w'}
		if !white {
			o.Opponent, o.Color = b.White, 'b'
		}
		switch b.Result {
		case "":
			o.Pending, o.Code = true, 'P'
		case ResultWhiteWins, ResultBlackWins:
			o.Played = true
			if (b.Result == ResultWhiteWins) == white {
				o.Points, o.Code = t.PointsWin, '1'
			} else {
				o.Code = '0'
			}
		case ResultDraw:
			o.Played, o.Points, o.Code = true, t.PointsDraw, '='
		case ResultWhiteForfeit, ResultBlackForfeit:
			if (b.Result == ResultWhiteForfeit) == white {
				o.Points, o.Code = t.PointsWin, '+'
			} else {
				o.Code = '-'
			}
		case ResultDoubleForfeit:
			o.Code = '-'
		}
		return o, true
	}
	if code, ok := r.Absences[id]; ok {
		o := Outcome{Color: '-', Code: code[0]}
		switch code {
		case ByeHalf:
			o.Points = t.PointsDraw
		case ByeFull:
			o.Points = t.PointsWin
		}
		return o, true
	}
	return Outcome{}, false
}

// Score returns a player's points after the first n rounds.
func (t *Tournament) Score(id, n int) float64 {
	s := 0.0
	for i := 0; i < n && i < len(t.Rounds); i++ {
		if o, ok := t.OutcomeIn(t.Rounds[i], id); ok {
			s += o.Points
		}
	}
	return s
}

// Scores returns every player's points after the first n rounds.
func (t *Tournament) Scores(n int) map[int]float64 {
	m := make(map[int]float64, len(t.Players))
	for _, p := range t.Players {
		m[p.ID] = t.Score(p.ID, n)
	}
	return m
}

// Met reports whether two players have played (or been paired, forfeits
// excluded) against each other in the first n rounds.
func (t *Tournament) Met(a, b, n int) bool {
	for i := 0; i < n && i < len(t.Rounds); i++ {
		for _, bd := range t.Rounds[i].Boards {
			if (bd.White == a && bd.Black == b) || (bd.White == b && bd.Black == a) {
				if bd.Result != ResultWhiteForfeit && bd.Result != ResultBlackForfeit && bd.Result != ResultDoubleForfeit {
					return true
				}
			}
		}
	}
	return false
}
