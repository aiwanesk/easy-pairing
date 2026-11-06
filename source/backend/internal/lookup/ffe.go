package lookup

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"pairingeasy/internal/mdb"
)

// DefaultPapiURL is where the FFE publishes the PAPI player database.
const DefaultPapiURL = "https://www.echecs.asso.fr/Papi/PapiData.zip"

// FFEBase is an in-memory index of the FFE PAPI database (Data.mdb).
type FFEBase struct {
	Dir string // where Data.mdb is stored
	URL string

	mu       sync.RWMutex
	players  []Player
	keys     []string // folded "NOM PRENOM" per player, for name search
	byFFE    map[string]int
	byFIDE   map[string]int
	state    string // empty | loading | ready | error
	err      string
	loadedAt time.Time
	fileDate time.Time
}

// FFEStatus is exposed to the UI.
type FFEStatus struct {
	State    string    `json:"state"`
	Error    string    `json:"error,omitempty"`
	Players  int       `json:"players"`
	LoadedAt time.Time `json:"loadedAt"`
	FileDate time.Time `json:"fileDate"`
	URL      string    `json:"url"`
}

// NewFFEBase prepares the index; call LoadExisting to read a saved file.
func NewFFEBase(dir string) *FFEBase {
	return &FFEBase{Dir: dir, URL: DefaultPapiURL, state: "empty"}
}

func (b *FFEBase) mdbPath() string { return filepath.Join(b.Dir, "Data.mdb") }

// Status returns the current state of the index.
func (b *FFEBase) Status() FFEStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return FFEStatus{State: b.state, Error: b.err, Players: len(b.players), LoadedAt: b.loadedAt, FileDate: b.fileDate, URL: b.URL}
}

// LoadExisting loads Data.mdb from Dir if present.
func (b *FFEBase) LoadExisting() error {
	if _, err := os.Stat(b.mdbPath()); err != nil {
		return nil
	}
	return b.load(b.mdbPath())
}

// Download fetches PapiData.zip, extracts Data.mdb and reloads the index.
func (b *FFEBase) Download(ctx context.Context, client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	b.setState("loading", "")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.URL, nil)
	if err != nil {
		return b.fail(err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Roque chess pairing)")
	resp, err := client.Do(req)
	if err != nil {
		return b.fail(fmt.Errorf("téléchargement de la base FFE: %w", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return b.fail(fmt.Errorf("téléchargement de la base FFE: HTTP %d", resp.StatusCode))
	}
	return b.Import(resp.Body, "PapiData.zip")
}

// Import stores an uploaded PapiData.zip or Data.mdb and reloads the index.
func (b *FFEBase) Import(r io.Reader, filename string) error {
	b.setState("loading", "")
	if err := os.MkdirAll(b.Dir, 0o755); err != nil {
		return b.fail(err)
	}
	tmp, err := os.CreateTemp(b.Dir, "upload-*")
	if err != nil {
		return b.fail(err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return b.fail(err)
	}
	tmp.Close()

	isZip := strings.HasSuffix(strings.ToLower(filename), ".zip")
	if !isZip {
		if f, err := os.Open(tmpName); err == nil {
			head := make([]byte, 4)
			io.ReadFull(f, head)
			f.Close()
			isZip = string(head) == "PK\x03\x04"
		}
	}
	src := tmpName
	if isZip {
		extracted, err := extractMDB(tmpName, b.Dir)
		if err != nil {
			return b.fail(err)
		}
		defer os.Remove(extracted)
		src = extracted
	}
	// Validate before replacing the current file.
	if err := b.load(src); err != nil {
		return err
	}
	if err := os.Rename(src, b.mdbPath()); err != nil {
		return b.fail(err)
	}
	return nil
}

func extractMDB(zipPath, dir string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("archive invalide: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !strings.EqualFold(filepath.Ext(f.Name), ".mdb") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.CreateTemp(dir, "data-*.mdb")
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			os.Remove(out.Name())
			return "", err
		}
		return out.Name(), nil
	}
	return "", errors.New("aucun fichier .mdb dans l'archive")
}

func (b *FFEBase) setState(state, msg string) {
	b.mu.Lock()
	b.state, b.err = state, msg
	b.mu.Unlock()
}

func (b *FFEBase) fail(err error) error {
	b.mu.Lock()
	if len(b.players) > 0 {
		b.state = "ready" // keep serving the previous index
	} else {
		b.state = "error"
	}
	b.err = err.Error()
	b.mu.Unlock()
	return err
}

func (b *FFEBase) load(path string) error {
	b.setState("loading", "")
	db, err := mdb.Open(path)
	if err != nil {
		return b.fail(fmt.Errorf("lecture de la base FFE: %w", err))
	}
	defer db.Close()

	clubs := map[int64]string{}
	if ct, err := db.Table("CLUB"); err == nil {
		db.Scan(ct, func(r map[string]any) error {
			if ref, ok := r["Ref"].(int64); ok {
				clubs[ref] = str(r["Nom"])
			}
			return nil
		})
	}
	jt, err := db.Table("JOUEUR")
	if err != nil {
		return b.fail(fmt.Errorf("table JOUEUR absente: ce n'est pas une base PAPI (%w)", err))
	}
	players := make([]Player, 0, jt.NumRows)
	err = db.Scan(jt, func(r map[string]any) error {
		p := Player{
			Source:      "ffe",
			FFEID:       strings.ToUpper(str(r["NrFFE"])),
			FideID:      strings.TrimLeft(str(r["FideCode"]), "0"),
			LastName:    str(r["Nom"]),
			FirstName:   str(r["Prenom"]),
			Sex:         strings.ToLower(str(r["Sexe"])),
			Fed:         str(r["Federation"]),
			Category:    str(r["Cat"]),
			Licence:     str(r["AffType"]),
			Title:       papiTitle(str(r["FideTitre"])),
			Standard:    num(r["Elo"]),
			StandardTyp: str(r["Fide"]),
			Rapid:       num(r["Rapide"]),
			RapidTyp:    str(r["RapideFide"]),
			Blitz:       num(r["Blitz"]),
			BlitzTyp:    str(r["BlitzFide"]),
		}
		if t, ok := r["NeLe"].(time.Time); ok && t.Year() > 1900 {
			p.BirthDate = t.Format("2006/01/02")
		}
		if ref, ok := r["ClubRef"].(int64); ok {
			p.Club = clubs[ref]
		}
		if p.FFEID != "" || p.LastName != "" {
			players = append(players, p)
		}
		return nil
	})
	if err != nil {
		return b.fail(err)
	}
	if len(players) == 0 {
		return b.fail(errors.New("la base FFE ne contient aucun joueur"))
	}

	keys := make([]string, len(players))
	byFFE := make(map[string]int, len(players))
	byFIDE := make(map[string]int, len(players)/4)
	for i, p := range players {
		keys[i] = fold(p.LastName + " " + p.FirstName)
		if p.FFEID != "" {
			byFFE[p.FFEID] = i
		}
		if p.FideID != "" {
			byFIDE[p.FideID] = i
		}
	}
	var fileDate time.Time
	if st, err := os.Stat(path); err == nil {
		fileDate = st.ModTime()
	}
	b.mu.Lock()
	b.players, b.keys, b.byFFE, b.byFIDE = players, keys, byFFE, byFIDE
	b.state, b.err, b.loadedAt, b.fileDate = "ready", "", time.Now(), fileDate
	b.mu.Unlock()
	return nil
}

var ffeIDRe = regexp.MustCompile(`^[A-Za-z]\d{5}$`)

// ByFFE returns a player by licence number (e.g. K59144).
func (b *FFEBase) ByFFE(id string) (*Player, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if i, ok := b.byFFE[strings.ToUpper(strings.TrimSpace(id))]; ok {
		p := b.players[i]
		return &p, nil
	}
	return nil, ErrNotFound
}

// ByFIDE returns a player of the FFE base by FIDE id.
func (b *FFEBase) ByFIDE(id string) (*Player, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if i, ok := b.byFIDE[strings.TrimLeft(strings.TrimSpace(id), "0")]; ok {
		p := b.players[i]
		return &p, nil
	}
	return nil, ErrNotFound
}

// Ready reports whether the index can answer queries.
func (b *FFEBase) Ready() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.players) > 0
}

// Search finds players by licence number, FIDE id, or name prefix
// ("DUPONT", "DUPONT Jean", "dup je").
func (b *FFEBase) Search(q string, limit int) []Player {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	if ffeIDRe.MatchString(q) {
		if p, err := b.ByFFE(q); err == nil {
			return []Player{*p}
		}
		return nil
	}
	if fideIDRe.MatchString(q) {
		if p, err := b.ByFIDE(q); err == nil {
			return []Player{*p}
		}
		return nil
	}
	terms := strings.Fields(fold(q))
	b.mu.RLock()
	defer b.mu.RUnlock()
	var hits []int
	for i, k := range b.keys {
		if matchTerms(k, terms) {
			hits = append(hits, i)
			if len(hits) > 2000 {
				break
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, c := b.players[hits[i]], b.players[hits[j]]
		// active licences first, then rating
		if (a.Licence != "N") != (c.Licence != "N") {
			return a.Licence != "N"
		}
		return a.Standard > c.Standard
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Player, len(hits))
	for i, h := range hits {
		out[i] = b.players[h]
	}
	return out
}

// matchTerms: first term must prefix the last name, others prefix any word.
func matchTerms(key string, terms []string) bool {
	if len(terms) == 0 || !strings.HasPrefix(key, terms[0]) {
		return false
	}
	words := strings.Fields(key)
	for _, t := range terms[1:] {
		found := false
		for _, w := range words[1:] {
			if strings.HasPrefix(w, t) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func papiTitle(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "g":
		return "GM"
	case "m":
		return "IM"
	case "f":
		return "FM"
	case "c":
		return "CM"
	case "gf":
		return "WGM"
	case "mf":
		return "WIM"
	case "ff":
		return "WFM"
	case "cf":
		return "WCM"
	}
	return ""
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func num(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// fold uppercases and strips the accents found in French names.
func fold(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case 'À', 'Â', 'Ä', 'Á', 'Ã', 'Å':
			r = 'A'
		case 'Ç':
			r = 'C'
		case 'É', 'È', 'Ê', 'Ë':
			r = 'E'
		case 'Î', 'Ï', 'Í', 'Ì':
			r = 'I'
		case 'Ô', 'Ö', 'Ó', 'Ò', 'Õ', 'Ø':
			r = 'O'
		case 'Ù', 'Û', 'Ü', 'Ú':
			r = 'U'
		case 'Ÿ', 'Ý':
			r = 'Y'
		case 'Ñ':
			r = 'N'
		case '-', '\'', '’', '.':
			r = ' '
		}
		sb.WriteRune(r)
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}
