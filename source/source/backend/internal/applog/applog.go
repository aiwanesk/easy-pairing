// Package applog configures structured logging (log/slog): human-readable
// text on the console and JSON lines in daily files, purged after a
// retention period.
package applog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dailyFile is an io.Writer that switches file at midnight.
type dailyFile struct {
	mu   sync.Mutex
	dir  string
	day  string
	file *os.File
}

func (d *dailyFile) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	day := time.Now().Format("2006-01-02")
	if d.file == nil || day != d.day {
		if d.file != nil {
			d.file.Close()
		}
		f, err := os.OpenFile(filepath.Join(d.dir, "pairing_easy-"+day+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, err
		}
		d.file, d.day = f, day
	}
	return d.file.Write(p)
}

// multiHandler sends records to several handlers.
type multiHandler []slog.Handler

func (m multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (m multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m {
		if h.Enabled(ctx, r.Level) {
			h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (m multiHandler) WithAttrs(a []slog.Attr) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithAttrs(a)
	}
	return out
}

func (m multiHandler) WithGroup(n string) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithGroup(n)
	}
	return out
}

// Setup installs the default logger. level is debug|info|warn|error.
func Setup(dir, level string, retentionDays int, console io.Writer) (*slog.Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	purge(dir, retentionDays)
	h := multiHandler{
		slog.NewTextHandler(console, &slog.HandlerOptions{Level: lv}),
		slog.NewJSONHandler(&dailyFile{dir: dir}, &slog.HandlerOptions{Level: lv}),
	}
	l := slog.New(h)
	slog.SetDefault(l)
	return l, nil
}

func purge(dir string, days int) {
	if days <= 0 {
		return
	}
	limit := time.Now().AddDate(0, 0, -days)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "pairing_easy-") || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(limit) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
