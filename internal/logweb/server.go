package logweb

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/status"
)

type RuntimeSnapshotProvider interface {
	Snapshot() status.RuntimeSnapshot
}

type Server struct {
	config appconfig.LogWebConfig
	status RuntimeSnapshotProvider
	mux    *http.ServeMux
}

func NewServer(config appconfig.LogWebConfig, store RuntimeSnapshotProvider) (*Server, error) {
	if !config.Enable {
		return nil, fmt.Errorf("log web is disabled")
	}
	if config.Bind == "" {
		config.Bind = "127.0.0.1"
	}
	if config.Port <= 0 {
		return nil, fmt.Errorf("log web port is required")
	}
	if store == nil {
		return nil, fmt.Errorf("status provider is required")
	}

	server := &Server{
		config: config,
		status: store,
		mux:    http.NewServeMux(),
	}
	server.routes()
	return server, nil
}

func (s *Server) Addr() string {
	return net.JoinHostPort(s.config.Bind, strconv.Itoa(s.config.Port))
}

func (s *Server) Handler() http.Handler {
	return s.auth(s.mux)
}

func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(s.Addr(), s.Handler())
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})

	s.mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, s.status.Snapshot())
	})

	s.mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, s.status.Snapshot().Logs)
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.Token != "" && bearerToken(r) != s.config.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
