package api

import (
	"errors"
	"net/http"
)

// NetworkInfo describes where the server can be reached.
type NetworkInfo struct {
	LAN       bool     `json:"lan"`       // reachable from other devices of the local network
	Forced    bool     `json:"forced"`    // -lan given on the command line (cannot be turned off here)
	Local     string   `json:"local"`     // address on this computer
	Addresses []string `json:"addresses"` // addresses for other devices (empty when LAN is off)
	Error     string   `json:"error,omitempty"`
	PublicURL string   `json:"publicUrl,omitempty"` // address on Internet (online platform)
	Server    bool     `json:"server"`              // online platform mode
}

// Network lets the HTTP layer read and switch local-network access. It is
// implemented by the command (which owns the listeners).
type Network interface {
	Info() NetworkInfo
	SetLAN(on bool) error
}

func (s *Server) networkInfo(w http.ResponseWriter, r *http.Request) {
	if s.Net == nil {
		writeJSON(w, http.StatusOK, NetworkInfo{Addresses: []string{}})
		return
	}
	writeJSON(w, http.StatusOK, s.Net.Info())
}

func (s *Server) setNetwork(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LAN bool `json:"lan"`
	}
	if err := decode(w, r, &in); err != nil {
		writeAnyErr(w, err)
		return
	}
	if s.Net == nil {
		writeErr(w, http.StatusNotImplemented, errors.New("réglage réseau indisponible"))
		return
	}
	if err := s.Net.SetLAN(in.LAN); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.Store.SetSetting("lan", map[bool]string{true: "1", false: "0"}[in.LAN])
	detail := "désactivé"
	if in.LAN {
		detail = "activé"
	}
	s.audit_(r, "", 0, "system.lan", "réseau local", detail)
	writeJSON(w, http.StatusOK, s.Net.Info())
}
