package bbp

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ReleasesAPI is the GitHub endpoint of the latest official release.
const ReleasesAPI = "https://api.github.com/repos/BieremaBoyzProgramming/bbpPairings/releases/latest"

func exeName() string {
	if runtime.GOOS == "windows" {
		return "bbpPairings.exe"
	}
	return "bbpPairings"
}

// Install makes the engine available in dir: an already installed copy
// that passes the self-test, else the latest official release downloaded
// from GitHub, else the copy embedded in this build. explicit, when set, is
// used as is.
func Install(ctx context.Context, e *Engine, dir, explicit string, log *slog.Logger) error {
	if explicit != "" {
		e.Set(explicit, "chemin fourni", "")
		return e.SelfTest(ctx)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(dir, exeName())
	meta := filepath.Join(dir, "version.json")

	var info struct{ Source, Version string }
	if b, err := os.ReadFile(meta); err == nil {
		json.Unmarshal(b, &info)
	}
	if _, err := os.Stat(target); err == nil {
		e.Set(target, info.Source, info.Version)
		if err := e.SelfTest(ctx); err == nil {
			log.Info("moteur bbpPairings prêt", "path", target, "source", info.Source, "version", info.Version)
			return nil
		}
		log.Warn("moteur bbpPairings installé défectueux, réinstallation", "path", target)
	}

	save := func(data []byte, source, version string) error {
		tmp := target + ".tmp"
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, target); err != nil {
			return err
		}
		e.Set(target, source, version)
		if err := e.SelfTest(ctx); err != nil {
			os.Remove(target)
			e.Set("", "", "")
			return fmt.Errorf("auto-test échoué : %w", err)
		}
		b, _ := json.Marshal(map[string]string{"Source": source, "Version": version})
		os.WriteFile(meta, b, 0o644)
		log.Info("moteur bbpPairings installé", "path", target, "source", source, "version", version)
		return nil
	}

	data, tag, derr := downloadRelease(ctx)
	if derr == nil {
		if err := save(data, "GitHub "+tag, tag); err == nil {
			return nil
		} else {
			derr = err
		}
	}
	log.Warn("téléchargement de bbpPairings impossible", "err", derr)
	if embedded := embeddedBinary(); embedded != nil {
		return save(embedded, "copie intégrée (bbpPairings "+embeddedVersion+")", embeddedVersion)
	}
	return fmt.Errorf("bbpPairings introuvable : %v ; compilez-le (make bbp) ou indiquez son chemin avec -bbp", derr)
}

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// downloadRelease fetches the binary for this OS from the latest release.
func downloadRelease(ctx context.Context) ([]byte, string, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ReleasesAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "pairing-easy")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GitHub : HTTP %d", resp.StatusCode)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, "", err
	}
	want := map[string][]string{
		"windows": {"win", ".exe"},
		"linux":   {"linux"},
		"darwin":  {"mac", "darwin", "osx"},
	}[runtime.GOOS]
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		match := false
		for _, w := range want {
			if strings.Contains(name, w) {
				match = true
			}
		}
		if !match {
			continue
		}
		data, err := fetch(ctx, client, a.URL)
		if err != nil {
			return nil, "", err
		}
		if strings.HasSuffix(name, ".zip") {
			data, err = extractExe(data)
			if err != nil {
				return nil, "", fmt.Errorf("%s : %w", a.Name, err)
			}
		}
		return data, rel.TagName, nil
	}
	return nil, "", fmt.Errorf("aucun binaire %s dans la release %s", runtime.GOOS, rel.TagName)
}

func fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "pairing-easy")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func extractExe(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if base == strings.ToLower(exeName()) || (runtime.GOOS == "windows" && strings.HasSuffix(base, ".exe")) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 64<<20))
		}
	}
	return nil, errors.New("exécutable absent de l'archive")
}
