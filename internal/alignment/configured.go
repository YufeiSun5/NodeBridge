package alignment

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/agentstate"
	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/go-sql-driver/mysql"
	"github.com/rabbitmq/amqp091-go"
)

type ConfiguredRequest struct {
	RuleID  string `json:"rule_id"`
	PeerID  string `json:"peer_node_id"`
	Confirm bool   `json:"confirm"`
}

// RunConfigured uses only the saved local credentials and the shared broker.
// The existing Agent lock also excludes a second UI/CLI alignment operation.
func RunConfigured(ctx context.Context, configPath, rulesPath string, request ConfiguredRequest, progress func(string)) (CutoverProof, error) {
	var proof CutoverProof
	if !request.Confirm {
		return proof, errors.New("alignment_confirmation_required")
	}
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return proof, err
	}
	rulesPath, err = filepath.Abs(rulesPath)
	if err != nil {
		return proof, err
	}
	lock, err := agentstate.Lock(configPath + ".agent.lock")
	if err != nil {
		return proof, fmt.Errorf("alignment_requires_stopped_agent: %w", err)
	}
	defer lock.Close()
	if err := CheckRebaselinePreparation(configPath); err != nil {
		return proof, err
	}
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return proof, err
	}
	configRevision := sha256.Sum256(configBytes)
	cfg, err := appconfig.LoadFile(configPath)
	if err != nil {
		return proof, err
	}
	if cfg.Mode != appconfig.ModeEdge && cfg.Mode != appconfig.ModeServer {
		return proof, errors.New("alignment_edge_or_server_required")
	}
	peers := ParsePeerIDs(request.PeerID)
	if len(peers) == 0 || len(peers) > 256 || cfg.Mode == appconfig.ModeEdge && len(peers) != 1 {
		return proof, errors.New("alignment_peer_count_invalid")
	}
	seen := map[string]bool{}
	for _, peer := range peers {
		if seen[peer] || cfg.Node.ID == peer || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(peer) {
			return proof, errors.New("alignment_peer_id_invalid")
		}
		seen[peer] = true
	}
	if !strings.EqualFold(cfg.CDC.Type, "canal") || cfg.RabbitMQ.ServerURL == "" {
		return proof, errors.New("alignment_canal_and_shared_broker_required")
	}
	set, revision, err := rules.LoadFileWithRevision(rulesPath)
	if err != nil {
		return proof, err
	}
	var selected *rules.SyncRule
	for i := range set.Rules {
		if set.Rules[i].ID == request.RuleID {
			selected = &set.Rules[i]
		}
	}
	if selected == nil {
		return proof, errors.New("alignment_rule_not_found")
	}
	if selected.InitialAlignment.EffectivePolicy() != rules.AlignmentManual {
		return proof, errors.New("alignment_manual_policy_required")
	}
	timeout := SnapshotTimeout
	if selected.Direction == rules.DirectionBidirectional {
		timeout = TopologyTimeout
	} else if len(peers) != 1 {
		return proof, errors.New("alignment_multiple_peers_require_bidirectional")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dsn, err := mysql.ParseDSN(mysqlconn.DSN(cfg.MySQL))
	if err != nil {
		return proof, err
	}
	dsn.Timeout, dsn.ReadTimeout, dsn.WriteTimeout = 5*time.Second, 30*time.Second, 30*time.Second
	db, err := mysqlconn.OpenDSN(dsn.FormatDSN())
	if err != nil {
		return proof, err
	}
	defer db.Close()
	executable, err := os.Executable()
	if err != nil {
		return proof, err
	}
	migrations := filepath.Join(filepath.Dir(executable), "migrations", cfg.Mode)
	if _, err := os.Stat(migrations); errors.Is(err, os.ErrNotExist) {
		migrations = filepath.Join("migrations", cfg.Mode)
	}
	if err := mysqlconn.RunMigrations(ctx, db, migrations); err != nil {
		return proof, err
	}
	conn, err := amqp091.DialConfig(cfg.RabbitMQ.ServerURL, amqp091.Config{Heartbeat: 10 * time.Second, Dial: amqp091.DefaultDial(5 * time.Second)})
	if err != nil {
		return proof, err
	}
	defer conn.Close()
	filter := cfg.CDC.Filter
	markerFilter := regexp.QuoteMeta(cfg.MySQL.Database) + `\.sync_capture_fence`
	if filter == "" {
		filter = `.*\..*`
	} else {
		filter += "," + markerFilter
	}
	readerName := cfg.CDC.ReaderName
	if readerName == "" {
		readerName = cfg.Node.ID
	}
	canalConfig := canal.Config{ReaderName: readerName, Address: cfg.CDC.CanalAddr, Destination: cfg.CDC.Destination, Username: cfg.CDC.Username, Password: cfg.CDC.Password, Filter: filter, BatchSize: cfg.CDC.BatchSize}
	if selected.Direction == rules.DirectionBidirectional {
		var previous []rulecheck.ObservedPair
		previous, err = rulecheck.ReadPairManifest(rulesPath + ".pairs.json")
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return proof, err
		}
		group := configuredTopology{cfg: cfg, set: set, db: db, broker: conn, canal: canalConfig,
			configPath: configPath, rulesPath: rulesPath, configRevision: configRevision, rulesRevision: revision, previous: previous, progress: progress}
		if cfg.Mode == appconfig.ModeEdge {
			return group.runEdge(ctx, *selected, peers[0])
		}
		intent, err := configuredTopologyIntent(cfg.Node.ID, *set, *selected, peers)
		if err != nil {
			return proof, err
		}
		return group.runServer(ctx, intent)
	}
	return RunPairSession(ctx, PairSessionOptions{
		NodeID: cfg.Node.ID, PeerID: peers[0], IsEdge: cfg.Mode == appconfig.ModeEdge,
		Rule: *selected, DB: db, Broker: conn, Confirm: true, Progress: progress,
		Canal: canalConfig,
		Ready: func(ctx context.Context, proof CutoverProof) error {
			current, err := os.ReadFile(configPath)
			if err != nil || sha256.Sum256(current) != configRevision {
				return errors.Join(errors.New("alignment_config_changed"), err)
			}
			if !proof.MatchesRule(*selected) {
				return errors.New("alignment_rule_changed")
			}
			var pairs []rulecheck.ObservedPair
			if previous, err := rulecheck.ReadPairManifest(rulesPath + ".pairs.json"); err == nil {
				pairs = previous
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			selected.Enable = true
			if selected.Direction == rules.DirectionBidirectional {
				left, right := proof.Plan.Source, proof.Plan.Target
				if proof.Plan.Direction == ToLeft {
					left, right = right, left
				}
				pair := rulecheck.ObservedPair{Rule: *selected, EdgeNode: left.NodeID, ServerNode: right.NodeID, Edge: left.Schema, Server: right.Schema}
				replaced := false
				for i := range pairs {
					if pairs[i].Rule.ID == selected.ID {
						pairs[i] = pair
						replaced = true
						continue
					}
					if pairs[i].Server.Database == pair.Server.Database && pairs[i].Server.Table == pair.Server.Table {
						return errors.New("alignment_pair_only: table already belongs to another pair")
					}
				}
				if !replaced {
					pairs = append(pairs, pair)
				}
				if _, err := rulecheck.CompileEndpoint(cfg.Node.ID, cfg.Mode, *set, pairs); err != nil {
					return err
				}
				if err := rulecheck.VerifyLocalObservations(ctx, db, cfg.Node.ID, pairs); err != nil {
					return err
				}
				if err := rulecheck.WritePairManifest(rulesPath+".pairs.json", pairs); err != nil {
					return err
				}
			}
			for i := range set.Rules {
				if set.Rules[i].Enable && set.Rules[i].Direction == rules.DirectionBidirectional {
					set.Rules[i] = set.Rules[i].BindPairedRuntime()
				}
			}
			_, err = rules.SaveFileCAS(rulesPath, *set, revision)
			return err
		},
	})
}
