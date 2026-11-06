package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// hub notifies live screens that an event changed (Server-Sent Events).
type hub struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func newHub() *hub { return &hub{subs: map[string]map[chan struct{}]struct{}{}} }

func (h *hub) subscribe(id string) chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	if h.subs[id] == nil {
		h.subs[id] = map[chan struct{}]struct{}{}
	}
	h.subs[id][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(id string, ch chan struct{}) {
	h.mu.Lock()
	delete(h.subs[id], ch)
	if len(h.subs[id]) == 0 {
		delete(h.subs, id)
	}
	h.mu.Unlock()
}

func (h *hub) publish(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[id] {
		select {
		case ch <- struct{}{}:
		default: // a notification is already pending
		}
	}
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request, eventID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("streaming non supporté"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := s.hub.subscribe(eventID)
	defer s.hub.unsubscribe(eventID, ch)
	fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "event: update\ndata: %d\n\n", time.Now().UnixMilli())
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) adminStream(w http.ResponseWriter, r *http.Request) {
	eid := r.PathValue("eid")
	if err := s.require(r, eid, levelViewer); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.stream(w, r, eid)
}

func (s *Server) publicStream(w http.ResponseWriter, r *http.Request) {
	e, err := s.Store.EventBySlug(r.PathValue("slug"))
	if err != nil || !e.Public {
		writeErr(w, http.StatusNotFound, fmt.Errorf("événement introuvable"))
		return
	}
	s.stream(w, r, e.ID)
}
