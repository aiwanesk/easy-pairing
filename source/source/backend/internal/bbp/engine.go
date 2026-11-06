// Package bbp runs the bbpPairings engine (FIDE Dutch / Burstein) and
// installs it: download of the latest official release, or the copy
// embedded in the Windows build.
package bbp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNoPairing is returned when the engine reports that no legal pairing
// exists (exit code 1).
var ErrNoPairing = errors.New("aucun appariement valide n'existe pour cette ronde (règles absolues FIDE)")

// Engine wraps the bbpPairings executable.
type Engine struct {
	mu      sync.RWMutex
	path    string
	version string
	source  string // where the binary came from (release tag, embedded, local)
	Timeout time.Duration
}

// Status describes the installed engine.
type Status struct {
	Ready   bool   `json:"ready"`
	Path    string `json:"path"`
	Source  string `json:"source"`
	Version string `json:"version"`
}

// Status returns the engine state.
func (e *Engine) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return Status{Ready: e.path != "", Path: e.path, Source: e.source, Version: e.version}
}

// Set installs a binary path.
func (e *Engine) Set(path, source, version string) {
	e.mu.Lock()
	e.path, e.source, e.version = path, source, version
	e.mu.Unlock()
}

func (e *Engine) binary() (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.path == "" {
		return "", errors.New("moteur bbpPairings non installé")
	}
	return e.path, nil
}

func (e *Engine) run(ctx context.Context, args ...string) (stdout string, code int, err error) {
	bin, err := e.binary()
	if err != nil {
		return "", -1, err
	}
	timeout := e.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	hideWindow(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", -1, fmt.Errorf("bbpPairings : délai dépassé (%s)", timeout)
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.String(), exit.ExitCode(), nil
		}
		return "", -1, fmt.Errorf("bbpPairings : %w", err)
	}
	return out.String(), 0, nil
}

func systemFlag(system string) string {
	if system == "burstein" {
		return "--burstein"
	}
	return "--dutch"
}

// Pair runs the engine on a TRF and returns the pairing file content.
func (e *Engine) Pair(ctx context.Context, system, trf string) (string, error) {
	dir, err := os.MkdirTemp("", "pairing-easy-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "round.trf")
	outFile := filepath.Join(dir, "pairs.txt")
	if err := os.WriteFile(in, []byte(trf), 0o600); err != nil {
		return "", err
	}
	msg, code, err := e.run(ctx, systemFlag(system), in, "-p", outFile)
	if err != nil {
		return "", err
	}
	switch code {
	case 0:
	case 1:
		return "", ErrNoPairing
	case 3:
		return "", fmt.Errorf("bbpPairings : fichier TRF refusé : %s", strings.TrimSpace(msg))
	case 4:
		return "", fmt.Errorf("bbpPairings : tournoi trop grand : %s", strings.TrimSpace(msg))
	default:
		return "", fmt.Errorf("bbpPairings (code %d) : %s", code, strings.TrimSpace(msg))
	}
	res, err := os.ReadFile(outFile)
	if err != nil {
		return "", fmt.Errorf("bbpPairings : sortie manquante : %w", err)
	}
	return string(res), nil
}

// Check re-pairs every round of a TRF and returns the differences found
// (empty when every round matches what the engine would have produced).
func (e *Engine) Check(ctx context.Context, system, trf string) (string, error) {
	dir, err := os.MkdirTemp("", "pairing-easy-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "check.trf")
	if err := os.WriteFile(in, []byte(trf), 0o600); err != nil {
		return "", err
	}
	out, code, err := e.run(ctx, systemFlag(system), in, "-c")
	if err != nil {
		return "", err
	}
	if code != 0 && !strings.Contains(out, "Checker pairings") {
		return "", fmt.Errorf("bbpPairings -c (code %d) : %s", code, strings.TrimSpace(out))
	}
	if !strings.Contains(out, "Checker pairings") {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

// SelfTest pairs a tiny tournament to make sure the binary works.
func (e *Engine) SelfTest(ctx context.Context) error {
	trf := "012 test\n" +
		"001    1      A                                 2000                             0.0    1\n" +
		"001    2      B                                 1900                             0.0    2\n" +
		"XXR 1\nXXC white1\n"
	out, err := e.Pair(ctx, "dutch", trf)
	if err != nil {
		return err
	}
	if f := strings.Fields(out); len(f) != 3 || f[0] != "1" {
		return fmt.Errorf("réponse inattendue : %q", out)
	}
	return nil
}
