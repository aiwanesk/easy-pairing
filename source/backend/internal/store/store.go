// Package store persists accounts, sessions, permissions, events and the
// audit log in a single SQLite database (pure Go driver, no cgo).
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"

	"pairingeasy/internal/chess"
)

var (
	ErrNotFound = errors.New("introuvable")
	ErrConflict = errors.New("modifié entre-temps par quelqu'un d'autre, rechargez la page")
)

// Store is the application database.
type Store struct {
	db    *sql.DB
	locks sync.Map // event id -> *sync.Mutex
}

const schemaVersion = 2

var migrations = []string{
	// v1
	`CREATE TABLE accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT NOT NULL UNIQUE COLLATE NOCASE,
		name TEXT NOT NULL,
		email TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL CHECK (role IN ('admin','user')),
		password_hash TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		last_login TEXT
	);
	CREATE TABLE sessions (
		token_hash TEXT PRIMARY KEY,
		account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		ip TEXT NOT NULL DEFAULT '',
		user_agent TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX sessions_account ON sessions(account_id);
	CREATE TABLE events (
		id TEXT PRIMARY KEY,
		slug TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		data TEXT NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		created_by INTEGER,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE event_permissions (
		account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		level TEXT NOT NULL CHECK (level IN ('owner','arbiter','viewer')),
		PRIMARY KEY (account_id, event_id)
	);
	CREATE TABLE audit (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		at TEXT NOT NULL,
		account_id INTEGER,
		account_name TEXT NOT NULL DEFAULT '',
		ip TEXT NOT NULL DEFAULT '',
		event_id TEXT NOT NULL DEFAULT '',
		tournament_id INTEGER NOT NULL DEFAULT 0,
		action TEXT NOT NULL,
		target TEXT NOT NULL DEFAULT '',
		details TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX audit_at ON audit(at);
	CREATE INDEX audit_event ON audit(event_id, at);
	CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);`,
	// v2: role "manager" (may create events; existing users keep that right)
	// and event level "results" (read-only plus result entry). SQLite cannot
	// change a CHECK constraint: tables are rebuilt (foreign keys off, see
	// migrate).
	`CREATE TABLE accounts_v2 (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		login TEXT NOT NULL UNIQUE COLLATE NOCASE,
		name TEXT NOT NULL,
		email TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL CHECK (role IN ('admin','manager','user')),
		password_hash TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		last_login TEXT
	);
	INSERT INTO accounts_v2(id, login, name, email, role, password_hash, active, created_at, last_login)
		SELECT id, login, name, email, CASE role WHEN 'user' THEN 'manager' ELSE role END, password_hash, active, created_at, last_login FROM accounts;
	DROP TABLE accounts;
	ALTER TABLE accounts_v2 RENAME TO accounts;
	CREATE TABLE event_permissions_v2 (
		account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		level TEXT NOT NULL CHECK (level IN ('owner','arbiter','results','viewer')),
		PRIMARY KEY (account_id, event_id)
	);
	INSERT INTO event_permissions_v2 SELECT account_id, event_id, level FROM event_permissions;
	DROP TABLE event_permissions;
	ALTER TABLE event_permissions_v2 RENAME TO event_permissions;`,
}

// Open opens (and migrates) the database file.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(wal)&_pragma=foreign_keys(1)&_pragma=synchronous(normal)"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	// One connection with foreign keys off: table rebuilds must not cascade.
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	var v int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	for v < len(migrations) {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", v+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		v++
	}
	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return errors.New("base de données : contrôle des clés étrangères en échec après migration")
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func parseTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil
	}
	return &t
}

// ---------------------------------------------------------------- settings

// Setting returns a stored setting or "".
func (s *Store) Setting(key string) string {
	var v string
	s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	return v
}

// SetSetting stores a setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

// ---------------------------------------------------------------- accounts

// Account is a user of the application.
type Account struct {
	ID        int        `json:"id"`
	Login     string     `json:"login"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Role      string     `json:"role"` // admin | user
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"createdAt"`
	LastLogin *time.Time `json:"lastLogin"`
	hash      string
}

// PasswordHash returns the stored hash (for verification by package auth).
func (a *Account) PasswordHash() string { return a.hash }

const accountCols = "id, login, name, email, role, password_hash, active, created_at, last_login"

func scanAccount(row interface{ Scan(...any) error }) (*Account, error) {
	var a Account
	var created string
	var last sql.NullString
	var active int
	if err := row.Scan(&a.ID, &a.Login, &a.Name, &a.Email, &a.Role, &a.hash, &active, &created, &last); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.Active = active == 1
	if t := parseTime(sql.NullString{String: created, Valid: true}); t != nil {
		a.CreatedAt = *t
	}
	a.LastLogin = parseTime(last)
	return &a, nil
}

// CountAccounts returns the number of accounts.
func (s *Store) CountAccounts() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&n)
	return n, err
}

// CreateAccount inserts an account with an already hashed password.
func (s *Store) CreateAccount(a *Account, passwordHash string) error {
	a.Login = strings.TrimSpace(a.Login)
	if a.Login == "" || strings.TrimSpace(a.Name) == "" {
		return errors.New("identifiant et nom obligatoires")
	}
	if a.Role != "admin" && a.Role != "manager" {
		a.Role = "user"
	}
	res, err := s.db.Exec("INSERT INTO accounts(login, name, email, role, password_hash, active, created_at) VALUES(?,?,?,?,?,1,?)",
		a.Login, strings.TrimSpace(a.Name), strings.TrimSpace(a.Email), a.Role, passwordHash, now())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("l'identifiant %q existe déjà", a.Login)
		}
		return err
	}
	id, _ := res.LastInsertId()
	a.ID = int(id)
	a.Active = true
	a.hash = passwordHash
	return nil
}

// Account returns an account by id.
func (s *Store) Account(id int) (*Account, error) {
	return scanAccount(s.db.QueryRow("SELECT "+accountCols+" FROM accounts WHERE id = ?", id))
}

// AccountByLogin returns an account by login (case-insensitive).
func (s *Store) AccountByLogin(login string) (*Account, error) {
	return scanAccount(s.db.QueryRow("SELECT "+accountCols+" FROM accounts WHERE login = ?", strings.TrimSpace(login)))
}

// Accounts lists every account.
func (s *Store) Accounts() ([]*Account, error) {
	rows, err := s.db.Query("SELECT " + accountCols + " FROM accounts ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAccount changes name, email, role and active flag.
func (s *Store) UpdateAccount(a *Account) error {
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("le nom est obligatoire")
	}
	if a.Role != "admin" && a.Role != "manager" {
		a.Role = "user"
	}
	active := 0
	if a.Active {
		active = 1
	}
	res, err := s.db.Exec("UPDATE accounts SET name = ?, email = ?, role = ?, active = ? WHERE id = ?",
		strings.TrimSpace(a.Name), strings.TrimSpace(a.Email), a.Role, active, a.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if !a.Active {
		s.db.Exec("DELETE FROM sessions WHERE account_id = ?", a.ID)
	}
	return nil
}

// SetPassword replaces an account's password hash and signs it out everywhere.
func (s *Store) SetPassword(id int, hash string) error {
	if _, err := s.db.Exec("UPDATE accounts SET password_hash = ? WHERE id = ?", hash, id); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM sessions WHERE account_id = ?", id)
	return err
}

// DeleteAccount removes an account.
func (s *Store) DeleteAccount(id int) error {
	_, err := s.db.Exec("DELETE FROM accounts WHERE id = ?", id)
	return err
}

// CountActiveAdmins returns the number of active administrators.
func (s *Store) CountActiveAdmins() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE role = 'admin' AND active = 1").Scan(&n)
	return n, err
}

// ---------------------------------------------------------------- sessions

// CreateSession stores a session token hash.
func (s *Store) CreateSession(tokenHash string, accountID int, ttl time.Duration, ip, ua string) error {
	t := time.Now().UTC()
	_, err := s.db.Exec("INSERT INTO sessions(token_hash, account_id, created_at, expires_at, ip, user_agent) VALUES(?,?,?,?,?,?)",
		tokenHash, accountID, t.Format(time.RFC3339Nano), t.Add(ttl).Format(time.RFC3339Nano), ip, truncate(ua, 300))
	if err == nil {
		s.db.Exec("UPDATE accounts SET last_login = ? WHERE id = ?", t.Format(time.RFC3339Nano), accountID)
	}
	return err
}

// SessionAccount returns the active account of a valid session.
func (s *Store) SessionAccount(tokenHash string) (*Account, error) {
	row := s.db.QueryRow(`SELECT a.id, a.login, a.name, a.email, a.role, a.password_hash, a.active, a.created_at, a.last_login
		FROM sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = ? AND s.expires_at > ? AND a.active = 1`, tokenHash, now())
	return scanAccount(row)
}

// DeleteSession removes a session.
func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}

// PurgeSessions removes expired sessions.
func (s *Store) PurgeSessions() {
	s.db.Exec("DELETE FROM sessions WHERE expires_at <= ?", now())
}

// ---------------------------------------------------------------- events

// EventSummary is the list view of an event.
type EventSummary struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Location    string    `json:"location"`
	StartDate   string    `json:"startDate"`
	EndDate     string    `json:"endDate"`
	Public      bool      `json:"public"`
	Tournaments int       `json:"tournaments"`
	Players     int       `json:"players"`
	Level       string    `json:"level"` // caller's permission
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (s *Store) lock(id string) *sync.Mutex {
	m, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return m.(*sync.Mutex)
}

func decodeEvent(data string) (*chess.Event, error) {
	var e chess.Event
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		return nil, err
	}
	e.Normalize()
	return &e, nil
}

// CreateEvent stores a new event; creator becomes its owner.
func (s *Store) CreateEvent(e *chess.Event, creator int) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	t := now()
	if _, err := tx.Exec("INSERT INTO events(id, slug, name, data, version, created_by, created_at, updated_at) VALUES(?,?,?,?,1,?,?,?)",
		e.ID, e.Slug, e.Name, string(data), creator, t, t); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("l'adresse publique %q est déjà utilisée", e.Slug)
		}
		return err
	}
	if creator > 0 {
		if _, err := tx.Exec("INSERT INTO event_permissions(account_id, event_id, level) VALUES(?,?,'owner')", creator, e.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Event loads an event by id.
func (s *Store) Event(id string) (*chess.Event, error) {
	var data string
	if err := s.db.QueryRow("SELECT data FROM events WHERE id = ?", id).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return decodeEvent(data)
}

// EventBySlug loads an event by its public slug.
func (s *Store) EventBySlug(slug string) (*chess.Event, error) {
	var data string
	if err := s.db.QueryRow("SELECT data FROM events WHERE slug = ?", slug).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return decodeEvent(data)
}

// UpdateEvent applies fn to the current event under the event's lock and
// saves it. fn may run the pairing engine.
func (s *Store) UpdateEvent(ctx context.Context, id string, fn func(*chess.Event) error) (*chess.Event, error) {
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	var data string
	var version int
	if err := s.db.QueryRowContext(ctx, "SELECT data, version FROM events WHERE id = ?", id).Scan(&data, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	e, err := decodeEvent(data)
	if err != nil {
		return nil, err
	}
	if err := fn(e); err != nil {
		return nil, err
	}
	e.ID = id
	e.UpdatedAt = time.Now().UTC()
	e.Normalize()
	out, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, "UPDATE events SET data = ?, name = ?, slug = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?",
		string(out), e.Name, e.Slug, now(), id, version)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("l'adresse publique %q est déjà utilisée", e.Slug)
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrConflict
	}
	return e, nil
}

// DeleteEvent removes an event.
func (s *Store) DeleteEvent(id string) error {
	_, err := s.db.Exec("DELETE FROM events WHERE id = ?", id)
	return err
}

// Events lists events visible to an account (all for admins). Public
// listing uses accountID = -1 (public events only).
func (s *Store) Events(accountID int, admin bool) ([]EventSummary, error) {
	q := `SELECT e.id, e.data, e.updated_at, COALESCE(p.level, '') FROM events e
		LEFT JOIN event_permissions p ON p.event_id = e.id AND p.account_id = ?`
	if !admin {
		q += " WHERE p.level IS NOT NULL"
	}
	q += " ORDER BY e.updated_at DESC"
	rows, err := s.db.Query(q, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventSummary
	for rows.Next() {
		var id, data, updated, level string
		if err := rows.Scan(&id, &data, &updated, &level); err != nil {
			return nil, err
		}
		e, err := decodeEvent(data)
		if err != nil {
			continue
		}
		if admin && level == "" {
			level = "admin"
		}
		players := 0
		for _, t := range e.Tournaments {
			players += len(t.Participants())
		}
		sum := EventSummary{ID: e.ID, Slug: e.Slug, Name: e.Name, Location: e.Location, StartDate: e.StartDate, EndDate: e.EndDate,
			Public: e.Public, Tournaments: len(e.Tournaments), Players: players, Level: level}
		if t := parseTime(sql.NullString{String: updated, Valid: true}); t != nil {
			sum.UpdatedAt = *t
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// PublicEvents lists the events marked public.
func (s *Store) PublicEvents() ([]EventSummary, error) {
	all, err := s.Events(0, true)
	if err != nil {
		return nil, err
	}
	var out []EventSummary
	for _, e := range all {
		if e.Public {
			e.Level = ""
			out = append(out, e)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- permissions

// Permission links an account to an event.
type Permission struct {
	AccountID int    `json:"accountId"`
	Name      string `json:"name"`
	Login     string `json:"login"`
	Level     string `json:"level"`
}

// Level returns an account's permission level on an event ("" if none).
func (s *Store) Level(accountID int, eventID string) string {
	var l string
	s.db.QueryRow("SELECT level FROM event_permissions WHERE account_id = ? AND event_id = ?", accountID, eventID).Scan(&l)
	return l
}

// Permissions lists the accounts with access to an event.
func (s *Store) Permissions(eventID string) ([]Permission, error) {
	rows, err := s.db.Query(`SELECT a.id, a.name, a.login, p.level FROM event_permissions p JOIN accounts a ON a.id = p.account_id
		WHERE p.event_id = ? ORDER BY a.name COLLATE NOCASE`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Permission{}
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.AccountID, &p.Name, &p.Login, &p.Level); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetPermission grants (or with level "" revokes) access to an event.
func (s *Store) SetPermission(accountID int, eventID, level string) error {
	if level == "" {
		_, err := s.db.Exec("DELETE FROM event_permissions WHERE account_id = ? AND event_id = ?", accountID, eventID)
		return err
	}
	switch level {
	case "owner", "arbiter", "results", "viewer":
	default:
		return fmt.Errorf("niveau inconnu %q", level)
	}
	_, err := s.db.Exec(`INSERT INTO event_permissions(account_id, event_id, level) VALUES(?,?,?)
		ON CONFLICT(account_id, event_id) DO UPDATE SET level = excluded.level`, accountID, eventID, level)
	return err
}

// CountOwners returns the number of owners of an event.
func (s *Store) CountOwners(eventID string) int {
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM event_permissions WHERE event_id = ? AND level = 'owner'", eventID).Scan(&n)
	return n
}

// ---------------------------------------------------------------- audit

// AuditEntry is one line of the audit log.
type AuditEntry struct {
	ID           int64     `json:"id"`
	At           time.Time `json:"at"`
	AccountID    int       `json:"accountId"`
	AccountName  string    `json:"accountName"`
	IP           string    `json:"ip"`
	EventID      string    `json:"eventId"`
	TournamentID int       `json:"tournamentId"`
	Action       string    `json:"action"`
	Target       string    `json:"target"`
	Details      string    `json:"details"`
}

// Audit appends an entry to the audit log.
func (s *Store) Audit(e AuditEntry) error {
	_, err := s.db.Exec("INSERT INTO audit(at, account_id, account_name, ip, event_id, tournament_id, action, target, details) VALUES(?,?,?,?,?,?,?,?,?)",
		now(), nullInt(e.AccountID), e.AccountName, e.IP, e.EventID, e.TournamentID, e.Action, truncate(e.Target, 500), truncate(e.Details, 4000))
	return err
}

// AuditFilter narrows an audit query.
type AuditFilter struct {
	EventID   string
	AccountID int
	Action    string
	Search    string
	Since     string
	Limit     int
	Offset    int
}

// AuditLog returns entries, most recent first, plus the total count.
func (s *Store) AuditLog(f AuditFilter) ([]AuditEntry, int, error) {
	var where []string
	var args []any
	if f.EventID != "" {
		where = append(where, "event_id = ?")
		args = append(args, f.EventID)
	}
	if f.AccountID > 0 {
		where = append(where, "account_id = ?")
		args = append(args, f.AccountID)
	}
	if f.Action != "" {
		where = append(where, "action LIKE ?")
		args = append(args, f.Action+"%")
	}
	if f.Search != "" {
		where = append(where, "(target LIKE ? OR details LIKE ? OR account_name LIKE ?)")
		like := "%" + f.Search + "%"
		args = append(args, like, like, like)
	}
	if f.Since != "" {
		where = append(where, "at >= ?")
		args = append(args, f.Since)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM audit"+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := s.db.Query("SELECT id, at, COALESCE(account_id, 0), account_name, ip, event_id, tournament_id, action, target, details FROM audit"+
		clause+" ORDER BY id DESC LIMIT ? OFFSET ?", append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		var at string
		if err := rows.Scan(&e.ID, &at, &e.AccountID, &e.AccountName, &e.IP, &e.EventID, &e.TournamentID, &e.Action, &e.Target, &e.Details); err != nil {
			return nil, 0, err
		}
		if t := parseTime(sql.NullString{String: at, Valid: true}); t != nil {
			e.At = *t
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
