package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pairingeasy/internal/chess"
	"pairingeasy/internal/lookup"
)

var (
	reFFE  = regexp.MustCompile(`^[A-Za-z]\d{5}$`)
	reFIDE = regexp.MustCompile(`^\d{3,10}$`)
)

// fromLookup converts a lookup result into a player, using the rating of
// the tournament's rate (standard, rapid or blitz).
func fromLookup(l *lookup.Player, kind string) *chess.Player {
	p := &chess.Player{LastName: l.LastName, FirstName: l.FirstName, Title: l.Title, Fed: l.Fed, Sex: l.Sex,
		BirthDate: l.BirthDate, Category: l.Category, Club: l.Club, FideID: l.FideID, FFEID: l.FFEID, Licence: l.Licence}
	switch kind {
	case "rapid":
		p.Rating, p.RatingType = l.Rapid, l.RapidTyp
	case "blitz":
		p.Rating, p.RatingType = l.Blitz, l.BlitzTyp
	default:
		p.Rating, p.RatingType = l.Standard, l.StandardTyp
	}
	return p
}

// BulkLine is the outcome of one input line of a bulk add.
type BulkLine struct {
	Line   int    `json:"line"`
	Input  string `json:"input"`
	Player string `json:"player,omitempty"`
	Error  string `json:"error,omitempty"`
}

// resolveLine turns one line into a player: FFE licence, FIDE id, or a
// "Nom;Prénom;Elo;Club;Fédération;FIDE;FFE" CSV record.
func (s *Server) resolveLine(ctx context.Context, line, kind string) (*chess.Player, error) {
	line = strings.TrimSpace(line)
	switch {
	case reFFE.MatchString(line):
		if !s.FFE.Ready() {
			return nil, errors.New("base FFE non chargée")
		}
		l, err := s.FFE.ByFFE(line)
		if err != nil {
			return nil, fmt.Errorf("licence %s inconnue", strings.ToUpper(line))
		}
		return fromLookup(l, kind), nil
	case reFIDE.MatchString(line):
		if s.FFE.Ready() {
			if l, err := s.FFE.ByFIDE(line); err == nil {
				return fromLookup(l, kind), nil
			}
		}
		c, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		l, err := s.FIDE.Get(c, line)
		if err != nil {
			return nil, fmt.Errorf("code FIDE %s : %v", line, err)
		}
		return fromLookup(l, kind), nil
	}
	sep := ';'
	if !strings.Contains(line, ";") {
		if strings.Contains(line, "\t") {
			sep = '\t'
		} else {
			sep = ','
		}
	}
	rd := csv.NewReader(strings.NewReader(line))
	rd.Comma = sep
	rd.FieldsPerRecord = -1
	rec, err := rd.Read()
	if err != nil {
		return nil, fmt.Errorf("ligne illisible : %v", err)
	}
	get := func(i int) string {
		if i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	p := &chess.Player{LastName: get(0), FirstName: get(1), Club: get(3), Fed: get(4), FideID: get(5), FFEID: get(6)}
	if p.LastName == "" {
		return nil, errors.New("nom manquant")
	}
	if v := get(2); v != "" {
		elo, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("Elo invalide %q", v)
		}
		p.Rating, p.RatingType = elo, "E"
	}
	if p.Fed == "" {
		p.Fed = "FRA"
	}
	return p, nil
}

func (s *Server) bulkAddPlayers(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	eid := r.PathValue("eid")
	if err := s.require(r, eid, levelArbiter); err != nil {
		writeAnyErr(w, err)
		return
	}
	e, err := s.Store.Event(eid)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	t, err := tournamentOf(e, r)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	lines := strings.Split(strings.ReplaceAll(in.Text, "\r\n", "\n"), "\n")
	if len(lines) > 500 {
		writeErr(w, http.StatusUnprocessableEntity, errors.New("500 lignes au maximum par ajout"))
		return
	}
	// Resolve outside the event lock (FIDE lookups may be slow).
	report := []BulkLine{}
	var resolved []*chess.Player
	var resolvedIdx []int
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		p, err := s.resolveLine(r.Context(), l, t.RatingKind)
		bl := BulkLine{Line: i + 1, Input: strings.TrimSpace(l)}
		if err != nil {
			bl.Error = err.Error()
		} else {
			resolved = append(resolved, p)
			resolvedIdx = append(resolvedIdx, len(report))
		}
		report = append(report, bl)
	}
	added := 0
	s.mutate(&discard{}, r, levelArbiter, "player.bulk", func(e *chess.Event, by string) (int, string, string, error) {
		t, err := tournamentOf(e, r)
		if err != nil {
			return 0, "", "", err
		}
		for k, p := range resolved {
			p.Source, p.Status = "bulk", chess.StatusConfirmed
			if err := t.AddPlayer(p); err != nil {
				report[resolvedIdx[k]].Error = err.Error()
				continue
			}
			report[resolvedIdx[k]].Player = p.FullName()
			added++
		}
		return t.ID, t.Name, fmt.Sprintf("%d ajouté(s) sur %d ligne(s)", added, len(report)), nil
	})
	ev, _ := s.Store.Event(eid)
	writeJSON(w, http.StatusOK, map[string]any{"added": added, "lines": report, "event": s.view(r, ev)})
}

// discard is a ResponseWriter used when a handler reuses mutate but writes
// its own response.
type discard struct{ h http.Header }

func (d *discard) Header() http.Header {
	if d.h == nil {
		d.h = http.Header{}
	}
	return d.h
}
func (d *discard) Write(b []byte) (int, error) { return len(b), nil }
func (d *discard) WriteHeader(int)             {}

// ---------------------------------------------------------------- exports

func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	e, t, ok := s.loadTournament(w, r)
	if !ok {
		return
	}
	base := Slugify(e.Name + "-" + t.Name)
	switch r.PathValue("format") {
	case "trf":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.trf"`, base))
		w.Write([]byte(t.ExportTRF(e)))
	case "players.csv":
		var buf bytes.Buffer
		buf.WriteString("\ufeff")
		cw := csv.NewWriter(&buf)
		cw.Comma = ';'
		cw.Write([]string{"N°", "Statut", "Titre", "Nom", "Prénom", "Elo", "Type", "Féd.", "Cat.", "Sexe", "Club", "FIDE", "FFE", "Licence", "Email", "Téléphone", "Pointé", "Droits", "Payé", "Inscription"})
		pno := t.PairingNumbers()
		for _, p := range t.Players {
			cw.Write([]string{itoa(pno[p.ID]), p.Status, p.Title, p.LastName, p.FirstName, itoa(p.Rating), p.RatingType, p.Fed, p.Category, p.Sex,
				p.Club, p.FideID, p.FFEID, p.Licence, p.Email, p.Phone, map[bool]string{true: "oui", false: "non"}[p.CheckedIn],
				ftoa(p.Fee), ftoa(p.Paid), p.Source})
		}
		cw.Flush()
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-joueurs.csv"`, base))
		w.Write(buf.Bytes())
	case "standings.csv":
		var buf bytes.Buffer
		buf.WriteString("\ufeff")
		cw := csv.NewWriter(&buf)
		cw.Comma = ';'
		head := []string{"Pl.", "N°", "Titre", "Nom", "Elo", "Féd.", "Club", "Pts"}
		for _, tb := range t.TieBreaks {
			head = append(head, tb)
		}
		cw.Write(head)
		for _, st := range t.Standings(len(t.Rounds)) {
			p := t.Player(st.PlayerID)
			row := []string{itoa(st.Rank), itoa(st.PairingNo), p.Title, p.FullName(), itoa(p.Rating), p.Fed, p.Club, ftoa(st.Points)}
			for _, tb := range t.TieBreaks {
				row = append(row, ftoa(st.TB[tb]))
			}
			cw.Write(row)
		}
		cw.Flush()
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-classement.csv"`, base))
		w.Write(buf.Bytes())
	case "pgn":
		w.Header().Set("Content-Type", "application/x-chess-pgn; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pgn"`, base))
		w.Write([]byte(exportPGN(e, t)))
	default:
		writeErr(w, http.StatusNotFound, errors.New("format inconnu"))
	}
}

// exportPGN writes every paired game with its result and headers (no moves).
func exportPGN(e *chess.Event, t *chess.Tournament) string {
	var b strings.Builder
	tag := func(k, v string) { fmt.Fprintf(&b, "[%s \"%s\"]\n", k, strings.ReplaceAll(v, `"`, `'`)) }
	for _, r := range t.Rounds {
		for _, bd := range r.Boards {
			if bd.Black == 0 || bd.White == 0 {
				continue
			}
			wp, bp := t.Player(bd.White), t.Player(bd.Black)
			res := map[string]string{"1-0": "1-0", "0-1": "0-1", "1/2": "1/2-1/2", "+-": "1-0", "-+": "0-1", "--": "*", "": "*"}[bd.Result]
			tag("Event", e.Name+" - "+t.Name)
			tag("Site", e.Location)
			tag("Date", strings.ReplaceAll(e.StartDate, "-", "."))
			tag("Round", fmt.Sprintf("%d.%d", r.Number, bd.Number))
			tag("White", strings.TrimSpace(wp.LastName+", "+wp.FirstName))
			tag("Black", strings.TrimSpace(bp.LastName+", "+bp.FirstName))
			tag("Result", res)
			if wp.Rating > 0 {
				tag("WhiteElo", itoa(wp.Rating))
			}
			if bp.Rating > 0 {
				tag("BlackElo", itoa(bp.Rating))
			}
			if wp.FideID != "" {
				tag("WhiteFideId", wp.FideID)
			}
			if bp.FideID != "" {
				tag("BlackFideId", bp.FideID)
			}
			if t.TimeControl != "" {
				tag("TimeControl", t.TimeControl)
			}
			if bd.Result == "+-" || bd.Result == "-+" || bd.Result == "--" {
				tag("Termination", "forfeit")
			}
			fmt.Fprintf(&b, "\n%s\n\n", res)
		}
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func ftoa(f float64) string {
	return strings.Replace(strconv.FormatFloat(f, 'f', -1, 64), ".", ",", 1)
}
