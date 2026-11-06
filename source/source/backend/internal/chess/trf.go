package chess

import (
	"fmt"
	"sort"
	"strings"
)

// ExportTRF returns a TRF file of the current state: results of all paired
// rounds, ranking in the rank column, and the tournament log as comments.
// final=false marks a partial file (ITDX): unplayed results are left blank.
func (t *Tournament) ExportTRF(e *Event) string {
	trf, _ := t.trf(e, 0, nil)
	return trf
}

// buildTRF writes the TRF used to pair round next; absent players get an
// entry for that round so that the engine leaves them unpaired.
func (t *Tournament) buildTRF(next int, absent map[int]string) (string, map[int]int) {
	return t.trf(nil, next, absent)
}

func (t *Tournament) trf(e *Event, next int, absent map[int]string) (string, map[int]int) {
	seeding := t.Seeding()
	pno := map[int]int{}
	byPNo := map[int]int{}
	for i, p := range seeding {
		pno[p.ID] = i + 1
		byPNo[i+1] = p.ID
	}
	played := len(t.Rounds)
	if next > 0 {
		played = next - 1
	}
	rank := map[int]int{}
	for i, s := range t.Standings(played) {
		rank[s.PlayerID] = i + 1
	}

	var sb strings.Builder
	line := func(code, text string) {
		if strings.TrimSpace(text) != "" {
			fmt.Fprintf(&sb, "%s %s\n", code, sanitize(text))
		}
	}
	name := t.Name
	if e != nil && e.Name != "" && !strings.Contains(t.Name, e.Name) {
		name = e.Name + " - " + t.Name
	}
	line("012", name)
	if e != nil {
		line("022", e.Location)
		line("042", strings.ReplaceAll(e.StartDate, "-", "/"))
		line("052", strings.ReplaceAll(e.EndDate, "-", "/"))
	}
	line("032", t.Federation)
	fmt.Fprintf(&sb, "062 %d\n", len(seeding))
	line("102", t.ChiefArbiter)
	line("122", t.TimeControl)

	for _, p := range seeding {
		nm := sanitize(strings.TrimSuffix(strings.TrimSpace(p.LastName+", "+p.FirstName), ","))
		row := []rune(strings.Repeat(" ", 89))
		put := func(col int, s string) { copy(row[col-1:], []rune(s)) } // 1-based TRF columns
		put(1, "001")
		put(5, fmt.Sprintf("%4d", pno[p.ID]))
		put(10, fixed(p.Sex, 1))
		put(11, fmt.Sprintf("%3s", fixed(trfTitle(p.Title), 3)))
		put(15, fixed(nm, 33))
		if p.Rating > 0 {
			put(49, fmt.Sprintf("%4d", p.Rating))
		}
		put(54, fixed(strings.ToUpper(p.Fed), 3))
		if p.FideID != "" {
			put(58, fmt.Sprintf("%11s", fixed(p.FideID, 11)))
		}
		put(70, fixed(strings.ReplaceAll(p.BirthDate, "-", "/"), 10))
		put(81, fmt.Sprintf("%4.1f", t.Score(p.ID, played)))
		put(86, fmt.Sprintf("%4d", rank[p.ID]))

		var games strings.Builder
		for i := 0; i < played && i < len(t.Rounds); i++ {
			o, ok := t.OutcomeIn(t.Rounds[i], p.ID)
			switch {
			case !ok:
				games.WriteString("  0000 - Z")
			case o.Opponent == 0:
				fmt.Fprintf(&games, "  0000 - %c", o.Code)
			case o.Pending:
				fmt.Fprintf(&games, "  %4d %c  ", pno[o.Opponent], o.Color)
			default:
				fmt.Fprintf(&games, "  %4d %c %c", pno[o.Opponent], o.Color, o.Code)
			}
		}
		if next > 0 {
			if code, ok := absent[p.ID]; ok {
				fmt.Fprintf(&games, "  0000 - %s", code)
			}
		}
		sb.WriteString(strings.TrimRight(string(row)+games.String(), " "))
		sb.WriteByte('\n')
	}

	rounds := t.TotalRounds
	if t.Kind == KindRoundRobin {
		rounds = len(t.Rounds)
	}
	fmt.Fprintf(&sb, "XXR %d\n", rounds)
	fmt.Fprintf(&sb, "XXC %s1\n", t.InitialColor)
	if t.PointsWin != 1 || t.PointsDraw != 0.5 {
		fmt.Fprintf(&sb, "BBW %4.1f\nBBD %4.1f\n", t.PointsWin, t.PointsDraw)
	}
	if t.PointsPAB != t.PointsWin {
		fmt.Fprintf(&sb, "BBU %4.1f\n", t.PointsPAB)
	}
	if t.Kind == KindSwiss {
		t.writeAccelerations(&sb, seeding, pno, rounds)
		for _, g := range t.ForbiddenGroups {
			var ids []string
			for _, id := range g {
				if n, ok := pno[id]; ok {
					ids = append(ids, fmt.Sprint(n))
				}
			}
			if len(ids) >= 2 {
				fmt.Fprintf(&sb, "XXP %s\n", strings.Join(ids, " "))
			}
		}
	}
	if next == 0 {
		for _, l := range t.Log {
			fmt.Fprintf(&sb, "### %s R%d %s: %s\n", l.At.Format("2006-01-02 15:04"), l.Round, l.Kind, sanitize(l.Message))
		}
	}
	return sb.String(), byPNo
}

func (t *Tournament) writeAccelerations(sb *strings.Builder, seeding []*Player, pno map[int]int, rounds int) {
	switch t.Acceleration {
	case AccelBaku:
		for _, p := range seeding {
			fmt.Fprintf(sb, "XXA %4d", pno[p.ID])
			for r := 1; r <= rounds; r++ {
				fmt.Fprintf(sb, " %4.1f", bakuAcceleration(pno[p.ID], len(seeding), rounds, r)*t.PointsWin)
			}
			sb.WriteByte('\n')
		}
	}
	// Without XXA lines, bbpPairings applies the acceleration defined by the
	// Burstein rules themselves; the Dutch system has none.
}

// ForbiddenPairs lists player ids that must never be paired together.
func (t *Tournament) forbidden() map[[2]int]bool {
	m := map[[2]int]bool{}
	for _, g := range t.ForbiddenGroups {
		ids := append([]int{}, g...)
		sort.Ints(ids)
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				m[[2]int{ids[i], ids[j]}] = true
			}
		}
	}
	return m
}

func trfTitle(title string) string {
	switch strings.ToUpper(strings.TrimSpace(title)) {
	case "GM":
		return "g"
	case "IM":
		return "m"
	case "WGM":
		return "wg"
	case "FM":
		return "f"
	case "WIM":
		return "wm"
	case "CM":
		return "c"
	case "WFM":
		return "wf"
	case "WCM":
		return "wc"
	}
	return ""
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		return r
	}, s)
}

func fixed(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
