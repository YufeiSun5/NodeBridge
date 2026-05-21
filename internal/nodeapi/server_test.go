package nodeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

func TestRegisterUpsertsNodeAndEnsuresQueue(t *testing.T) {
	store := &fakeStore{}
	topology := &fakeTopology{}
	server := NewServer(store, topology, nil)

	body := bytes.NewBufferString(`{"node_id":"edge-001","node_name":"Edge 1","location":"line-a","version":"0.17.0"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/nodes/register", body)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d body=%s", rr.Code, rr.Body.String())
	}
	if len(store.nodes) != 1 || store.nodes[0].NodeID != "edge-001" || store.nodes[0].Status != syncstore.StatusActive {
		t.Fatalf("unexpected registered nodes %+v", store.nodes)
	}
	if len(topology.nodes) != 1 || topology.nodes[0] != "edge-001" {
		t.Fatalf("expected topology ensure, got %+v", topology.nodes)
	}
}

func TestPutConfigStoresAndPublishes(t *testing.T) {
	store := &fakeStore{}
	publisher := &fakePublisher{}
	server := NewServer(store, nil, publisher)

	body := bytes.NewBufferString(`{"mysql_host":"127.0.0.1","mysql_port":3307,"mysql_database":"scada_edge","mysql_username":"sync_user","cdc_type":"canal","rule_version":7}`)
	req := httptest.NewRequest(http.MethodPut, "/api/nodes/edge-001/config", body)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d body=%s", rr.Code, rr.Body.String())
	}
	if len(store.configs) != 1 || store.configs[0].NodeID != "edge-001" || store.configs[0].MySQLHost != "127.0.0.1" {
		t.Fatalf("unexpected configs %+v", store.configs)
	}
	if len(publisher.configs) != 1 || publisher.configs[0].NodeID != "edge-001" {
		t.Fatalf("expected config publish, got %+v", publisher.configs)
	}
}

func TestGetConfigReturnsNonSecretConfig(t *testing.T) {
	store := &fakeStore{config: syncstore.NodeConfig{NodeID: "edge-001", MySQLUsername: "sync_user", RuleVersion: 3}}
	server := NewServer(store, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/nodes/edge-001/config", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d body=%s", rr.Code, rr.Body.String())
	}
	var cfg syncstore.NodeConfig
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if cfg.NodeID != "edge-001" || cfg.MySQLUsername != "sync_user" || cfg.RuleVersion != 3 {
		t.Fatalf("unexpected config %+v", cfg)
	}
}

type fakeStore struct {
	nodes   []syncstore.NodeRecord
	configs []syncstore.NodeConfig
	config  syncstore.NodeConfig
}

func (s *fakeStore) UpsertNode(ctx context.Context, record syncstore.NodeRecord) error {
	s.nodes = append(s.nodes, record)
	return nil
}

func (s *fakeStore) ListNodes(ctx context.Context) ([]syncstore.NodeRecord, error) {
	return s.nodes, nil
}

func (s *fakeStore) GetNodeConfig(ctx context.Context, nodeID string) (syncstore.NodeConfig, error) {
	if s.config.NodeID != "" {
		return s.config, nil
	}
	return syncstore.NodeConfig{NodeID: nodeID}, nil
}

func (s *fakeStore) UpsertNodeConfig(ctx context.Context, config syncstore.NodeConfig) error {
	s.configs = append(s.configs, config)
	return nil
}

type fakeTopology struct {
	nodes []string
}

func (t *fakeTopology) EnsureNode(ctx context.Context, nodeID string) error {
	t.nodes = append(t.nodes, nodeID)
	return nil
}

type fakePublisher struct {
	configs []syncstore.NodeConfig
}

func (p *fakePublisher) PublishConfig(ctx context.Context, config syncstore.NodeConfig) error {
	p.configs = append(p.configs, config)
	return nil
}
