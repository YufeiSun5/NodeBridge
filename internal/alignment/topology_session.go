package alignment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
	"github.com/YufeiSun5/NodeBridge/internal/rulecheck"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/rabbitmq/amqp091-go"
)

const TopologyTimeout = time.Hour

func ParsePeerIDs(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' })
}

func configuredTopologyIntent(node string, set rules.RuleSet, selected rules.SyncRule, peers []string) (TopologyIntent, error) {
	database, table := selected.TargetDatabaseName, selected.TargetTableName
	if database == "" {
		database = selected.DatabaseName
	}
	if table == "" {
		table = selected.TableName
	}
	intent := TopologyIntent{ServerNode: node, ServerDatabase: database, ServerTable: table}
	for _, peer := range peers {
		var candidates []rules.SyncRule
		for _, rule := range set.Rules {
			db, name := rule.TargetDatabaseName, rule.TargetTableName
			if db == "" {
				db = rule.DatabaseName
			}
			if name == "" {
				name = rule.TableName
			}
			if rule.Direction == rules.DirectionBidirectional && db == database && name == table && (len(rule.SourceNodeIDs) == 0 || slices.Contains(rule.SourceNodeIDs, peer)) {
				candidates = append(candidates, rule)
			}
		}
		if len(candidates) != 1 {
			return intent, fmt.Errorf("alignment_topology_peer_rule_ambiguous: %s", peer)
		}
		intent.Members = append(intent.Members, TopologyMember{EdgeNode: peer, Rule: candidates[0]})
	}
	return sealTopologyIntent(intent)
}

func checkTopologyExtension(previous []rulecheck.ObservedPair, intent TopologyIntent) error {
	for _, old := range previous {
		if old.ServerNode != intent.ServerNode || old.Server.Database != intent.ServerDatabase || old.Server.Table != intent.ServerTable {
			continue
		}
		found := false
		for _, member := range intent.Members {
			if old.EdgeNode == member.EdgeNode && hash(canonicalCutoverRule(old.Rule)) == hash(member.Rule) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("alignment_topology_existing_member_must_participate: %s", old.EdgeNode)
		}
	}
	return nil
}

type configuredTopology struct {
	cfg            *appconfig.Config
	set            *rules.RuleSet
	db             *sql.DB
	broker         *amqp091.Connection
	canal          canal.Config
	configPath     string
	rulesPath      string
	configRevision [32]byte
	rulesRevision  string
	previous       []rulecheck.ObservedPair
	progress       func(string)
}

func (c *configuredTopology) save(ctx context.Context, intent TopologyIntent, topology TopologyProof) (TopologyReceipt, error) {
	if err := intent.validateProof(topology); err != nil {
		return TopologyReceipt{}, err
	}
	current, err := os.ReadFile(c.configPath)
	if err != nil || sha256.Sum256(current) != c.configRevision {
		return TopologyReceipt{}, errors.Join(errors.New("alignment_config_changed"), err)
	}
	if err := checkTopologyExtension(c.previous, intent); err != nil {
		return TopologyReceipt{}, err
	}
	updated := rules.RuleSet{Rules: slices.Clone(c.set.Rules)}
	for _, member := range intent.Members {
		if c.cfg.Node.ID != intent.ServerNode && c.cfg.Node.ID != member.EdgeNode {
			continue
		}
		found := false
		for i, rule := range updated.Rules {
			if rule.ID == member.Rule.ID && hash(canonicalCutoverRule(rule)) == hash(member.Rule) {
				updated.Rules[i].Enable = true
				found = true
			}
		}
		if !found {
			return TopologyReceipt{}, errors.New("alignment_topology_local_rule_changed")
		}
	}
	pairs := make([]rulecheck.ObservedPair, 0, len(c.previous)+len(topology.Proofs))
	for _, previous := range c.previous {
		if previous.ServerNode != intent.ServerNode || previous.Server.Database != intent.ServerDatabase || previous.Server.Table != intent.ServerTable {
			pairs = append(pairs, previous)
		}
	}
	for _, observed := range topology.Observations() {
		observed.Rule.Enable = true
		pairs = append(pairs, observed)
	}
	if _, err := rulecheck.CompileEndpoint(c.cfg.Node.ID, c.cfg.Mode, updated, pairs); err != nil {
		return TopologyReceipt{}, err
	}
	if err := rulecheck.VerifyLocalObservations(ctx, c.db, c.cfg.Node.ID, pairs); err != nil {
		return TopologyReceipt{}, err
	}
	if err := rulecheck.WritePairManifest(c.rulesPath+".pairs.json", pairs); err != nil {
		return TopologyReceipt{}, err
	}
	for i, rule := range updated.Rules {
		if rule.Enable && rule.Direction == rules.DirectionBidirectional {
			updated.Rules[i] = rule.BindPairedRuntime()
		}
	}
	if _, err := rules.SaveFileCAS(c.rulesPath, updated, c.rulesRevision); err != nil {
		return TopologyReceipt{}, err
	}
	return readyTopology(ctx, c.db, c.cfg.Node.ID, intent, topology)
}

func (c *configuredTopology) pairOptions(member TopologyMember, peer string) PairSessionOptions {
	return PairSessionOptions{NodeID: c.cfg.Node.ID, PeerID: peer, IsEdge: c.cfg.Mode == appconfig.ModeEdge,
		Rule: member.Rule, DB: c.db, Broker: c.broker, Canal: c.canal, Confirm: true, Timeout: TopologyTimeout,
		Ready: func(context.Context, CutoverProof) error { return nil }, Progress: c.progress}
}

func (c *configuredTopology) runEdge(ctx context.Context, selected rules.SyncRule, server string) (CutoverProof, error) {
	var intent TopologyIntent
	options := c.pairOptions(TopologyMember{EdgeNode: c.cfg.Node.ID, Rule: selected}, server)
	options.BeforePlan = func(ctx context.Context, control *PairControl, _, _ Observation) error {
		if err := control.Exchange(ctx, "topology_intent", TopologyIntent{}, &intent); err != nil {
			return err
		}
		if err := intent.Validate(); err != nil {
			return err
		}
		found := false
		for _, member := range intent.Members {
			found = found || member.EdgeNode == c.cfg.Node.ID && hash(member.Rule) == hash(canonicalCutoverRule(selected))
		}
		if intent.ServerNode != server || !found {
			return errors.New("alignment_topology_unexpected_member")
		}
		if err := checkTopologyExtension(c.previous, intent); err != nil {
			return err
		}
		if err := prepareTopology(ctx, c.db, c.cfg.Node.ID, intent); err != nil {
			return err
		}
		var allowed bool
		if err := control.Exchange(ctx, "copy_turn", true, &allowed); err != nil {
			return err
		}
		if !allowed {
			return errors.New("alignment_topology_copy_not_scheduled")
		}
		return nil
	}
	options.AfterActive = func(ctx context.Context, control *PairControl, local CutoverProof) error {
		var topology TopologyProof
		if err := control.Exchange(ctx, "topology", local.ID, &topology); err != nil {
			return err
		}
		if err := intent.validateProof(topology); err != nil {
			return err
		}
		found := false
		for _, proof := range topology.Proofs {
			found = found || proof.ID == local.ID
		}
		if !found {
			return errors.New("alignment_topology_local_proof_missing")
		}
		receipt, err := c.save(ctx, intent, topology)
		if err != nil {
			return err
		}
		var serverReceipt TopologyReceipt
		if err := control.Exchange(ctx, "topology_ready", receipt, &serverReceipt); err != nil {
			return err
		}
		if serverReceipt.NodeID != intent.ServerNode || serverReceipt.TopologyID != topology.ID || serverReceipt.IntentID != intent.ID {
			return errors.New("alignment_topology_server_receipt_invalid")
		}
		var receipts []TopologyReceipt
		// Exchange acknowledges delivery, not activation. A distinct final stage
		// is sent only after this node durably activates the complete certificate.
		if err := control.Exchange(ctx, "topology_certificate", receipt, &receipts); err != nil {
			return err
		}
		if err := activateTopology(ctx, c.db, c.cfg.Node.ID, intent, topology, receipts); err != nil {
			return err
		}
		receipt.Phase = CutoverActive
		return control.Exchange(ctx, "topology_active", receipt, &serverReceipt)
	}
	return RunPairSession(ctx, options)
}

type topologyObserved struct {
	member int
	local  Observation
	peer   Observation
}

type topologyCopied struct {
	member int
	proof  CutoverProof
}

func (c *configuredTopology) runServer(ctx context.Context, intent TopologyIntent) (CutoverProof, error) {
	if err := checkTopologyExtension(c.previous, intent); err != nil {
		return CutoverProof{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup
	var firstFailure error
	var failureOnce sync.Once
	start := func(run func() error) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := run(); err != nil {
				failureOnce.Do(func() { firstFailure = err; cancel() })
			}
		}()
	}
	observed := make(chan topologyObserved, len(intent.Members))
	copied := make(chan topologyCopied, len(intent.Members))
	ready := make(chan TopologyReceipt, len(intent.Members))
	turns := make([]chan struct{}, len(intent.Members))
	topologyReady, activateReady := make(chan struct{}), make(chan struct{})
	var topology TopologyProof
	var serverReceipt TopologyReceipt
	var certificates []TopologyReceipt
	for i, member := range intent.Members {
		turns[i] = make(chan struct{})
		start(func() error {
			broker, err := amqp091.DialConfig(c.cfg.RabbitMQ.ServerURL, amqp091.Config{Heartbeat: 10 * time.Second, Dial: amqp091.DefaultDial(5 * time.Second)})
			if err != nil {
				return err
			}
			defer broker.Close()
			options := c.pairOptions(member, member.EdgeNode)
			options.Broker = broker
			options.BeforePlan = func(ctx context.Context, control *PairControl, local, peer Observation) error {
				var acknowledgement TopologyIntent
				if err := control.Exchange(ctx, "topology_intent", intent, &acknowledgement); err != nil {
					return err
				}
				if err := prepareTopology(ctx, c.db, c.cfg.Node.ID, intent); err != nil {
					return err
				}
				select {
				case observed <- topologyObserved{i, local, peer}:
				case <-ctx.Done():
					return ctx.Err()
				}
				select {
				case <-turns[i]:
				case <-ctx.Done():
					return ctx.Err()
				}
				var allowed bool
				if err := control.Exchange(ctx, "copy_turn", true, &allowed); err != nil {
					return err
				}
				if !allowed {
					return errors.New("alignment_topology_copy_not_scheduled")
				}
				return nil
			}
			options.AfterActive = func(ctx context.Context, control *PairControl, proof CutoverProof) error {
				select {
				case copied <- topologyCopied{i, proof}:
				case <-ctx.Done():
					return ctx.Err()
				}
				select {
				case <-topologyReady:
				case <-ctx.Done():
					return ctx.Err()
				}
				var localProofID string
				if err := control.Exchange(ctx, "topology", topology, &localProofID); err != nil {
					return err
				}
				if localProofID != proof.ID {
					return errors.New("alignment_topology_peer_proof_changed")
				}
				var receipt TopologyReceipt
				if err := control.Exchange(ctx, "topology_ready", serverReceipt, &receipt); err != nil {
					return err
				}
				if receipt.NodeID != member.EdgeNode || receipt.TopologyID != topology.ID || receipt.IntentID != intent.ID || receipt.Phase != CutoverReady && receipt.Phase != CutoverActive {
					return errors.New("alignment_topology_peer_receipt_invalid")
				}
				select {
				case ready <- receipt:
				case <-ctx.Done():
					return ctx.Err()
				}
				select {
				case <-activateReady:
				case <-ctx.Done():
					return ctx.Err()
				}
				if err := control.Exchange(ctx, "topology_certificate", certificates, &receipt); err != nil {
					return err
				}
				active := serverReceipt
				active.Phase = CutoverActive
				if err := control.Exchange(ctx, "topology_active", active, &receipt); err != nil {
					return err
				}
				if receipt.NodeID != member.EdgeNode || receipt.TopologyID != topology.ID || receipt.IntentID != intent.ID || receipt.Phase != CutoverActive {
					return errors.New("alignment_topology_peer_activation_invalid")
				}
				return nil
			}
			_, err = RunPairSession(ctx, options)
			return err
		})
	}
	coordinate := func() error {
		var states []topologyObserved
		for range intent.Members {
			select {
			case state := <-observed:
				states = append(states, state)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		serverHasRows := states[0].local.HasRows
		populated := 0
		for _, state := range states {
			if state.local.HasRows != serverHasRows {
				return errors.New("alignment_topology_server_emptiness_changed")
			}
			if state.peer.HasRows {
				populated++
			}
		}
		if !serverHasRows && populated > 1 {
			return errors.New("alignment_topology_multiple_nonempty_sources: merge is not authorized")
		}
		sort.Slice(states, func(i, j int) bool {
			if !serverHasRows && states[i].peer.HasRows != states[j].peer.HasRows {
				return states[i].peer.HasRows
			}
			return states[i].member < states[j].member
		})
		var proofs []CutoverProof
		for _, state := range states {
			close(turns[state.member])
			select {
			case result := <-copied:
				if result.member != state.member {
					return errors.New("alignment_topology_copy_order_changed")
				}
				proofs = append(proofs, result.proof)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		var err error
		topology, err = BuildTopologyProof(proofs)
		if err != nil {
			return err
		}
		serverReceipt, err = c.save(ctx, intent, topology)
		if err != nil {
			return err
		}
		close(topologyReady)
		certificates = []TopologyReceipt{serverReceipt}
		for range intent.Members {
			select {
			case receipt := <-ready:
				certificates = append(certificates, receipt)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err := activateTopology(ctx, c.db, c.cfg.Node.ID, intent, topology, certificates); err != nil {
			return err
		}
		close(activateReady)
		return nil
	}
	if err := coordinate(); err != nil {
		cancel()
		workers.Wait()
		return CutoverProof{}, errors.Join(err, firstFailure)
	}
	workers.Wait()
	if firstFailure != nil {
		return CutoverProof{}, firstFailure
	}
	return topology.Proofs[0], nil
}
