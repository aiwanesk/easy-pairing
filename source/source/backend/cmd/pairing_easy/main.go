// Command pairing_easy is the whole application in one executable: it
// installs the bbpPairings engine (latest official release, or the copy
// embedded in the Windows build), serves the API and the embedded
// front-end, and opens the browser.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"pairingeasy/internal/api"
	"pairingeasy/internal/applog"
	"pairingeasy/internal/bbp"
	"pairingeasy/internal/lookup"
	"pairingeasy/internal/store"
	"pairingeasy/internal/web"
)

func defaultDataDir() string {
	switch runtime.GOOS {
	case "windows":
		// Not in Documents/Desktop: OneDrive may sync and lock the database.
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "pairing_easy")
		}
	case "darwin":
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, "Library", "Application Support", "pairing_easy")
		}
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, "pairing_easy")
		}
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, ".local", "share", "pairing_easy")
		}
	}
	return "data"
}

func main() {
	port := flag.Int("port", 8080, "port HTTP")
	lan := flag.Bool("lan", false, "écouter sur le réseau local (écrans, kiosques, inscriptions depuis d'autres appareils)")
	dataDir := flag.String("data", defaultDataDir(), "répertoire des données")
	bbpPath := flag.String("bbp", "", "chemin d'un exécutable bbpPairings (sinon installation automatique)")
	noBrowser := flag.Bool("no-browser", false, "ne pas ouvrir le navigateur")
	logLevel := flag.String("log-level", "info", "niveau de journalisation : debug, info, warn, error")
	fideURL := flag.String("fide-url", "https://ratings.fide.com", "base URL des fiches FIDE")
	papiURL := flag.String("papi-url", lookup.DefaultPapiURL, "URL de PapiData.zip (base FFE)")
	showVersion := flag.Bool("version", false, "afficher la version")
	listen := flag.String("listen", "", "plateforme en ligne : adresse d'écoute (ex. :8080 ou 0.0.0.0:8080) ; désactive l'ouverture du navigateur")
	publicURL := flag.String("public-url", "", "plateforme en ligne : adresse publique (ex. https://appariements.example.org)")
	trustProxy := flag.Bool("trust-proxy", false, "plateforme en ligne : derrière un proxy HTTPS (en-têtes X-Forwarded-*)")
	flag.Parse()
	if v := os.Getenv("PAIRING_EASY_LISTEN"); v != "" && *listen == "" {
		*listen = v
	}
	if v := os.Getenv("PAIRING_EASY_PUBLIC_URL"); v != "" && *publicURL == "" {
		*publicURL = v
	}
	if os.Getenv("PAIRING_EASY_TRUST_PROXY") == "1" {
		*trustProxy = true
	}
	if v := os.Getenv("PAIRING_EASY_DATA"); v != "" {
		*dataDir = v
	}
	srvOpts = serverOptions{listen: *listen, publicURL: strings.TrimRight(*publicURL, "/"), trustProxy: *trustProxy}
	if *listen != "" {
		*noBrowser = true
	}

	if *showVersion {
		fmt.Println("pairing_easy", api.Version)
		return
	}
	if err := run(*port, *lan, *dataDir, *bbpPath, *noBrowser, *logLevel, *fideURL, *papiURL); err != nil {
		fmt.Fprintln(os.Stderr, "ERREUR :", err)
		if runtime.GOOS == "windows" {
			fmt.Fprintln(os.Stderr, "Appuyez sur Entrée pour fermer.")
			fmt.Scanln()
		}
		os.Exit(1)
	}
}

type serverOptions struct {
	listen, publicURL string
	trustProxy        bool
}

var srvOpts serverOptions

func run(port int, lan bool, dataDir, bbpPath string, noBrowser bool, logLevel, fideURL, papiURL string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	logDir := filepath.Join(dataDir, "logs")
	log, err := applog.Setup(logDir, logLevel, 30, os.Stdout)
	if err != nil {
		return err
	}

	// Always listen on the loopback; local-network access adds one listener
	// per LAN address and can be switched on and off from the interface.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)
	if srvOpts.listen != "" {
		addr = srvOpts.listen
		if _, p, err := net.SplitHostPort(addr); err == nil {
			url = "http://localhost:" + p
		}
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil && srvOpts.listen == "" {
		// Another version running? Ask it to quit, then take its place.
		if v := runningVersion(url); v != "" && v != api.Version {
			log.Info("une autre version tourne : arrêt demandé", "running", v, "new", api.Version)
			fmt.Printf("  Remplacement de la version %s en cours d'exécution…\n", v)
			if stopRunning(url) {
				for i := 0; i < 50 && err != nil; i++ {
					time.Sleep(100 * time.Millisecond)
					ln, err = net.Listen("tcp", addr)
				}
			}
		}
	}
	if err != nil {
		// Already running? Then just open the browser on it.
		if alreadyRunning(url) {
			log.Info("pairing_easy tourne déjà, ouverture du navigateur", "url", url)
			if !noBrowser {
				openBrowser(url)
			}
			return nil
		}
		return fmt.Errorf("le port %d est occupé (option -port pour en choisir un autre) : %w", port, err)
	}

	st, err := store.Open(filepath.Join(dataDir, "pairing_easy.db"))
	if err != nil {
		return fmt.Errorf("base de données : %w", err)
	}
	defer st.Close()
	st.PurgeSessions()

	engine := &bbp.Engine{Timeout: 2 * time.Minute}
	engineDir := filepath.Join(dataDir, "engine")
	install := func(ctx context.Context, force bool) error {
		if force {
			os.Remove(filepath.Join(engineDir, map[bool]string{true: "bbpPairings.exe", false: "bbpPairings"}[runtime.GOOS == "windows"]))
		}
		return bbp.Install(ctx, engine, engineDir, bbpPath, log)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := install(ctx, false); err != nil {
			log.Error("moteur bbpPairings indisponible", "err", err)
		}
	}()

	ffe := lookup.NewFFEBase(filepath.Join(dataDir, "ffe"))
	ffe.URL = papiURL
	go func() {
		if err := ffe.LoadExisting(); err != nil {
			log.Error("base FFE", "err", err)
		} else if s := ffe.Status(); s.Players > 0 {
			log.Info("base FFE chargée", "players", s.Players)
		}
	}()

	netw := &lanNet{port: port, forced: lan, local: url, log: log, server: srvOpts.listen != "", publicURL: srvOpts.publicURL}
	srv := &api.Server{
		Store: st, Engine: engine, FFE: ffe, FIDE: &lookup.FIDEClient{BaseURL: fideURL}, Log: log,
		Static: web.FS(), DataDir: dataDir, LogDir: logDir, Net: netw, TrustProxy: srvOpts.trustProxy,
		Reinstall: func(ctx context.Context) error { return install(ctx, true) },
	}
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
	netw.srv = httpSrv
	if srvOpts.listen == "" && (lan || st.Setting("lan") == "1") {
		if err := netw.SetLAN(true); err != nil {
			log.Error("réseau local", "err", err)
		}
	}

	fmt.Println()
	fmt.Println("  pairing_easy", api.Version)
	fmt.Println("  Ouvrez", url, "dans votre navigateur.")
	for _, a := range netw.Info().Addresses {
		fmt.Println("  Réseau local :", a)
	}
	fmt.Println("  Données :", dataDir)
	fmt.Println("  Fermez cette fenêtre (ou Ctrl+C) pour arrêter.")
	fmt.Println()
	log.Info("démarrage", "version", api.Version, "addr", addr, "data", dataDir)

	go func() {
		if !noBrowser {
			time.Sleep(600 * time.Millisecond)
			openBrowser(url)
		}
	}()

	errc := make(chan error, 1)
	go func() { errc <- httpSrv.Serve(ln) }()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	quit := make(chan struct{}, 1)
	if srvOpts.listen == "" {
		srv.Shutdown = func() { quit <- struct{}{} }
	}
	select {
	case <-quit:
		log.Info("arrêt : remplacé par une autre version")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		netw.SetLAN(false)
		httpSrv.Shutdown(ctx)
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-sig:
		log.Info("arrêt demandé")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		netw.SetLAN(false)
		httpSrv.Shutdown(ctx)
	}
	return nil
}

// lanNet serves the application on every IPv4 address of the local network
// in addition to the loopback, on demand.
type lanNet struct {
	mu      sync.Mutex
	port    int
	forced  bool
	local   string
	srv     *http.Server
	log     *slog.Logger
	ls      []net.Listener
	lastErr string
	// Online platform: the main listener is already public.
	server    bool
	publicURL string
}

func (n *lanNet) Info() api.NetworkInfo {
	n.mu.Lock()
	defer n.mu.Unlock()
	info := api.NetworkInfo{LAN: len(n.ls) > 0 || n.server, Forced: n.forced || n.server, Local: n.local, Addresses: []string{}, Error: n.lastErr,
		PublicURL: n.publicURL, Server: n.server}
	for _, l := range n.ls {
		info.Addresses = append(info.Addresses, "http://"+l.Addr().String())
	}
	return info
}

func (n *lanNet) SetLAN(on bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.server {
		return errors.New("plateforme en ligne : l'accès réseau dépend de la configuration du serveur")
	}
	if !on {
		if n.forced {
			return errors.New("l'option -lan est active : relancez sans elle pour couper l'accès réseau")
		}
		for _, l := range n.ls {
			l.Close()
		}
		n.ls, n.lastErr = nil, ""
		return nil
	}
	if len(n.ls) > 0 {
		return nil
	}
	var errs []string
	for _, ip := range lanAddresses() {
		l, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(n.port)))
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		n.ls = append(n.ls, l)
		go n.srv.Serve(l)
	}
	n.lastErr = strings.Join(errs, " ; ")
	if len(n.ls) == 0 {
		if n.lastErr == "" {
			n.lastErr = "aucune adresse réseau local trouvée (Wi-Fi ou câble branché ?)"
		}
		return errors.New(n.lastErr)
	}
	n.log.Info("réseau local activé", "addresses", len(n.ls))
	return nil
}

// runningVersion returns the version of a pairing_easy answering on url.
func runningVersion(url string) string {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(url + "/api/session")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var s struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&s) != nil {
		return ""
	}
	return s.Version
}

func stopRunning(url string) bool {
	req, _ := http.NewRequest(http.MethodPost, url+"/api/local/shutdown", nil)
	req.Header.Set("X-Requested-With", "pairing-easy")
	req.Header.Set("X-Pairing-Easy-Version", api.Version)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func alreadyRunning(url string) bool {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(url + "/api/session")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK && strings.Contains(resp.Header.Get("Content-Type"), "json")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("ouverture du navigateur impossible", "err", err)
	}
}

func lanAddresses() []string {
	var out []string
	ifaces, _ := net.InterfaceAddrs()
	for _, a := range ifaces {
		if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() && ipn.IP.To4() != nil {
			out = append(out, ipn.IP.String())
		}
	}
	return out
}
