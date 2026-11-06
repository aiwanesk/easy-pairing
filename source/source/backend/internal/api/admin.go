package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pairingeasy/internal/auth"
	"pairingeasy/internal/lookup"
	"pairingeasy/internal/store"
)

// ---------------------------------------------------------------- session

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.CountAccounts()
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account":       s.currentAccount(r),
		"setupRequired": n == 0,
		"version":       Version,
	})
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, a *store.Account) error {
	token, hash, err := auth.NewToken()
	if err != nil {
		return err
	}
	if err := s.Store.CreateSession(hash, a.ID, sessionTTL, clientIP(r), r.UserAgent()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: isHTTPS(r), MaxAge: int(sessionTTL.Seconds()),
	})
	return nil
}

type credentials struct {
	Login    string `json:"login"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	if n, _ := s.Store.CountAccounts(); n > 0 {
		writeErr(w, http.StatusConflict, errors.New("l'installation est déjà faite"))
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	a := &store.Account{Login: in.Login, Name: in.Name, Email: in.Email, Role: "admin"}
	if err := s.Store.CreateAccount(a, hash); err != nil {
		writeAnyErr(w, err)
		return
	}
	if err := s.startSession(w, r, a); err != nil {
		writeAnyErr(w, err)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), accountKey, a))
	s.audit_(r, "", 0, "setup", a.Login, "création du compte administrateur")
	writeJSON(w, http.StatusOK, map[string]any{"account": a})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	key := clientIP(r) + "|" + strings.ToLower(strings.TrimSpace(in.Login))
	if !s.limiter.allow("login:"+key, 5, 10*time.Minute, false) || !s.limiter.allow("loginip:"+clientIP(r), 30, 10*time.Minute, false) {
		writeErr(w, http.StatusTooManyRequests, errors.New("trop de tentatives, réessayez dans quelques minutes"))
		return
	}
	a, err := s.Store.AccountByLogin(in.Login)
	ok := false
	if err == nil {
		ok = a.Active && auth.CheckPassword(a.PasswordHash(), in.Password)
	} else {
		auth.CheckDummy(in.Password)
	}
	if !ok {
		s.limiter.allow("login:"+key, 5, 10*time.Minute, true)
		s.limiter.allow("loginip:"+clientIP(r), 30, 10*time.Minute, true)
		s.audit_(r, "", 0, "login.failed", in.Login, "")
		writeErr(w, http.StatusUnauthorized, auth.ErrInvalidCredentials)
		return
	}
	s.limiter.reset("login:" + key)
	if err := s.startSession(w, r, a); err != nil {
		writeAnyErr(w, err)
		return
	}
	r = r.WithContext(context.WithValue(r.Context(), accountKey, a))
	s.audit_(r, "", 0, "login", a.Login, "")
	writeJSON(w, http.StatusOK, map[string]any{"account": a})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if a := s.currentAccount(r); a != nil {
			r = r.WithContext(context.WithValue(r.Context(), accountKey, a))
			s.audit_(r, "", 0, "logout", a.Login, "")
		}
		s.Store.DeleteSession(auth.HashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: isHTTPS(r)})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) changeOwnPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	a := accountFrom(r)
	if !auth.CheckPassword(a.PasswordHash(), in.Current) {
		writeErr(w, http.StatusUnprocessableEntity, errors.New("mot de passe actuel incorrect"))
		return
	}
	hash, err := auth.HashPassword(in.New)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if err := s.Store.SetPassword(a.ID, hash); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.startSession(w, r, a)
	s.audit_(r, "", 0, "account.password", a.Login, "changement de son mot de passe")
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- accounts

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.Accounts()
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if list == nil {
		list = []*store.Account{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) directory(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.Accounts()
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	type entry struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Login string `json:"login"`
	}
	out := []entry{}
	for _, a := range list {
		if a.Active {
			out = append(out, entry{a.ID, a.Name, a.Login})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		credentials
		Role string `json:"role"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	a := &store.Account{Login: in.Login, Name: in.Name, Email: in.Email, Role: in.Role}
	if err := s.Store.CreateAccount(a, hash); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, "", 0, "account.create", a.Login, fmt.Sprintf("rôle %s", a.Role))
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in store.Account
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	cur, err := s.Store.Account(id)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if cur.Role == "admin" && cur.Active && (in.Role != "admin" || !in.Active) {
		if n, _ := s.Store.CountActiveAdmins(); n <= 1 {
			writeErr(w, http.StatusUnprocessableEntity, errors.New("il doit rester au moins un administrateur actif"))
			return
		}
	}
	in.ID = id
	if err := s.Store.UpdateAccount(&in); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, "", 0, "account.update", cur.Login, fmt.Sprintf("nom=%q rôle=%s actif=%v", in.Name, in.Role, in.Active))
	a, _ := s.Store.Account(id)
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	a, err := s.Store.Account(id)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if err := s.Store.SetPassword(id, hash); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, "", 0, "account.password", a.Login, "réinitialisation par un administrateur")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt(r, "id")
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if id == accountFrom(r).ID {
		writeErr(w, http.StatusUnprocessableEntity, errors.New("vous ne pouvez pas supprimer votre propre compte"))
		return
	}
	a, err := s.Store.Account(id)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	if a.Role == "admin" && a.Active {
		if n, _ := s.Store.CountActiveAdmins(); n <= 1 {
			writeErr(w, http.StatusUnprocessableEntity, errors.New("il doit rester au moins un administrateur actif"))
			return
		}
	}
	if err := s.Store.DeleteAccount(id); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, "", 0, "account.delete", a.Login, "")
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- audit log

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	a := accountFrom(r)
	f := store.AuditFilter{EventID: q.Get("event"), Action: q.Get("action"), Search: q.Get("q"), Since: q.Get("since")}
	f.AccountID, _ = strconv.Atoi(q.Get("account"))
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	if a.Role != "admin" {
		// Non-admins only see the journal of events they own.
		if f.EventID == "" || s.Store.Level(a.ID, f.EventID) != "owner" {
			writeErr(w, http.StatusForbidden, errors.New("journal réservé aux administrateurs et aux propriétaires de l'événement"))
			return
		}
	}
	entries, total, err := s.Store.AuditLog(f)
	if err != nil {
		writeAnyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": total})
}

// ---------------------------------------------------------------- system

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": Version,
		"engine":  s.Engine.Status(),
		"ffe":     s.FFE.Status(),
		"dataDir": s.DataDir,
		"logDir":  s.LogDir,
	})
}

func (s *Server) reinstallEngine(w http.ResponseWriter, r *http.Request) {
	if s.Reinstall == nil {
		writeErr(w, http.StatusNotImplemented, errors.New("réinstallation indisponible"))
		return
	}
	if err := s.Reinstall(r.Context()); err != nil {
		writeAnyErr(w, err)
		return
	}
	st := s.Engine.Status()
	s.audit_(r, "", 0, "system.engine", st.Source, st.Path)
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) ffeStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.FFE.Status())
}

func (s *Server) ffeDownload(w http.ResponseWriter, r *http.Request) {
	if s.FFE.Status().State == "loading" {
		writeErr(w, http.StatusConflict, errors.New("chargement déjà en cours"))
		return
	}
	s.audit_(r, "", 0, "ffe.download", s.FFE.URL, "")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if err := s.FFE.Download(ctx, nil); err != nil {
			s.Log.Error("base FFE", "err", err)
		} else {
			s.Log.Info("base FFE chargée", "players", s.FFE.Status().Players)
		}
	}()
	writeJSON(w, http.StatusAccepted, s.FFE.Status())
}

func (s *Server) ffeUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("fichier manquant : %w", err))
		return
	}
	defer f.Close()
	if err := s.FFE.Import(f, hdr.Filename); err != nil {
		writeAnyErr(w, err)
		return
	}
	s.audit_(r, "", 0, "ffe.upload", hdr.Filename, fmt.Sprintf("%d joueurs", s.FFE.Status().Players))
	writeJSON(w, http.StatusOK, s.FFE.Status())
}

func (s *Server) lookupFFE(w http.ResponseWriter, r *http.Request) {
	if !s.FFE.Ready() {
		writeErr(w, http.StatusConflict, errors.New("base FFE non chargée : téléchargez-la ou importez Data.mdb (Système)"))
		return
	}
	q := r.URL.Query().Get("q")
	res := []lookup.Player{}
	if len(strings.TrimSpace(q)) >= 2 {
		if found := s.FFE.Search(q, 30); found != nil {
			res = found
		}
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) lookupFIDE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("fideId")
	if r.URL.Query().Get("online") != "1" && s.FFE.Ready() {
		if p, err := s.FFE.ByFIDE(id); err == nil {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	p, err := s.FIDE.Get(ctx, id)
	if err != nil {
		if errors.Is(err, lookup.ErrNotFound) {
			writeAnyErr(w, err)
		} else {
			writeErr(w, http.StatusBadGateway, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, p)
}
