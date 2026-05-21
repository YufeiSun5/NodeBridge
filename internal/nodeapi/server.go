package nodeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/event"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

type Store interface {
	UpsertNode(ctx context.Context, record syncstore.NodeRecord) error
	ListNodes(ctx context.Context) ([]syncstore.NodeRecord, error)
	GetNodeConfig(ctx context.Context, nodeID string) (syncstore.NodeConfig, error)
	UpsertNodeConfig(ctx context.Context, config syncstore.NodeConfig) error
}

type TopologyInitializer interface {
	EnsureNode(ctx context.Context, nodeID string) error
}

type ConfigPublisher interface {
	PublishConfig(ctx context.Context, config syncstore.NodeConfig) error
}

type Server struct {
	store     Store
	topology  TopologyInitializer
	publisher ConfigPublisher
	mux       *http.ServeMux
}

func NewServer(store Store, topology TopologyInitializer, publisher ConfigPublisher) *Server {
	s := &Server{store: store, topology: topology, publisher: publisher, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/nodes/register", s.handleRegister)
	s.mux.HandleFunc("/api/nodes", s.handleNodes)
	s.mux.HandleFunc("/api/nodes/", s.handleNodePath)
}

type RegisterRequest struct {
	NodeID      string `json:"node_id"`
	NodeName    string `json:"node_name"`
	Location    string `json:"location,omitempty"`
	MachineCode string `json:"machine_code,omitempty"`
	Version     string `json:"version,omitempty"`
}

type RegisterResponse struct {
	Status        string `json:"status"`
	DownlinkQueue string `json:"downlink_queue"`
	ConfigVersion int64  `json:"config_version"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.NodeID == "" || req.NodeName == "" {
		writeError(w, http.StatusBadRequest, "node_id and node_name are required")
		return
	}
	record := syncstore.NodeRecord{
		NodeID:   req.NodeID,
		NodeName: req.NodeName,
		NodeType: "edge",
		Location: req.Location,
		Status:   syncstore.StatusActive,
		Version:  req.Version,
	}
	if err := s.store.UpsertNode(r.Context(), record); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.topology != nil {
		if err := s.topology.EnsureNode(r.Context(), req.NodeID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	version := int64(0)
	if cfg, err := s.store.GetNodeConfig(r.Context(), req.NodeID); err == nil {
		version = cfg.RuleVersion
	}
	writeJSON(w, http.StatusOK, RegisterResponse{Status: syncstore.StatusActive, DownlinkQueue: req.NodeID + ".downlink.q", ConfigVersion: version})
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	nodes, err := s.store.ListNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleNodePath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/nodes/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "config" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	nodeID := parts[0]
	switch r.Method {
	case http.MethodGet:
		cfg, err := s.store.GetNodeConfig(r.Context(), nodeID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPut:
		var cfg syncstore.NodeConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		cfg.NodeID = nodeID
		if err := s.store.UpsertNodeConfig(r.Context(), cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if s.publisher != nil {
			if err := s.publisher.PublishConfig(r.Context(), cfg); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, cfg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type AMQPNodeTopology struct {
	Channel rabbitmq.Declarer
}

func (t AMQPNodeTopology) EnsureNode(ctx context.Context, nodeID string) error {
	_ = ctx
	return rabbitmq.InitializeTopology(t.Channel, rabbitmq.ServerTopology([]string{nodeID}))
}

type AMQPConfigPublisher struct {
	Publisher interface {
		Publish(context.Context, rabbitmq.PublishRequest) error
	}
	Exchange     string
	SourceNodeID string
}

func (p AMQPConfigPublisher) PublishConfig(ctx context.Context, config syncstore.NodeConfig) error {
	body, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("map config: %w", err)
	}
	evt := event.SyncEvent{
		EventID:      fmt.Sprintf("cfg-%s-%d", config.NodeID, time.Now().UnixNano()),
		EventType:    event.TypeConfigUpdate,
		OriginNodeID: p.SourceNodeID,
		SourceNodeID: p.SourceNodeID,
		TargetNodeID: config.NodeID,
		DatabaseName: "nodebridge",
		TableName:    "sync_node_config",
		PrimaryKey:   map[string]any{"node_id": config.NodeID},
		After:        payload,
		CreatedAt:    time.Now(),
		EventTime:    time.Now(),
		TraceID:      fmt.Sprintf("config-%s", config.NodeID),
	}
	encoded, err := rabbitmq.EncodeJSON(evt)
	if err != nil {
		return err
	}
	exchange := p.Exchange
	if exchange == "" {
		exchange = "server.dispatch.x"
	}
	return p.Publisher.Publish(ctx, rabbitmq.PublishRequest{
		Exchange:   exchange,
		RoutingKey: config.NodeID + ".downlink",
		Body:       encoded,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
