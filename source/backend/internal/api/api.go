// Package api exposes the JSON API (admin and public) and serves the
// embedded React front-end.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"pairingeasy/internal/auth"
	"pairingeasy/internal/bbp"
	"pairingeasy/internal/chess"
	"pairingeasy/internal/lookup"
	"pairingeasy/internal/store"
)

// Version is set by the build (-ldflags).
var Version = "dev"

// Server wires every dependency of the HTTP layer.
type Server struct {
	Store   *store.Store
	Engine  *bbp.Engine
	FFE     *lookup.FFEBase
	FIDE    *lookup.FIDEClient
	Log     *slog.Logger
	Static  fs.FS // built front-end (index.html at the root)
	DataDir string
	LogDir  string
	// Reinstall reinstalls the engine (download of the latest release).
	Reinstall func(ctx context.Context) error
	// Net switches local-network access (nil in tests).
	Net Network
	// TrustProxy: behind a reverse proxy (online platform).
	TrustProxy bool
	// Shutdown stops the program (a newer version asks the running one to
	// quit so it can replace it). nil = not allowed.
	Shutdown func()

	hub      *hub
	limiter  *limiter
	kioskKey []byte
	once     sync.Once
}

type ctxKey int

const accountKey ctxKey = 1

const sessionCookie = "pe_session"
const sessionTTL = 12 * time.Hour

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	s.once.Do(func() {
		trustProxy = s.TrustProxy
		s.hub = newHub()
		s.limiter = newLimiter()
		key := s.Store.Setting("kiosk_key")
		if key == "" {
			key = auth.RandomID(32)
			s.Store.SetSetting("kiosk_key", key)
		}
		s.kioskKey = []byte(key)
	})
	mux := http.NewServeMux()

	// session & setup
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("POST /api/setup", s.setup)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.logout)
	mux.HandleFunc("POST /api/me/password", s.authed(s.changeOwnPassword))

	// administration
	mux.HandleFunc("GET /api/accounts", s.admin(s.listAccounts))
	mux.HandleFunc("POST /api/accounts", s.admin(s.createAccount))
	mux.HandleFunc("PUT /api/accounts/{id}", s.admin(s.updateAccount))
	mux.HandleFunc("POST /api/accounts/{id}/password", s.admin(s.resetPassword))
	mux.HandleFunc("DELETE /api/accounts/{id}", s.admin(s.deleteAccount))
	mux.HandleFunc("GET /api/directory", s.authed(s.directory))
	mux.HandleFunc("GET /api/audit", s.authed(s.audit))
	mux.HandleFunc("GET /api/system", s.authed(s.system))
	mux.HandleFunc("POST /api/system/engine", s.admin(s.reinstallEngine))
	mux.HandleFunc("GET /api/network", s.authed(s.networkInfo))
	mux.HandleFunc("POST /api/local/shutdown", s.localShutdown)
	mux.HandleFunc("PUT /api/network", s.admin(s.setNetwork))
	mux.HandleFunc("GET /api/ffe/status", s.authed(s.ffeStatus))
	mux.HandleFunc("POST /api/ffe/download", s.authed(s.ffeDownload))
	mux.HandleFunc("POST /api/ffe/upload", s.authed(s.ffeUpload))
	mux.HandleFunc("GET /api/lookup/ffe", s.authed(s.lookupFFE))
	mux.HandleFunc("GET /api/lookup/fide/{fideId}", s.authed(s.lookupFIDE))
	mux.HandleFunc("GET /api/tiebreaks", s.tieBreaks)

	// events
	mux.HandleFunc("GET /api/events", s.authed(s.listEvents))
	mux.HandleFunc("POST /api/events", s.authed(s.createEvent))
	mux.HandleFunc("POST /api/events/import", s.authed(s.importEvent))
	mux.HandleFunc("GET /api/events/{eid}/export", s.authed(s.exportEvent))
	mux.HandleFunc("GET /api/events/{eid}", s.authed(s.getEvent))
	mux.HandleFunc("PUT /api/events/{eid}", s.authed(s.updateEvent))
	mux.HandleFunc("DELETE /api/events/{eid}", s.authed(s.deleteEvent))
	mux.HandleFunc("GET /api/events/{eid}/permissions", s.authed(s.listPermissions))
	mux.HandleFunc("PUT /api/events/{eid}/permissions", s.authed(s.setPermission))
	mux.HandleFunc("GET /api/events/{eid}/stream", s.authed(s.adminStream))

	// tournaments
	mux.HandleFunc("POST /api/events/{eid}/tournaments", s.authed(s.createTournament))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}", s.authed(s.updateTournament))
	mux.HandleFunc("DELETE /api/events/{eid}/tournaments/{tid}", s.authed(s.deleteTournament))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/players", s.authed(s.addPlayer))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/players/bulk", s.authed(s.bulkAddPlayers))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/players/{pid}", s.authed(s.updatePlayer))
	mux.HandleFunc("DELETE /api/events/{eid}/tournaments/{tid}/players/{pid}", s.authed(s.deletePlayer))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/players/{pid}/checkin", s.authed(s.checkIn))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/players/{pid}/bye", s.authed(s.setBye))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/players/{pid}/status", s.authed(s.setPlayerStatus))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/checkin", s.authed(s.checkInAll))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/forbidden", s.authed(s.setForbidden))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/prizes", s.authed(s.setPrizes))
	mux.HandleFunc("GET /api/events/{eid}/tournaments/{tid}/prizes", s.authed(s.getPrizes))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/ratings/refresh", s.authed(s.refreshRatings))
	mux.HandleFunc("GET /api/events/{eid}/tournaments/{tid}/standings", s.authed(s.standings))
	mux.HandleFunc("GET /api/events/{eid}/tournaments/{tid}/norms", s.authed(s.norms))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/rounds", s.authed(s.pairNext))
	mux.HandleFunc("DELETE /api/events/{eid}/tournaments/{tid}/rounds/last", s.authed(s.deleteLastRound))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/rounds/reset", s.authed(s.resetRoundRobin))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/rounds/{n}/boards/{b}", s.authed(s.setResult))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/rounds/{n}/pairing", s.authed(s.manualPairing))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/rounds/{n}/matches/{m}/games/{g}", s.authed(s.koResult))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/teams", s.authed(s.setTeams))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/rounds/{n}/teammatches/{m}/boards/{b}", s.authed(s.teamResult))
	mux.HandleFunc("PUT /api/events/{eid}/tournaments/{tid}/rounds/{n}/teammatches/{m}/lineup", s.authed(s.teamLineup))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/rounds/{n}/teammatches/{m}/forfeit", s.authed(s.teamForfeit))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/rounds/{n}/matches/{m}/tiebreak", s.authed(s.koTieBreak))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/rounds/{n}/matches/{m}/winner", s.authed(s.koWinner))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/rounds/{n}/publish", s.authed(s.publishRound))
	mux.HandleFunc("POST /api/events/{eid}/tournaments/{tid}/rounds/{n}/check", s.authed(s.checkRound))
	mux.HandleFunc("GET /api/events/{eid}/tournaments/{tid}/export/{format}", s.authed(s.export))

	// public
	mux.HandleFunc("GET /api/public/events", s.publicEvents)
	mux.HandleFunc("GET /api/public/e/{slug}", s.publicEvent)
	mux.HandleFunc("GET /api/public/e/{slug}/stream", s.publicStream)
	mux.HandleFunc("GET /api/public/e/{slug}/t/{tid}/standings", s.publicStandings)
	mux.HandleFunc("POST /api/public/e/{slug}/t/{tid}/register", s.publicRegister)
	mux.HandleFunc("GET /api/public/lookup/ffe", s.publicLookupFFE)
	mux.HandleFunc("POST /api/public/e/{slug}/t/{tid}/kiosk/auth", s.kioskAuth)
	mux.HandleFunc("GET /api/public/e/{slug}/t/{tid}/kiosk", s.kioskData)
	mux.HandleFunc("POST /api/public/e/{slug}/t/{tid}/kiosk/checkin", s.kioskCheckIn)
	mux.HandleFunc("POST /api/public/e/{slug}/t/{tid}/kiosk/result", s.kioskResult)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, errors.New("route inconnue"))
	})
	if s.Static != nil {
		mux.Handle("/", spa(s.Static))
	}
	return s.recoverer(s.logRequests(s.securityHeaders(s.csrf(mux))))
}

// ---------------------------------------------------------------- helpers

type apiError struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, apiError{Error: err.Error()})
}

// httpError carries an HTTP status through mutation callbacks.
type httpError struct {
	status  int
	msg     string
	details []string
}

func (e *httpError) Error() string { return e.msg }

func fail(status int, format string, args ...any) error {
	return &httpError{status: status, msg: fmt.Sprintf(format, args...)}
}

func writeAnyErr(w http.ResponseWriter, err error) {
	var he *httpError
	switch {
	case errors.As(err, &he):
		writeJSON(w, he.status, apiError{Error: he.msg, Details: he.details})
	case errors.Is(err, store.ErrNotFound), errors.Is(err, lookup.ErrNotFound):
		writeErr(w, http.StatusNotFound, err)
	case errors.Is(err, store.ErrConflict):
		writeErr(w, http.StatusConflict, err)
	default:
		writeErr(w, http.StatusUnprocessableEntity, err)
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return fail(http.StatusBadRequest, "JSON invalide : %v", err)
	}
	return nil
}

func pathInt(r *http.Request, name string) (int, error) {
	n, err := strconv.Atoi(r.PathValue(name))
	if err != nil {
		return 0, fail(http.StatusBadRequest, "paramètre %s invalide", name)
	}
	return n, nil
}

// trustProxy is set when the server runs behind a reverse proxy (online
// platform): client address and scheme come from X-Forwarded-* headers.
var trustProxy bool

func clientIP(r *http.Request) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		if xr := r.Header.Get("X-Real-IP"); xr != "" {
			return strings.TrimSpace(xr)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isHTTPS reports whether the client reached the server over HTTPS.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || trustProxy && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func accountFrom(r *http.Request) *store.Account {
	a, _ := r.Context().Value(accountKey).(*store.Account)
	return a
}

// ---------------------------------------------------------------- middleware

func (s *Server) currentAccount(r *http.Request) *store.Account {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	a, err := s.Store.SessionAccount(auth.HashToken(c.Value))
	if err != nil {
		return nil
	}
	return a
}

func (s *Server) authed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := s.currentAccount(r)
		if a == nil {
			writeErr(w, http.StatusUnauthorized, errors.New("connexion requise"))
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), accountKey, a)))
	}
}

func (s *Server) admin(h http.HandlerFunc) http.HandlerFunc {
	return s.authed(func(w http.ResponseWriter, r *http.Request) {
		if accountFrom(r).Role != "admin" {
			writeErr(w, http.StatusForbidden, errors.New("réservé aux administrateurs"))
			return
		}
		h(w, r)
	})
}

// csrf requires a custom header on state-changing API requests: browsers
// cannot send it cross-site without a CORS preflight, which we never allow.
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-Requested-With") != "pairing-easy" {
				writeErr(w, http.StatusForbidden, errors.New("requête refusée (en-tête X-Requested-With manquant)"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		if isHTTPS(r) {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || strings.HasSuffix(r.URL.Path, "/stream") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		level := slog.LevelDebug
		if r.Method != http.MethodGet {
			level = slog.LevelInfo
		}
		if rec.status >= 500 {
			level = slog.LevelError
		} else if rec.status >= 400 && r.Method != http.MethodGet {
			level = slog.LevelWarn
		}
		s.Log.Log(r.Context(), level, "http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(), "ip", clientIP(r))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.Log.Error("panic", "err", fmt.Sprint(v), "path", r.URL.Path)
				writeErr(w, http.StatusInternalServerError, errors.New("erreur interne"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// spa serves the built front-end, falling back to index.html for client
// routes.
func spa(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if st, err := fs.Stat(static, p); err != nil || st.IsDir() {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, r2)
			return
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------- audit

func (s *Server) audit_(r *http.Request, eventID string, tid int, action, target, details string) {
	e := store.AuditEntry{IP: clientIP(r), EventID: eventID, TournamentID: tid, Action: action, Target: target, Details: details}
	if a := accountFrom(r); a != nil {
		e.AccountID, e.AccountName = a.ID, a.Name
	}
	if err := s.Store.Audit(e); err != nil {
		s.Log.Error("audit", "err", err)
	}
	s.Log.Info("audit", "action", action, "event", eventID, "tournament", tid, "target", target, "by", e.AccountName)
}

// ---------------------------------------------------------------- limiter

// limiter counts failures per key and blocks a key after too many.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newLimiter() *limiter { return &limiter{hits: map[string][]time.Time{}} }

// allow reports whether key had fewer than max hits in window, and records
// a hit when record is true.
func (l *limiter) allow(key string, max int, window time.Duration, record bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := time.Now().Add(-window)
	h := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cut) {
			h = append(h, t)
		}
	}
	if len(h) >= max {
		l.hits[key] = h
		return false
	}
	if record {
		h = append(h, time.Now())
	}
	l.hits[key] = h
	return true
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	delete(l.hits, key)
	l.mu.Unlock()
}

// ---------------------------------------------------------------- misc

func (s *Server) tieBreaks(w http.ResponseWriter, r *http.Request) {
	type tb struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	out := []tb{}
	for _, id := range chess.TieBreakIDs() {
		out = append(out, tb{id, chess.TieBreakLabel(id)})
	}
	writeJSON(w, http.StatusOK, out)
}

// localShutdown lets a newer pairing_easy started on the same computer stop
// this one (only from the loopback, never through a proxy).
func (s *Server) localShutdown(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if s.Shutdown == nil || ip == nil || !ip.IsLoopback() || r.Header.Get("X-Forwarded-For") != "" || trustProxy {
		writeErr(w, http.StatusForbidden, errors.New("refusé"))
		return
	}
	s.Log.Info("arrêt demandé par une autre version", "version", r.Header.Get("X-Pairing-Easy-Version"))
	writeJSON(w, http.StatusOK, map[string]string{"version": Version})
	go func() {
		time.Sleep(200 * time.Millisecond)
		s.Shutdown()
	}()
}
