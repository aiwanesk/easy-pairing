package lookup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestParseFIDEProfile(t *testing.T) {
	page, err := os.ReadFile("testdata/fide_profile_carlsen.html")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseFIDEProfile(string(page))
	if err != nil {
		t.Fatal(err)
	}
	want := Player{Source: "fide", LastName: "Carlsen", FirstName: "Magnus", Title: "GM", Fed: "NOR",
		Sex: "m", BirthDate: "1990", Standard: 2823, StandardTyp: "F", Rapid: 2803, RapidTyp: "F", Blitz: 2860, BlitzTyp: "F"}
	if *p != want {
		t.Fatalf("got  %+v\nwant %+v", *p, want)
	}
}

func TestFIDEClient(t *testing.T) {
	page, _ := os.ReadFile("testdata/fide_profile_carlsen.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/1503014" {
			http.NotFound(w, r)
			return
		}
		w.Write(page)
	}))
	defer srv.Close()
	c := &FIDEClient{BaseURL: srv.URL}
	p, err := c.Get(context.Background(), "1503014")
	if err != nil || p.FideID != "1503014" || p.LastName != "Carlsen" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := c.Get(context.Background(), "999"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := c.Get(context.Background(), "abc"); err == nil {
		t.Fatal("invalid id accepted")
	}
}

func TestSearchHelpers(t *testing.T) {
	if got := fold("Lévêque-D'Arc  Hélène"); got != "LEVEQUE D ARC HELENE" {
		t.Fatalf("fold: %q", got)
	}
	k := fold("DUPONT Jean-Pierre")
	if !matchTerms(k, []string{"DUP", "PI"}) || matchTerms(k, []string{"JEAN"}) {
		t.Fatal("matchTerms")
	}
	if papiTitle("mf") != "WIM" || papiTitle(" g") != "GM" || papiTitle("") != "" {
		t.Fatal("papiTitle")
	}
	b := NewFFEBase(t.TempDir())
	b.players = []Player{{FFEID: "K59144", FideID: "650000", LastName: "IWANESKO", FirstName: "Alexandre", Licence: "A", Standard: 2200}}
	b.keys = []string{fold("IWANESKO Alexandre")}
	b.byFFE = map[string]int{"K59144": 0}
	b.byFIDE = map[string]int{"650000": 0}
	if r := b.Search("k59144", 10); len(r) != 1 {
		t.Fatal("search by licence")
	}
	if r := b.Search("iwan al", 10); len(r) != 1 {
		t.Fatal("search by name")
	}
	if r := b.Search("0650000", 10); len(r) != 1 {
		t.Fatal("search by fide id")
	}
}
