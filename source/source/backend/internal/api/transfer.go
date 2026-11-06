package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"pairingeasy/internal/auth"
	"pairingeasy/internal/chess"
	"pairingeasy/internal/store"
)

// eventFile is the transfer format between installations: prepare an event
// on the online platform, run it offline at the venue (local network), then
// send it back, or the opposite.
type eventFile struct {
	Format     string       `json:"format"` // "pairing_easy-event"
	Version    int          `json:"version"`
	ExportedAt time.Time    `json:"exportedAt"`
	App        string       `json:"app"`
	Event      *chess.Event `json:"event"`
}

const eventFileFormat = "pairing_easy-event"

func (s *Server) exportEvent(w http.ResponseWriter, r *http.Request) {
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
	s.audit_(r, eid, 0, "event.export", e.Name, "")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.pairing.json"`, e.Slug, time.Now().Format("20060102-1504")))
	writeJSON(w, http.StatusOK, eventFile{Format: eventFileFormat, Version: 1, ExportedAt: time.Now().UTC(), App: Version, Event: e})
}

// importEvent creates an event from an export file. With ?replace=1 and an
// event of the same id the caller owns, the event is replaced (back from
// an offline session); otherwise a copy with a new id is created.
func (s *Server) importEvent(w http.ResponseWriter, r *http.Request) {
	a := accountFrom(r)
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, errors.New("fichier trop volumineux"))
		return
	}
	var f eventFile
	if err := json.Unmarshal(data, &f); err != nil || f.Format != eventFileFormat || f.Event == nil {
		writeErr(w, http.StatusUnprocessableEntity, errors.New("ce fichier n'est pas un export d'événement pairing_easy"))
		return
	}
	in := f.Event
	in.Normalize()
	for _, t := range in.Tournaments {
		t.Normalize()
	}
	if r.URL.Query().Get("replace") == "1" {
		if _, err := s.Store.Event(in.ID); err == nil {
			if err := s.require(r, in.ID, levelOwner); err != nil {
				writeAnyErr(w, err)
				return
			}
			e, err := s.Store.UpdateEvent(r.Context(), in.ID, func(e *chess.Event) error {
				id, slug, created := e.ID, e.Slug, e.CreatedAt
				*e = *in
				e.ID, e.Slug, e.CreatedAt = id, slug, created
				e.UpdatedAt = time.Now().UTC()
				return nil
			})
			if err != nil {
				writeAnyErr(w, err)
				return
			}
			s.audit_(r, e.ID, 0, "event.import", e.Name, fmt.Sprintf("remplacé par le fichier exporté le %s", f.ExportedAt.Format("02/01/2006 15:04")))
			s.hub.publish(e.ID)
			writeJSON(w, http.StatusOK, s.view(r, e))
			return
		} else if !errors.Is(err, store.ErrNotFound) {
			writeAnyErr(w, err)
			return
		}
	}
	if a.Role != "admin" && a.Role != "manager" {
		writeErr(w, http.StatusForbidden, errors.New("seuls les administrateurs et les gestionnaires peuvent créer un événement"))
		return
	}
	if _, err := s.Store.Event(in.ID); err == nil || in.ID == "" {
		in.ID = auth.RandomID(6)
	}
	base := in.Slug
	if base == "" {
		base = Slugify(in.Name)
	}
	in.CreatedAt, in.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	for i := 1; ; i++ {
		in.Slug = base
		if i > 1 {
			in.Slug = fmt.Sprintf("%s-%d", base, i)
		}
		err := s.Store.CreateEvent(in, a.ID)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "déjà utilisée") || i > 50 {
			writeAnyErr(w, err)
			return
		}
	}
	s.audit_(r, in.ID, 0, "event.import", in.Name, fmt.Sprintf("nouvel événement depuis un fichier exporté le %s", f.ExportedAt.Format("02/01/2006 15:04")))
	e, err := s.Store.Event(in.ID)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.view(r, e))
}
