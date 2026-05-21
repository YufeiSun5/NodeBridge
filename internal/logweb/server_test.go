package logweb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/status"
)

func TestNewServerDisabled(t *testing.T) {
	_, err := NewServer(appconfig.LogWebConfig{Enable: false}, status.NewRuntimeStore())
	if err == nil {
		t.Fatal("expected disabled error")
	}
}

func TestServerHealthz(t *testing.T) {
	server := newTestServer(t, "")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected body %+v", body)
	}
}

func TestServerStatusRequiresBearerToken(t *testing.T) {
	server := newTestServer(t, "secret")

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerStatusReturnsRuntimeSnapshot(t *testing.T) {
	store := status.NewRuntimeStore()
	store.RecordProcessed("edge-upload", "evt-001", "forwarded", 0)
	server, err := NewServer(appconfig.LogWebConfig{
		Enable: true,
		Bind:   "127.0.0.1",
		Port:   18080,
	}, store)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var snapshot status.RuntimeSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(snapshot.Workers) != 1 || snapshot.Workers[0].LastEventID != "evt-001" {
		t.Fatalf("unexpected snapshot %+v", snapshot)
	}
}

func TestServerLogsReturnsRuntimeLogs(t *testing.T) {
	store := status.NewRuntimeStore()
	store.RecordProcessed("server-ingress", "evt-001", "applied", 2)
	server, err := NewServer(appconfig.LogWebConfig{
		Enable: true,
		Bind:   "127.0.0.1",
		Port:   18080,
	}, store)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var logs []status.LogEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(logs) != 1 || logs[0].EventID != "evt-001" {
		t.Fatalf("unexpected logs %+v", logs)
	}
}

func newTestServer(t *testing.T, token string) *Server {
	t.Helper()
	server, err := NewServer(appconfig.LogWebConfig{
		Enable: true,
		Bind:   "127.0.0.1",
		Port:   18080,
		Token:  token,
	}, status.NewRuntimeStore())
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return server
}
