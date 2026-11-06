package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pairingeasy/internal/bbp"
	"pairingeasy/internal/lookup"
	"pairingeasy/internal/store"
)

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, base string) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: base, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, out any, headers ...string) int {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "pairing-easy")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: %v\n%s", method, path, err, data)
		}
	}
	if resp.StatusCode >= 400 && out == nil {
		c.t.Logf("%s %s -> %d %s", method, path, resp.StatusCode, data)
	}
	return resp.StatusCode
}

func (c *client) ok(method, path string, body any, out any) {
	c.t.Helper()
	var raw json.RawMessage
	code := c.do(method, path, body, &raw)
	if code >= 400 {
		c.t.Fatalf("%s %s -> %d %s", method, path, code, raw)
	}
	if out != nil {
		json.Unmarshal(raw, out)
	}
}

type viewT struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Tournaments []struct {
		ID      int `json:"id"`
		Players []struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		} `json:"players"`
		Rounds []struct {
			Number    int  `json:"number"`
			Published bool `json:"published"`
			Boards    []struct {
				Number int    `json:"number"`
				White  int    `json:"white"`
				Black  int    `json:"black"`
				Result string `json:"result"`
			} `json:"boards"`
		} `json:"rounds"`
	} `json:"tournaments"`
	Derived map[string]struct {
		CanPair       bool   `json:"canPair"`
		CanPairReason string `json:"canPairReason"`
	} `json:"derived"`
}

func TestEndToEnd(t *testing.T) {
	bbpPath := os.Getenv("BBP_PAIRINGS")
	if bbpPath == "" {
		t.Skip("BBP_PAIRINGS non défini")
	}
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := &bbp.Engine{}
	eng.Set(bbpPath, "test", "")
	srv := &Server{Store: st, Engine: eng, FFE: lookup.NewFFEBase(dir), FIDE: &lookup.FIDEClient{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), DataDir: dir}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	admin := newClient(t, ts.URL)
	var sess struct {
		SetupRequired bool `json:"setupRequired"`
	}
	admin.ok("GET", "/api/session", nil, &sess)
	if !sess.SetupRequired {
		t.Fatal("setup should be required")
	}
	if code := admin.do("POST", "/api/setup", map[string]string{"login": "admin", "name": "Admin", "password": "court"}, nil); code != 422 {
		t.Fatalf("weak password accepted: %d", code)
	}
	admin.ok("POST", "/api/setup", map[string]string{"login": "admin", "name": "Admin Principal", "password": "un-mot-de-passe-solide"}, nil)
	if code := admin.do("POST", "/api/setup", map[string]string{"login": "x", "name": "x", "password": "un-mot-de-passe-solide"}, nil); code != 409 {
		t.Fatal("second setup allowed")
	}
	// CSRF header required
	req, _ := http.NewRequest("POST", ts.URL+"/api/events", strings.NewReader(`{"name":"x"}`))
	resp, _ := admin.http.Do(req)
	if resp.StatusCode != 403 {
		t.Fatalf("missing CSRF header accepted: %d", resp.StatusCode)
	}

	// accounts
	admin.ok("POST", "/api/accounts", map[string]string{"login": "arbitre", "name": "Arbitre Un", "password": "arbitre-mot-de-passe", "role": "user"}, nil)
	admin.ok("POST", "/api/accounts", map[string]string{"login": "lecteur", "name": "Lecteur", "password": "lecteur-mot-de-passe", "role": "user"}, nil)
	var accounts []store.Account
	admin.ok("GET", "/api/accounts", nil, &accounts)
	if len(accounts) != 3 {
		t.Fatalf("accounts: %d", len(accounts))
	}
	idOf := func(login string) int {
		for _, a := range accounts {
			if a.Login == login {
				return a.ID
			}
		}
		return 0
	}

	// event + permissions
	var ev viewT
	admin.ok("POST", "/api/events", map[string]any{"name": "Open de Lancy 2026", "location": "Lancy", "public": true}, &ev)
	if ev.Slug != "open-de-lancy-2026" {
		t.Fatalf("slug %q", ev.Slug)
	}
	admin.ok("PUT", "/api/events/"+ev.ID+"/permissions", map[string]any{"accountId": idOf("arbitre"), "level": "arbiter"}, nil)
	admin.ok("PUT", "/api/events/"+ev.ID+"/permissions", map[string]any{"accountId": idOf("lecteur"), "level": "viewer"}, nil)

	arb := newClient(t, ts.URL)
	arb.ok("POST", "/api/login", map[string]string{"login": "ARBITRE", "password": "arbitre-mot-de-passe"}, nil)
	viewer := newClient(t, ts.URL)
	viewer.ok("POST", "/api/login", map[string]string{"login": "lecteur", "password": "lecteur-mot-de-passe"}, nil)
	if code := newClient(t, ts.URL).do("POST", "/api/login", map[string]string{"login": "lecteur", "password": "faux"}, nil); code != 401 {
		t.Fatal("bad password accepted")
	}

	base := "/api/events/" + ev.ID
	arb.ok("POST", base+"/tournaments", map[string]any{"name": "Open A", "kind": "swiss", "totalRounds": 5, "checkIn": true, "kioskPin": "1234", "kioskResults": true,
		"registration": map[string]any{"open": true, "maxPlayers": 12, "showList": true}}, &ev)
	tid := ev.Tournaments[0].ID
	tb := fmt.Sprintf("%s/tournaments/%d", base, tid)
	if code := viewer.do("POST", tb+"/players", map[string]any{"lastName": "X"}, nil); code != 403 {
		t.Fatalf("viewer could add a player: %d", code)
	}
	var bulk struct {
		Added int `json:"added"`
		Lines []struct {
			Error string `json:"error"`
		} `json:"lines"`
	}
	text := "DUPONT;Jean;2100;Club A\nMARTIN;Sophie;2000;Club B\nDURAND;Paul;1900\nLEROY;Anne;1850\nMOREAU;Luc;1700\nPETIT;Léa;1600\nGARNIER;Tom;abc\n"
	arb.ok("POST", tb+"/players/bulk", map[string]string{"text": text}, &bulk)
	if bulk.Added != 6 || bulk.Lines[6].Error == "" {
		t.Fatalf("bulk: %+v", bulk)
	}

	// online registration (public) + confirmation
	pub := newClient(t, ts.URL)
	var regOut struct {
		Status string `json:"status"`
	}
	pub.ok("POST", fmt.Sprintf("/api/public/e/%s/t/%d/register", ev.Slug, tid), map[string]any{"lastName": "Inscrit", "firstName": "En Ligne", "email": "a@b.ch", "rating": 1500}, &regOut)
	if regOut.Status != "pending" {
		t.Fatalf("registration status %q", regOut.Status)
	}
	arb.ok("GET", base, nil, &ev)
	var pendingID int
	for _, p := range ev.Tournaments[0].Players {
		if p.Status == "pending" {
			pendingID = p.ID
		}
	}
	arb.ok("POST", fmt.Sprintf("%s/players/%d/status", tb, pendingID), map[string]string{"status": "confirmed"}, nil)

	// check-in: everyone except player 1 via admin, player 2 via kiosk
	var tok struct {
		Token string `json:"token"`
	}
	if code := pub.do("POST", fmt.Sprintf("/api/public/e/%s/t/%d/kiosk/auth", ev.Slug, tid), map[string]string{"pin": "0000"}, &tok); code != 401 {
		t.Fatal("wrong kiosk PIN accepted")
	}
	pub.ok("POST", fmt.Sprintf("/api/public/e/%s/t/%d/kiosk/auth", ev.Slug, tid), map[string]string{"pin": "1234"}, &tok)
	kh := []string{"X-Kiosk-Token", tok.Token}
	if code := pub.do("POST", fmt.Sprintf("/api/public/e/%s/t/%d/kiosk/checkin", ev.Slug, tid), map[string]int{"playerId": 2}, nil, kh...); code != 200 {
		t.Fatalf("kiosk check-in: %d", code)
	}
	for _, p := range ev.Tournaments[0].Players {
		if p.ID != 1 && p.ID != 2 {
			arb.ok("POST", fmt.Sprintf("%s/players/%d/checkin", tb, p.ID), map[string]bool{"checkedIn": true}, nil)
		}
	}

	// rounds
	for round := 1; round <= 3; round++ {
		arb.ok("POST", tb+"/rounds", nil, &ev)
		r := ev.Tournaments[0].Rounds[round-1]
		if round == 1 {
			for _, b := range r.Boards {
				if b.White == 1 || b.Black == 1 {
					t.Fatal("unchecked player paired in round 1")
				}
			}
		}
		// public cannot see unpublished round
		var pe struct {
			Tournaments []struct {
				Rounds []any `json:"rounds"`
			} `json:"tournaments"`
		}
		pub.ok("GET", "/api/public/e/"+ev.Slug, nil, &pe)
		if len(pe.Tournaments[0].Rounds) != round-1 {
			t.Fatalf("public sees %d rounds before publication of round %d", len(pe.Tournaments[0].Rounds), round)
		}
		arb.ok("POST", fmt.Sprintf("%s/rounds/%d/publish", tb, round), map[string]bool{"published": true}, &ev)
		for i, b := range r.Boards {
			if b.Black == 0 {
				continue
			}
			res := []string{"1-0", "0-1", "1/2"}[i%3]
			if i == 0 && round == 3 {
				// kiosk entry on board 1, then a second attempt must be refused
				if code := pub.do("POST", fmt.Sprintf("/api/public/e/%s/t/%d/kiosk/result", ev.Slug, tid), map[string]any{"round": round, "board": b.Number, "result": "1-0"}, nil); code != 401 {
					t.Fatalf("kiosk result without token: %d", code)
				}
				if code := pub.do("POST", fmt.Sprintf("/api/public/e/%s/t/%d/kiosk/result", ev.Slug, tid), map[string]any{"round": round, "board": b.Number, "result": "1-0"}, nil, kh...); code != 204 {
					t.Fatalf("kiosk result: %d", code)
				}
				if code := pub.do("POST", fmt.Sprintf("/api/public/e/%s/t/%d/kiosk/result", ev.Slug, tid), map[string]any{"round": round, "board": b.Number, "result": "0-1"}, nil, kh...); code < 400 {
					t.Fatal("kiosk overwrote a result")
				}
				continue
			}
			arb.ok("PUT", fmt.Sprintf("%s/rounds/%d/boards/%d", tb, round, b.Number), map[string]string{"result": res}, nil)
		}
	}
	arb.ok("GET", base, nil, &ev)
	if ev.Tournaments[0].Rounds[2].Boards[0].Result != "1-0" {
		t.Fatal("kiosk result not recorded")
	}

	// manual pairing of round 4: swap colours on board 1 (warning or engine difference)
	arb.ok("POST", tb+"/rounds", nil, &ev)
	r4 := ev.Tournaments[0].Rounds[3]
	var boards []map[string]int
	for i, b := range r4.Boards {
		if i == 0 {
			boards = append(boards, map[string]int{"white": b.Black, "black": b.White})
		} else {
			boards = append(boards, map[string]int{"white": b.White, "black": b.Black})
		}
	}
	arb.ok("PUT", tb+"/rounds/4/pairing", map[string]any{"boards": boards}, &ev)
	var check struct {
		Differences []string `json:"differences"`
	}
	arb.ok("POST", tb+"/rounds/4/check", nil, &check)
	if len(check.Differences) == 0 {
		t.Fatal("engine check should see the swapped colours")
	}

	// standings, prizes, exports
	var stand struct {
		Standings []struct {
			Rank int `json:"rank"`
		} `json:"standings"`
	}
	viewer.ok("GET", tb+"/standings", nil, &stand)
	if len(stand.Standings) != 7 {
		t.Fatalf("standings: %d", len(stand.Standings))
	}
	arb.ok("PUT", tb+"/prizes", []map[string]any{{"label": "1er", "amount": 100}, {"label": "2e", "amount": 50}}, nil)
	if code := arb.do("PUT", tb+"/prizes", map[string]any{"split": "hort", "prizes": []map[string]any{{"label": "x", "minRating": 2000, "maxRating": 1500}}}, nil); code < 400 {
		t.Fatalf("min > max rating accepted: %d", code)
	}
	arb.ok("PUT", tb+"/prizes", map[string]any{"split": "hort", "prizes": []map[string]any{{"label": "1er", "amount": 100}, {"label": "2e", "amount": 50}}}, nil)
	var prizes struct {
		Awards []struct {
			PlayerID int `json:"playerId"`
		} `json:"awards"`
	}
	arb.ok("GET", tb+"/prizes", nil, &prizes)
	awards := prizes.Awards
	if len(awards) != 2 || awards[0].PlayerID == 0 || awards[0].PlayerID == awards[1].PlayerID {
		t.Fatalf("awards: %+v", awards)
	}
	for _, f := range []string{"trf", "players.csv", "standings.csv", "pgn"} {
		req, _ := http.NewRequest("GET", ts.URL+tb+"/export/"+f, nil)
		resp, err := arb.http.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("export %s: %v %d", f, err, resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if len(b) < 50 {
			t.Fatalf("export %s too short: %q", f, b)
		}
	}

	// audit log: admin sees everything, arbiter (not owner) is refused
	var audit struct {
		Total   int `json:"total"`
		Entries []struct {
			Action string `json:"action"`
		} `json:"entries"`
	}
	admin.ok("GET", "/api/audit?event="+ev.ID+"&limit=500", nil, &audit)
	actions := map[string]int{}
	for _, e := range audit.Entries {
		actions[e.Action]++
	}
	for _, a := range []string{"event.create", "tournament.create", "player.bulk", "registration.online", "registration.confirmed", "kiosk.checkin", "round.pair", "round.publish", "result.set", "kiosk.result", "pairing.manual"} {
		if actions[a] == 0 {
			t.Errorf("audit: no %s entry (%v)", a, actions)
		}
	}
	if code := arb.do("GET", "/api/audit?event="+ev.ID, nil, nil); code != 403 {
		t.Fatalf("arbiter read the audit log: %d", code)
	}
	// logout invalidates the session
	arb.ok("POST", "/api/logout", nil, nil)
	if code := arb.do("GET", base, nil, nil); code != 401 {
		t.Fatalf("session still valid after logout: %d", code)
	}
}
