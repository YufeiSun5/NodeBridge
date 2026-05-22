package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/YufeiSun5/NodeBridge/internal/appconfig"
	"github.com/YufeiSun5/NodeBridge/internal/apply"
	"github.com/YufeiSun5/NodeBridge/internal/cdc"
	canalcdc "github.com/YufeiSun5/NodeBridge/internal/cdc/canal"
	"github.com/YufeiSun5/NodeBridge/internal/event"
	installerassets "github.com/YufeiSun5/NodeBridge/internal/installer/assets"
	installerexec "github.com/YufeiSun5/NodeBridge/internal/installer/executor"
	"github.com/YufeiSun5/NodeBridge/internal/logweb"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mcpstdio"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
	"github.com/YufeiSun5/NodeBridge/internal/nodeapi"
	"github.com/YufeiSun5/NodeBridge/internal/normalizer"
	"github.com/YufeiSun5/NodeBridge/internal/rabbitmq"
	"github.com/YufeiSun5/NodeBridge/internal/rules"
	"github.com/YufeiSun5/NodeBridge/internal/status"
	"github.com/YufeiSun5/NodeBridge/internal/syncruntime"
	"github.com/YufeiSun5/NodeBridge/internal/syncstore"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "migrate":
			return runMigrate(args[1:], stdout, stderr)
		case "apply-event":
			return runApplyEvent(args[1:], stdout, stderr)
		case "init-rabbitmq":
			return runInitRabbitMQ(args[1:], stdout, stderr)
		case "publish-event":
			return runPublishEvent(args[1:], stdout, stderr)
		case "publish-stress-batch":
			return runPublishStressBatch(args[1:], stdout, stderr)
		case "publish-change-once":
			return runPublishChangeOnce(args[1:], stdout, stderr)
		case "canal-check":
			return runCanalCheck(args[1:], stdout, stderr)
		case "canal-publish-once":
			return runCanalPublishOnce(args[1:], stdout, stderr)
		case "server-cdc-dispatch-once":
			return runServerCDCDispatchOnce(args[1:], stdout, stderr)
		case "server-canal-dispatch-once":
			return runServerCanalDispatchOnce(args[1:], stdout, stderr)
		case "consume-once":
			return runConsumeOnce(args[1:], stdout, stderr)
		case "consume-batch-once":
			return runConsumeBatchOnce(args[1:], stdout, stderr)
		case "forward-upload-once":
			return runForwardUploadOnce(args[1:], stdout, stderr)
		case "forward-upload-batch-once":
			return runForwardUploadBatchOnce(args[1:], stdout, stderr)
		case "consume-downlink-once":
			return runConsumeDownlinkOnce(args[1:], stdout, stderr)
		case "consume-downlink-batch-once":
			return runConsumeDownlinkBatchOnce(args[1:], stdout, stderr)
		case "failed-events":
			return runFailedEvents(args[1:], stdout, stderr)
		case "retry-event":
			return runRetryEvent(args[1:], stdout, stderr)
		case "retry-failed-batch":
			return runRetryFailedBatch(args[1:], stdout, stderr)
		case "dead-letters":
			return runDeadLetters(args[1:], stdout, stderr)
		case "managed-plan":
			return runManagedPlan(args[1:], stdout, stderr)
		case "managed-apply":
			return runManagedApply(args[1:], stdout, stderr)
		case "managed-repair":
			return runManagedRepair(args[1:], stdout, stderr)
		case "managed-uninstall":
			return runManagedUninstall(args[1:], stdout, stderr)
		case "installer-assets-check":
			return runInstallerAssetsCheck(args[1:], stdout, stderr)
		case "installer-command-plan":
			return runInstallerCommandPlan(args[1:], stdout, stderr)
		case "mcp-stdio":
			return runMCPStdio(args[1:], stdout, stderr)
		case "replay-pending-once":
			return runReplayPendingOnce(args[1:], stdout, stderr)
		case "dispatch-event-once":
			return runDispatchEventOnce(args[1:], stdout, stderr)
		case "serve-log-web":
			return runServeLogWeb(args[1:], stdout, stderr)
		case "serve-node-api":
			return runServeNodeAPI(args[1:], stdout, stderr)
		case "register-node":
			return runRegisterNode(args[1:], stdout, stderr)
		case "set-node-config":
			return runSetNodeConfig(args[1:], stdout, stderr)
		case "list-nodes":
			return runListNodes(args[1:], stdout, stderr)
		case "list-node-config":
			return runListNodeConfig(args[1:], stdout, stderr)
		case "run":
			return runAgent(args[1:], stdout, stderr)
		}
	}
	return runReady(args, stdout, stderr)
}

func runAgent(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	edges := flags.String("edges", "", "comma-separated edge node ids for server dispatch")
	maxSteps := flags.Int("max-steps", 0, "maximum worker steps before exit; 0 means run forever")
	stopFile := flags.String("stop-file", "", "path watched for graceful shutdown request")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()
	if *stopFile != "" {
		go watchStopFile(ctx, *stopFile, 500*time.Millisecond, cancel, stdout)
	}

	store := status.NewRuntimeStore()
	shutdownLogWeb, err := startLogWeb(ctx, cfg.LogWeb, store, stdout, stderr)
	if err != nil {
		return err
	}
	defer shutdownLogWeb(context.Background())

	fmt.Fprintf(stdout, "sync-agent running mode=%s node_id=%s\n", cfg.Mode, cfg.Node.ID)
	switch cfg.Mode {
	case appconfig.ModeEdge:
		return runEdgeWorkers(ctx, cfg, ruleSet, store, *maxSteps, stdout, stderr)
	case appconfig.ModeServer:
		return runServerWorkers(ctx, cfg, ruleSet, splitCSV(*edges), store, *maxSteps, stdout, stderr)
	default:
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}
}

func watchStopFile(ctx context.Context, path string, interval time.Duration, cancel context.CancelFunc, stdout io.Writer) {
	if path == "" {
		return
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				fmt.Fprintf(stdout, "sync-agent stop requested file=%s\n", path)
				cancel()
				return
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(stdout, "sync-agent stop file check failed file=%s error=%v\n", path, err)
			}
		}
	}
}

func runEdgeWorkers(ctx context.Context, cfg *appconfig.Config, ruleSet *rules.RuleSet, store *status.RuntimeStore, maxSteps int, stdout, stderr io.Writer) error {
	if cfg.RabbitMQ.LocalURL == "" {
		return fmt.Errorf("rabbitmq.local_url is required for edge run")
	}
	if cfg.RabbitMQ.ServerURL == "" {
		return fmt.Errorf("rabbitmq.server_url is required for edge run")
	}

	localConn, err := rabbitmq.Dial(cfg.RabbitMQ.LocalURL)
	if err != nil {
		fmt.Fprintf(stderr, "local rabbitmq connect failed: %v\n", err)
		return err
	}
	defer localConn.Close()
	serverPublishConn, err := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
	if err != nil {
		fmt.Fprintf(stderr, "server rabbitmq connect failed: %v\n", err)
		return err
	}
	defer serverPublishConn.Close()
	// Downlink stays on Server RabbitMQ. / 下发留在 Server RabbitMQ。 / Downlink は Server RabbitMQ。
	serverDownlinkConn, err := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
	if err != nil {
		fmt.Fprintf(stderr, "server downlink rabbitmq connect failed: %v\n", err)
		return err
	}
	defer serverDownlinkConn.Close()

	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	publisher, err := rabbitmq.NewPublisher(serverPublishConn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	workers := []syncruntime.Worker{
		{
			Config: workerConfig("edge-upload", cfg.Sync.RetryIntervalSeconds, maxSteps),
			Stepper: syncruntime.EdgeUploadBatchRuntime{
				Source: syncruntime.AMQPBatchGetSource{
					Channel: localConn.Channel,
					Queue:   "edge.upload.cdc.q",
				},
				Publisher:     publisher,
				Consumer:      rabbitmq.Consumer{RequeueOnError: true},
				Exchange:      "server.ingress.x",
				RoutingKey:    "server.ingress",
				MaxBatch:      syncBatchSize(cfg.Sync.UploadBatchSize),
				FlushInterval: syncFlushInterval(cfg.Sync.FlushIntervalMillis),
			},
			Status: store,
		},
		{
			Config: workerConfig("edge-downlink", cfg.Sync.RetryIntervalSeconds, maxSteps),
			Stepper: syncruntime.EdgeDownlinkBatchRuntime{
				Source: syncruntime.AMQPBatchGetSource{
					Channel: serverDownlinkConn.Channel,
					Queue:   cfg.Node.ID + ".downlink.q",
				},
				Consumer:               rabbitmq.Consumer{RequeueOnError: true},
				Rules:                  ruleSet,
				Worker:                 apply.NewSQLWorker(db),
				TargetDatabaseOverride: cfg.MySQL.Database,
				ConfigStore:            syncstore.New(db),
				MaxBatch:               syncBatchSize(cfg.Sync.DispatchBatchSize),
				FlushInterval:          syncFlushInterval(cfg.Sync.FlushIntervalMillis),
			},
			Status: store,
		},
	}
	if strings.EqualFold(cfg.CDC.Type, "canal") {
		cdcConn, err := rabbitmq.Dial(cfg.RabbitMQ.LocalURL)
		if err != nil {
			fmt.Fprintf(stderr, "local cdc rabbitmq connect failed: %v\n", err)
			return err
		}
		defer cdcConn.Close()
		cdcPublisher, err := rabbitmq.NewPublisher(cdcConn.Channel)
		if err != nil {
			fmt.Fprintf(stderr, "cdc publisher init failed: %v\n", err)
			return err
		}
		canalRuntime, err := newCanalUploadRuntime(cfg, ruleSet, cdc.NewMySQLOffsetStore(db), cdcPublisher)
		if err != nil {
			return err
		}
		workers = append(workers, syncruntime.Worker{
			Config:  workerConfig("edge-cdc-canal", cfg.Sync.RetryIntervalSeconds, maxSteps),
			Stepper: canalRuntime,
			Status:  store,
		})
	}
	group := syncruntime.WorkerGroup{Workers: workers}
	if err := group.Run(ctx); err != nil && err != context.Canceled {
		return err
	}
	fmt.Fprintln(stdout, "edge workers stopped")
	return nil
}

func runServerWorkers(ctx context.Context, cfg *appconfig.Config, ruleSet *rules.RuleSet, edgeNodeIDs []string, store *status.RuntimeStore, maxSteps int, stdout, stderr io.Writer) error {
	if cfg.RabbitMQ.ServerURL == "" {
		return fmt.Errorf("rabbitmq.server_url is required for server run")
	}

	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	conn, err := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	replayConn, err := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq replay connect failed: %v\n", err)
		return err
	}
	defer replayConn.Close()

	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	replayPublisher, err := rabbitmq.NewPublisher(replayConn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "replay publisher init failed: %v\n", err)
		return err
	}
	syncStore := syncstore.New(db)
	workers := []syncruntime.Worker{
		{
			Config: workerConfig("server-ingress", cfg.Sync.RetryIntervalSeconds, maxSteps),
			Stepper: syncruntime.ServerIngressBatchRuntime{
				Source: syncruntime.AMQPBatchGetSource{
					Channel: conn.Channel,
					Queue:   "server.cdc.ingress.q",
				},
				Consumer:   rabbitmq.Consumer{RequeueOnError: true},
				Rules:      ruleSet,
				Worker:     apply.NewSQLWorker(db),
				EventStore: syncStore,
				Dispatcher: syncruntime.RoutingDownlinkDispatcher{
					Publisher: publisher,
					Exchange:  "server.dispatch.x",
				},
				EdgeNodes:     edgeNodeIDs,
				NodeStore:     syncStore,
				MaxBatch:      syncBatchSize(cfg.Sync.DispatchBatchSize),
				FlushInterval: syncFlushInterval(cfg.Sync.FlushIntervalMillis),
			},
			Status: store,
		},
		{
			Config: workerConfig("server-replay", cfg.Sync.RetryIntervalSeconds, maxSteps),
			Stepper: syncruntime.ReplayRuntime{
				Store: syncStore,
				Dispatcher: syncruntime.RoutingDownlinkDispatcher{
					Publisher: replayPublisher,
					Exchange:  "server.dispatch.x",
				},
				Limit: 1,
			},
			Status: store,
		},
	}
	if strings.EqualFold(cfg.CDC.Type, "canal") {
		serverCDCConn, err := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
		if err != nil {
			fmt.Fprintf(stderr, "rabbitmq server cdc connect failed: %v\n", err)
			return err
		}
		defer serverCDCConn.Close()
		serverCDCPublisher, err := rabbitmq.NewPublisher(serverCDCConn.Channel)
		if err != nil {
			fmt.Fprintf(stderr, "server cdc publisher init failed: %v\n", err)
			return err
		}
		serverCDCRuntime, err := newCanalServerDispatchRuntime(cfg, ruleSet, cdc.NewMySQLOffsetStore(db), serverCDCPublisher, syncStore, edgeNodeIDs)
		if err != nil {
			return err
		}
		workers = append(workers, syncruntime.Worker{
			Config:  workerConfig("server-cdc-canal", cfg.Sync.RetryIntervalSeconds, maxSteps),
			Stepper: serverCDCRuntime,
			Status:  store,
		})
	}
	group := syncruntime.WorkerGroup{Workers: workers}
	if err := group.Run(ctx); err != nil && err != context.Canceled {
		return err
	}
	fmt.Fprintln(stdout, "server workers stopped")
	return nil
}

func runInitRabbitMQ(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("init-rabbitmq", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "optional server config for dynamic edge nodes")
	amqpURL := flags.String("amqp-url", "", "RabbitMQ AMQP URL")
	mode := flags.String("mode", "edge", "topology mode: edge or server")
	edges := flags.String("edges", "", "comma-separated edge node ids for server topology")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url is required")
	}

	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()

	edgeIDs := splitCSV(*edges)
	if *mode == appconfig.ModeServer && len(edgeIDs) == 0 && *configPath != "" {
		cfg, err := appconfig.LoadFile(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config failed: %v\n", err)
			return err
		}
		db, err := openMySQL(cfg)
		if err != nil {
			fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
			return err
		}
		defer db.Close()
		edgeIDs, err = syncstore.New(db).ListActiveEdgeNodeIDs(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "list active nodes failed: %v\n", err)
			return err
		}
	}
	topology := topologyForMode(*mode, strings.Join(edgeIDs, ","))
	if err := rabbitmq.InitializeTopology(conn.Channel, topology); err != nil {
		fmt.Fprintf(stderr, "rabbitmq init failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "rabbitmq topology initialized mode=%s vhost=%s\n", *mode, topology.VHost)
	return nil
}

func runPublishEvent(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("publish-event", flag.ContinueOnError)
	flags.SetOutput(stderr)
	amqpURL := flags.String("amqp-url", "", "RabbitMQ AMQP URL")
	eventPath := flags.String("file", "", "path to SyncEvent JSON file")
	exchange := flags.String("exchange", "server.ingress.x", "RabbitMQ exchange")
	routingKey := flags.String("routing-key", "server.ingress", "RabbitMQ routing key")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url is required")
	}
	if *eventPath == "" {
		return fmt.Errorf("file is required")
	}

	body, err := os.ReadFile(*eventPath)
	if err != nil {
		fmt.Fprintf(stderr, "read event failed: %v\n", err)
		return err
	}
	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()

	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	if err := publisher.Publish(context.Background(), rabbitmq.PublishRequest{
		Exchange:   *exchange,
		RoutingKey: *routingKey,
		Body:       body,
	}); err != nil {
		fmt.Fprintf(stderr, "publish failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "event published exchange=%s routing_key=%s file=%s\n", *exchange, *routingKey, *eventPath)
	return nil
}

func runPublishStressBatch(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("publish-stress-batch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	amqpURL := flags.String("amqp-url", "", "RabbitMQ AMQP URL")
	exchange := flags.String("exchange", "edge.upload.x", "RabbitMQ exchange")
	routingKey := flags.String("routing-key", "edge.upload.cdc", "RabbitMQ routing key")
	count := flags.Int("count", 1000, "number of stress events")
	batchSize := flags.Int("batch-size", syncruntime.DefaultBatchSize, "progress batch size")
	eventIDPrefix := flags.String("event-id-prefix", "evt-device-stress", "event id prefix")
	originNodeID := flags.String("origin-node-id", "edge-001", "origin node id")
	databaseName := flags.String("database", "scada_edge", "source database name")
	tableName := flags.String("table", "device_config", "source table name")
	startID := flags.Int("start-id", 1, "starting primary key id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url is required")
	}
	if *count <= 0 {
		return fmt.Errorf("count must be positive")
	}
	if *batchSize <= 0 {
		*batchSize = syncruntime.DefaultBatchSize
	}

	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}

	started := time.Now()
	for i := 0; i < *count; i++ {
		evt := buildStressEvent(stressEventOptions{
			Index:         i + 1,
			PrimaryKeyID:  *startID + i,
			EventIDPrefix: *eventIDPrefix,
			OriginNodeID:  *originNodeID,
			DatabaseName:  *databaseName,
			TableName:     *tableName,
			Now:           started,
		})
		body, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal stress event %s: %w", evt.EventID, err)
		}
		if err := publisher.Publish(context.Background(), rabbitmq.PublishRequest{
			Exchange:   *exchange,
			RoutingKey: *routingKey,
			Body:       body,
		}); err != nil {
			fmt.Fprintf(stderr, "publish stress event failed event_id=%s error=%v\n", evt.EventID, err)
			return err
		}
		if (i+1)%*batchSize == 0 {
			fmt.Fprintf(stdout, "stress batch published count=%d last_event_id=%s\n", i+1, evt.EventID)
		}
	}
	elapsed := time.Since(started)
	fmt.Fprintf(stdout, "stress publish complete count=%d elapsed_ms=%d throughput_per_sec=%.2f\n",
		*count,
		elapsed.Milliseconds(),
		float64(*count)/elapsed.Seconds(),
	)
	return nil
}

type stressEventOptions struct {
	Index         int
	PrimaryKeyID  int
	EventIDPrefix string
	OriginNodeID  string
	DatabaseName  string
	TableName     string
	Now           time.Time
}

func buildStressEvent(opts stressEventOptions) event.SyncEvent {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	eventID := fmt.Sprintf("%s-%05d", opts.EventIDPrefix, opts.Index)
	value := fmt.Sprintf("VALUE-%05d", opts.Index)
	return event.SyncEvent{
		EventID:      eventID,
		EventType:    event.TypeInsert,
		OriginNodeID: opts.OriginNodeID,
		SourceNodeID: opts.OriginNodeID,
		DatabaseName: opts.DatabaseName,
		TableName:    opts.TableName,
		PrimaryKey:   map[string]any{"id": opts.PrimaryKeyID},
		After: map[string]any{
			"id":              opts.PrimaryKeyID,
			"name":            fmt.Sprintf("%s-%05d", opts.TableName, opts.Index),
			"value":           value,
			"sync_version":    opts.Index,
			"updated_by_node": opts.OriginNodeID,
			"last_event_id":   eventID,
			"updated_at":      now.Add(time.Duration(opts.Index) * time.Millisecond).Format("2006-01-02 15:04:05.000"),
			"is_deleted":      0,
		},
		SchemaVersion: 1,
		CreatedAt:     now,
		EventTime:     now.Add(time.Duration(opts.Index) * time.Millisecond),
		TraceID:       "trace-" + eventID,
		Headers:       map[string]string{"stress": "true"},
	}
}

func runPublishChangeOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("publish-change-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	amqpURL := flags.String("amqp-url", "", "Edge local RabbitMQ AMQP URL")
	changePath := flags.String("file", "", "path to ChangeEvent JSON file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *changePath == "" {
		return fmt.Errorf("file is required")
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	if *amqpURL == "" {
		*amqpURL = cfg.RabbitMQ.LocalURL
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url or rabbitmq.local_url is required")
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	change, err := loadChange(*changePath)
	if err != nil {
		fmt.Fprintf(stderr, "load change failed: %v\n", err)
		return err
	}

	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}

	runtime := syncruntime.CDCUploadRuntime{
		Source:     cdc.NewStubSource([]cdc.ChangeEvent{change}),
		Decider:    loop.NewSuppressor(cfg.Node.ID, *ruleSet, nil),
		Normalizer: normalizer.New(normalizer.Options{NodeID: cfg.Node.ID, SchemaVersion: 1}),
		Publisher:  publisher,
		Exchange:   "edge.upload.x",
		RoutingKey: "edge.upload.cdc",
	}
	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "publish change failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintln(stdout, "change source empty")
		return nil
	}
	fmt.Fprintf(stdout, "change published event_id=%s action=%s file=%s\n", result.EventID, result.Action, *changePath)
	return nil
}

func runCanalCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("canal-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	canalConfig := canalConfigFromApp(cfg)
	if err := canalConfig.Validate(); err != nil {
		fmt.Fprintf(stderr, "canal config invalid: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "canal config ready reader=%s addr=%s destination=%s batch_size=%d\n",
		canalConfig.ReaderName,
		canalConfig.Address,
		canalConfig.Destination,
		canalConfig.BatchSize,
	)
	return nil
}

func runCanalPublishOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("canal-publish-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	amqpURL := flags.String("amqp-url", "", "Edge local RabbitMQ AMQP URL")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	if *amqpURL == "" {
		*amqpURL = cfg.RabbitMQ.LocalURL
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url or rabbitmq.local_url is required")
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	runtime, err := newCanalUploadRuntime(cfg, ruleSet, cdc.NewMySQLOffsetStore(db), publisher)
	if err != nil {
		return err
	}
	defer runtime.Stop(context.Background())

	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "canal publish failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintln(stdout, "canal batch empty")
		return nil
	}
	fmt.Fprintf(stdout, "canal batch published action=%s event_id=%s count=%d\n", result.Action, result.EventID, result.DispatchCount)
	return nil
}

func runConsumeOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("consume-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	amqpURL := flags.String("amqp-url", "", "RabbitMQ AMQP URL")
	queueName := flags.String("queue", "server.cdc.ingress.q", "RabbitMQ queue")
	edges := flags.String("edges", "", "comma-separated edge node ids for server dispatch")
	requeue := flags.Bool("requeue-on-error", false, "requeue message when apply fails")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url is required")
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	var dispatcher syncruntime.DownlinkDispatcher
	edgeNodeIDs := splitCSV(*edges)
	if len(edgeNodeIDs) == 0 {
		var err error
		edgeNodeIDs, err = syncstore.New(db).ListActiveEdgeNodeIDs(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "list active nodes failed: %v\n", err)
			return err
		}
	}
	if len(edgeNodeIDs) > 0 {
		publisher, err := rabbitmq.NewPublisher(conn.Channel)
		if err != nil {
			fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
			return err
		}
		dispatcher = syncruntime.RoutingDownlinkDispatcher{
			Publisher: publisher,
			Exchange:  "server.dispatch.x",
		}
	}

	runtime := syncruntime.ServerIngressRuntime{
		Source: syncruntime.AMQPGetSource{
			Channel: conn.Channel,
			Queue:   *queueName,
		},
		Consumer:   rabbitmq.Consumer{RequeueOnError: *requeue},
		Rules:      ruleSet,
		Worker:     apply.NewSQLWorker(db),
		EventStore: syncstore.New(db),
		Dispatcher: dispatcher,
		EdgeNodes:  edgeNodeIDs,
		NodeStore:  syncstore.New(db),
	}
	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "consume apply failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintf(stdout, "queue empty queue=%s\n", *queueName)
		return nil
	}

	fmt.Fprintf(stdout, "message consumed queue=%s event_id=%s action=%s\n", *queueName, result.EventID, result.Action)
	return nil
}

func runForwardUploadOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("forward-upload-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	localURL := flags.String("local-amqp-url", "", "Edge local RabbitMQ AMQP URL")
	serverURL := flags.String("server-amqp-url", "", "Server RabbitMQ AMQP URL")
	queueName := flags.String("queue", "edge.upload.cdc.q", "Edge local upload queue")
	exchange := flags.String("exchange", "server.ingress.x", "Server ingress exchange")
	routingKey := flags.String("routing-key", "server.ingress", "Server ingress routing key")
	requeue := flags.Bool("requeue-on-error", true, "requeue message when forward fails")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *localURL == "" {
		return fmt.Errorf("local-amqp-url is required")
	}
	if *serverURL == "" {
		return fmt.Errorf("server-amqp-url is required")
	}

	localConn, err := rabbitmq.Dial(*localURL)
	if err != nil {
		fmt.Fprintf(stderr, "local rabbitmq connect failed: %v\n", err)
		return err
	}
	defer localConn.Close()
	serverConn, err := rabbitmq.Dial(*serverURL)
	if err != nil {
		fmt.Fprintf(stderr, "server rabbitmq connect failed: %v\n", err)
		return err
	}
	defer serverConn.Close()

	publisher, err := rabbitmq.NewPublisher(serverConn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	runtime := syncruntime.EdgeUploadRuntime{
		Source: syncruntime.AMQPGetSource{
			Channel: localConn.Channel,
			Queue:   *queueName,
		},
		Publisher:  publisher,
		Consumer:   rabbitmq.Consumer{RequeueOnError: *requeue},
		Exchange:   *exchange,
		RoutingKey: *routingKey,
	}
	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "forward upload failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintf(stdout, "queue empty queue=%s\n", *queueName)
		return nil
	}
	fmt.Fprintf(stdout, "upload forwarded queue=%s event_id=%s action=%s\n", *queueName, result.EventID, result.Action)
	return nil
}

func runForwardUploadBatchOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("forward-upload-batch-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	localURL := flags.String("local-amqp-url", "", "Edge local RabbitMQ AMQP URL")
	serverURL := flags.String("server-amqp-url", "", "Server RabbitMQ AMQP URL")
	queueName := flags.String("queue", "edge.upload.cdc.q", "Edge local upload queue")
	exchange := flags.String("exchange", "server.ingress.x", "Server ingress exchange")
	routingKey := flags.String("routing-key", "server.ingress", "Server ingress routing key")
	maxBatch := flags.Int("max-batch", syncruntime.DefaultBatchSize, "maximum messages per batch")
	flushMillis := flags.Int("flush-interval-millis", int(syncruntime.DefaultFlushInterval/time.Millisecond), "batch flush interval in milliseconds")
	requeue := flags.Bool("requeue-on-error", true, "requeue message when forward fails")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *localURL == "" {
		return fmt.Errorf("local-amqp-url is required")
	}
	if *serverURL == "" {
		return fmt.Errorf("server-amqp-url is required")
	}

	localConn, err := rabbitmq.Dial(*localURL)
	if err != nil {
		fmt.Fprintf(stderr, "local rabbitmq connect failed: %v\n", err)
		return err
	}
	defer localConn.Close()
	serverConn, err := rabbitmq.Dial(*serverURL)
	if err != nil {
		fmt.Fprintf(stderr, "server rabbitmq connect failed: %v\n", err)
		return err
	}
	defer serverConn.Close()
	publisher, err := rabbitmq.NewPublisher(serverConn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	result, err := (syncruntime.EdgeUploadBatchRuntime{
		Source: syncruntime.AMQPBatchGetSource{
			Channel: localConn.Channel,
			Queue:   *queueName,
		},
		Publisher:     publisher,
		Consumer:      rabbitmq.Consumer{RequeueOnError: *requeue},
		Exchange:      *exchange,
		RoutingKey:    *routingKey,
		MaxBatch:      *maxBatch,
		FlushInterval: time.Duration(*flushMillis) * time.Millisecond,
	}).RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "forward upload batch failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintf(stdout, "queue empty queue=%s\n", *queueName)
		return nil
	}
	fmt.Fprintf(stdout, "upload batch forwarded queue=%s count=%d event_id=%s action=%s\n", *queueName, result.Count, result.EventID, result.Action)
	return nil
}

func runConsumeDownlinkOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("consume-downlink-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	amqpURL := flags.String("amqp-url", "", "Server RabbitMQ AMQP URL")
	queueName := flags.String("queue", "", "Edge downlink queue on Server RabbitMQ")
	requeue := flags.Bool("requeue-on-error", true, "requeue message when apply fails")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url is required")
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	if *queueName == "" {
		*queueName = cfg.Node.ID + ".downlink.q"
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()

	runtime := syncruntime.EdgeDownlinkRuntime{
		Source: syncruntime.AMQPGetSource{
			Channel: conn.Channel,
			Queue:   *queueName,
		},
		Consumer:               rabbitmq.Consumer{RequeueOnError: *requeue},
		Rules:                  ruleSet,
		Worker:                 apply.NewSQLWorker(db),
		TargetDatabaseOverride: cfg.MySQL.Database,
		ConfigStore:            syncstore.New(db),
	}
	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "consume downlink failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintf(stdout, "queue empty queue=%s\n", *queueName)
		return nil
	}
	fmt.Fprintf(stdout, "downlink consumed queue=%s event_id=%s action=%s\n", *queueName, result.EventID, result.Action)
	return nil
}

func runConsumeBatchOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("consume-batch-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	amqpURL := flags.String("amqp-url", "", "RabbitMQ AMQP URL")
	queueName := flags.String("queue", "server.cdc.ingress.q", "RabbitMQ queue")
	maxBatch := flags.Int("max-batch", syncruntime.DefaultBatchSize, "maximum messages per batch")
	flushMillis := flags.Int("flush-interval-millis", int(syncruntime.DefaultFlushInterval/time.Millisecond), "batch flush interval in milliseconds")
	edges := flags.String("edges", "", "comma-separated edge node ids for server dispatch")
	requeue := flags.Bool("requeue-on-error", false, "requeue message when apply fails")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url is required")
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	var dispatcher syncruntime.DownlinkDispatcher
	edgeNodeIDs := splitCSV(*edges)
	if len(edgeNodeIDs) == 0 {
		edgeNodeIDs, err = syncstore.New(db).ListActiveEdgeNodeIDs(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "list active nodes failed: %v\n", err)
			return err
		}
	}
	if len(edgeNodeIDs) > 0 {
		publisher, err := rabbitmq.NewPublisher(conn.Channel)
		if err != nil {
			fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
			return err
		}
		dispatcher = syncruntime.RoutingDownlinkDispatcher{Publisher: publisher, Exchange: "server.dispatch.x"}
	}
	result, err := (syncruntime.ServerIngressBatchRuntime{
		Source: syncruntime.AMQPBatchGetSource{
			Channel: conn.Channel,
			Queue:   *queueName,
		},
		Consumer:      rabbitmq.Consumer{RequeueOnError: *requeue},
		Rules:         ruleSet,
		Worker:        apply.NewSQLWorker(db),
		EventStore:    syncstore.New(db),
		Dispatcher:    dispatcher,
		EdgeNodes:     edgeNodeIDs,
		NodeStore:     syncstore.New(db),
		MaxBatch:      *maxBatch,
		FlushInterval: time.Duration(*flushMillis) * time.Millisecond,
	}).RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "consume batch failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintf(stdout, "queue empty queue=%s\n", *queueName)
		return nil
	}
	fmt.Fprintf(stdout, "batch consumed queue=%s count=%d event_id=%s action=%s dispatch=%d\n", *queueName, result.Count, result.EventID, result.Action, result.DispatchCount)
	return nil
}

func runConsumeDownlinkBatchOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("consume-downlink-batch-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	amqpURL := flags.String("amqp-url", "", "Server RabbitMQ AMQP URL")
	queueName := flags.String("queue", "", "Edge downlink queue on Server RabbitMQ")
	maxBatch := flags.Int("max-batch", syncruntime.DefaultBatchSize, "maximum messages per batch")
	flushMillis := flags.Int("flush-interval-millis", int(syncruntime.DefaultFlushInterval/time.Millisecond), "batch flush interval in milliseconds")
	requeue := flags.Bool("requeue-on-error", true, "requeue message when apply fails")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url is required")
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	if *queueName == "" {
		*queueName = cfg.Node.ID + ".downlink.q"
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	result, err := (syncruntime.EdgeDownlinkBatchRuntime{
		Source: syncruntime.AMQPBatchGetSource{
			Channel: conn.Channel,
			Queue:   *queueName,
		},
		Consumer:               rabbitmq.Consumer{RequeueOnError: *requeue},
		Rules:                  ruleSet,
		Worker:                 apply.NewSQLWorker(db),
		TargetDatabaseOverride: cfg.MySQL.Database,
		ConfigStore:            syncstore.New(db),
		MaxBatch:               *maxBatch,
		FlushInterval:          time.Duration(*flushMillis) * time.Millisecond,
	}).RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "consume downlink batch failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintf(stdout, "queue empty queue=%s\n", *queueName)
		return nil
	}
	fmt.Fprintf(stdout, "downlink batch consumed queue=%s count=%d event_id=%s action=%s\n", *queueName, result.Count, result.EventID, result.Action)
	return nil
}

func runFailedEvents(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("failed-events", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to sync-agent config file")
	limit := flags.Int("limit", 100, "maximum failed events to list")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	events, err := syncstore.New(db).ListFailedEvents(context.Background(), *limit)
	if err != nil {
		fmt.Fprintf(stderr, "list failed events failed: %v\n", err)
		return err
	}
	for _, item := range events {
		fmt.Fprintf(stdout, "event_id=%s target_node_id=%s status=%s error=%s created_at=%s\n",
			item.EventID,
			item.TargetNodeID,
			item.Status,
			item.ErrorMessage,
			item.CreatedAt.Format(time.RFC3339),
		)
	}
	if len(events) == 0 {
		fmt.Fprintln(stdout, "no failed events")
	}
	return nil
}

func runRetryEvent(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("retry-event", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to sync-agent config file")
	eventID := flags.String("event-id", "", "event id to mark for retry")
	targetNodeID := flags.String("target-node-id", "", "target node id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *eventID == "" {
		return fmt.Errorf("event-id is required")
	}
	if *targetNodeID == "" {
		return fmt.Errorf("target-node-id is required")
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	if err := syncstore.New(db).MarkRetryPending(context.Background(), *eventID, *targetNodeID); err != nil {
		fmt.Fprintf(stderr, "mark retry failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "event marked pending event_id=%s target_node_id=%s\n", *eventID, *targetNodeID)
	return nil
}

func runRetryFailedBatch(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("retry-failed-batch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to sync-agent config file")
	limit := flags.Int("limit", 100, "maximum failed events to mark pending")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	count, err := syncstore.New(db).MarkFailedEventsPending(context.Background(), *limit)
	if err != nil {
		fmt.Fprintf(stderr, "mark failed batch retry failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "failed events marked pending count=%d\n", count)
	return nil
}

func runDeadLetters(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("dead-letters", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to sync-agent config file")
	amqpURL := flags.String("amqp-url", "", "RabbitMQ AMQP URL")
	queueName := flags.String("queue", "", "dead-letter queue name")
	limit := flags.Int("limit", 20, "maximum dead-letter messages to preview")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	targetURL := *amqpURL
	if targetURL == "" {
		targetURL = cfg.RabbitMQ.ServerURL
		if cfg.Mode == appconfig.ModeEdge {
			targetURL = cfg.RabbitMQ.LocalURL
		}
	}
	if targetURL == "" {
		return fmt.Errorf("amqp-url or rabbitmq url is required")
	}
	queue := *queueName
	if queue == "" {
		queue = "server.dead.q"
		if cfg.Mode == appconfig.ModeEdge {
			queue = "edge.dead.q"
		}
	}
	conn, err := rabbitmq.Dial(targetURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	messages, err := rabbitmq.PeekMessages(context.Background(), conn.Channel, queue, *limit)
	if err != nil {
		fmt.Fprintf(stderr, "dead-letter preview failed: %v\n", err)
		return err
	}
	for i, message := range messages {
		fmt.Fprintf(stdout, "dead_letter index=%d queue=%s body_size=%d content_type=%s body_preview=%s\n",
			i+1,
			queue,
			len(message.Body),
			message.ContentType,
			bodyPreview(message.Body, 300),
		)
	}
	if len(messages) == 0 {
		fmt.Fprintf(stdout, "no dead letters queue=%s\n", queue)
	}
	return nil
}

func runReplayPendingOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("replay-pending-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to sync-agent config file")
	amqpURL := flags.String("amqp-url", "", "Server RabbitMQ AMQP URL")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	if *amqpURL == "" {
		*amqpURL = cfg.RabbitMQ.ServerURL
	}
	if *amqpURL == "" {
		return fmt.Errorf("amqp-url or rabbitmq.server_url is required")
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	conn, err := rabbitmq.Dial(*amqpURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}

	result, err := syncruntime.ReplayRuntime{
		Store: syncstore.New(db),
		Dispatcher: syncruntime.RoutingDownlinkDispatcher{
			Publisher: publisher,
			Exchange:  "server.dispatch.x",
		},
		Limit: 1,
	}.RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "replay pending failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintln(stdout, "no pending replay")
		return nil
	}
	fmt.Fprintf(stdout, "pending replayed event_id=%s dispatch_count=%d\n", result.EventID, result.DispatchCount)
	return nil
}

func runDispatchEventOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("dispatch-event-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	amqpURL := flags.String("amqp-url", "", "Server RabbitMQ AMQP URL")
	eventPath := flags.String("file", "", "path to server-origin SyncEvent JSON file")
	edges := flags.String("edges", "", "comma-separated edge node ids; defaults to ACTIVE nodes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *eventPath == "" {
		return fmt.Errorf("file is required")
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	targetURL := *amqpURL
	if targetURL == "" {
		targetURL = cfg.RabbitMQ.ServerURL
	}
	if targetURL == "" {
		return fmt.Errorf("amqp-url or rabbitmq.server_url is required")
	}
	evt, err := loadEvent(*eventPath)
	if err != nil {
		fmt.Fprintf(stderr, "load event failed: %v\n", err)
		return err
	}
	nodeIDs := splitCSV(*edges)
	if len(nodeIDs) == 0 {
		db, err := openMySQL(cfg)
		if err != nil {
			fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
			return err
		}
		defer db.Close()
		nodeIDs, err = syncstore.New(db).ListActiveEdgeNodeIDs(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "list active nodes failed: %v\n", err)
			return err
		}
	}
	conn, err := rabbitmq.Dial(targetURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	dispatcher := syncruntime.RoutingDownlinkDispatcher{Publisher: publisher, Exchange: "server.dispatch.x"}
	count := 0
	for _, nodeID := range nodeIDs {
		if nodeID == "" || nodeID == evt.OriginNodeID {
			continue
		}
		if err := dispatcher.Dispatch(context.Background(), evt, nodeID); err != nil {
			fmt.Fprintf(stderr, "dispatch failed target_node_id=%s error=%v\n", nodeID, err)
			return err
		}
		count++
	}
	fmt.Fprintf(stdout, "event dispatched event_id=%s dispatch_count=%d\n", evt.EventID, count)
	return nil
}

func runServerCDCDispatchOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("server-cdc-dispatch-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules")
	amqpURL := flags.String("amqp-url", "", "Server RabbitMQ AMQP URL")
	changePath := flags.String("file", "", "path to server ChangeEvent JSON file")
	eventID := flags.String("event-id", "", "optional deterministic event id for tests")
	edges := flags.String("edges", "", "comma-separated edge node ids; defaults to ACTIVE nodes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *changePath == "" {
		return fmt.Errorf("file is required")
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	change, err := loadChange(*changePath)
	if err != nil {
		fmt.Fprintf(stderr, "load change failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	syncStore := syncstore.New(db)
	targetURL := *amqpURL
	if targetURL == "" {
		targetURL = cfg.RabbitMQ.ServerURL
	}
	if targetURL == "" {
		return fmt.Errorf("amqp-url or rabbitmq.server_url is required")
	}
	conn, err := rabbitmq.Dial(targetURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	normalizerOptions := normalizer.Options{NodeID: cfg.Node.ID, SchemaVersion: 1}
	if *eventID != "" {
		normalizerOptions.NewEventID = func(time.Time) (string, error) { return *eventID, nil }
	}
	result, err := (syncruntime.ServerCDCDispatchRuntime{
		Source:     cdc.NewStubSource([]cdc.ChangeEvent{change}),
		Decider:    loop.NewSuppressor(cfg.Node.ID, *ruleSet, syncStore),
		Normalizer: normalizer.New(normalizerOptions),
		Rules:      ruleSet,
		Dispatcher: syncruntime.RoutingDownlinkDispatcher{Publisher: publisher, Exchange: "server.dispatch.x"},
		EdgeNodes:  splitCSV(*edges),
		NodeStore:  syncStore,
		EventStore: syncStore,
	}).RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "server cdc dispatch failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "server cdc action=%s event_id=%s dispatch_count=%d\n", result.Action, result.EventID, result.DispatchCount)
	return nil
}

func runServerCanalDispatchOnce(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("server-canal-dispatch-once", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules")
	amqpURL := flags.String("amqp-url", "", "Server RabbitMQ AMQP URL")
	edges := flags.String("edges", "", "comma-separated edge node ids; defaults to ACTIVE nodes")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	targetURL := *amqpURL
	if targetURL == "" {
		targetURL = cfg.RabbitMQ.ServerURL
	}
	if targetURL == "" {
		return fmt.Errorf("amqp-url or rabbitmq.server_url is required")
	}
	conn, err := rabbitmq.Dial(targetURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer conn.Close()
	publisher, err := rabbitmq.NewPublisher(conn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}

	syncStore := syncstore.New(db)
	runtime, err := newCanalServerDispatchRuntime(cfg, ruleSet, cdc.NewMySQLOffsetStore(db), publisher, syncStore, splitCSV(*edges))
	if err != nil {
		return err
	}
	defer runtime.Stop(context.Background())
	result, err := runtime.RunOnce(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "server canal dispatch failed: %v\n", err)
		return err
	}
	if !result.Processed {
		fmt.Fprintln(stdout, "server canal batch empty")
		return nil
	}
	fmt.Fprintf(stdout, "server canal action=%s event_id=%s dispatch_count=%d\n", result.Action, result.EventID, result.DispatchCount)
	return nil
}

func runServeLogWeb(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve-log-web", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	store := status.NewRuntimeStore()
	store.RecordStopped("sync-agent")
	server, err := logweb.NewServer(cfg.LogWeb, store)
	if err != nil {
		fmt.Fprintf(stderr, "log web unavailable: %v\n", err)
		return err
	}

	fmt.Fprintf(stdout, "log web listening addr=%s\n", server.Addr())
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintf(stderr, "log web failed: %v\n", err)
		return err
	}
	return nil
}

func runServeNodeAPI(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve-node-api", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	bind := flags.String("bind", "127.0.0.1", "HTTP bind address")
	port := flags.Int("port", 18090, "HTTP port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	topologyConn, err := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
		return err
	}
	defer topologyConn.Close()
	publisherConn, err := rabbitmq.Dial(cfg.RabbitMQ.ServerURL)
	if err != nil {
		fmt.Fprintf(stderr, "rabbitmq publisher connect failed: %v\n", err)
		return err
	}
	defer publisherConn.Close()
	publisher, err := rabbitmq.NewPublisher(publisherConn.Channel)
	if err != nil {
		fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
		return err
	}
	api := nodeapi.NewServer(syncstore.New(db), nodeapi.AMQPNodeTopology{Channel: topologyConn.Channel}, nodeapi.AMQPConfigPublisher{
		Publisher:    publisher,
		SourceNodeID: cfg.Node.ID,
	})
	addr := fmt.Sprintf("%s:%d", *bind, *port)
	fmt.Fprintf(stdout, "node api listening addr=%s\n", addr)
	return http.ListenAndServe(addr, api.Handler())
}

func runRegisterNode(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("register-node", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	nodeID := flags.String("node-id", "", "edge node id")
	nodeName := flags.String("node-name", "", "edge node name")
	location := flags.String("location", "", "edge location")
	version := flags.String("version", "", "agent version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *nodeID == "" || *nodeName == "" {
		return fmt.Errorf("node-id and node-name are required")
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	if err := syncstore.New(db).UpsertNode(context.Background(), syncstore.NodeRecord{
		NodeID: *nodeID, NodeName: *nodeName, NodeType: "edge", Location: *location, Version: *version, Status: syncstore.StatusActive,
	}); err != nil {
		fmt.Fprintf(stderr, "register node failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "node registered node_id=%s status=%s\n", *nodeID, syncstore.StatusActive)
	return nil
}

func runListNodes(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("list-nodes", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	nodes, err := syncstore.New(db).ListNodes(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "list nodes failed: %v\n", err)
		return err
	}
	for _, node := range nodes {
		fmt.Fprintf(stdout, "node_id=%s name=%s type=%s status=%s version=%s\n", node.NodeID, node.NodeName, node.NodeType, node.Status, node.Version)
	}
	if len(nodes) == 0 {
		fmt.Fprintln(stdout, "no nodes")
	}
	return nil
}

func runSetNodeConfig(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("set-node-config", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	amqpURL := flags.String("amqp-url", "", "Server RabbitMQ AMQP URL")
	nodeID := flags.String("node-id", "", "edge node id")
	mysqlHost := flags.String("mysql-host", "", "edge mysql host")
	mysqlPort := flags.Int("mysql-port", 0, "edge mysql port")
	mysqlDatabase := flags.String("mysql-database", "", "edge mysql database")
	mysqlUsername := flags.String("mysql-username", "", "edge mysql username")
	cdcType := flags.String("cdc-type", "", "cdc type")
	cdcFilter := flags.String("cdc-filter", "", "cdc filter")
	cdcBatchSize := flags.Int("cdc-batch-size", 0, "cdc batch size")
	cdcDestination := flags.String("cdc-destination", "", "cdc destination")
	ruleVersion := flags.Int64("rule-version", 0, "rule version")
	publish := flags.Bool("publish", true, "publish CONFIG_UPDATE to node downlink queue")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *nodeID == "" {
		return fmt.Errorf("node-id is required")
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	nodeConfig := syncstore.NodeConfig{
		NodeID: *nodeID, MySQLHost: *mysqlHost, MySQLPort: *mysqlPort, MySQLDatabase: *mysqlDatabase,
		MySQLUsername: *mysqlUsername, CDCType: *cdcType, CDCFilter: *cdcFilter, CDCBatchSize: *cdcBatchSize,
		CDCDestination: *cdcDestination, RuleVersion: *ruleVersion,
	}
	if err := syncstore.New(db).UpsertNodeConfig(context.Background(), nodeConfig); err != nil {
		fmt.Fprintf(stderr, "set node config failed: %v\n", err)
		return err
	}
	if *publish {
		targetURL := *amqpURL
		if targetURL == "" {
			targetURL = cfg.RabbitMQ.ServerURL
		}
		conn, err := rabbitmq.Dial(targetURL)
		if err != nil {
			fmt.Fprintf(stderr, "rabbitmq connect failed: %v\n", err)
			return err
		}
		defer conn.Close()
		publisher, err := rabbitmq.NewPublisher(conn.Channel)
		if err != nil {
			fmt.Fprintf(stderr, "publisher init failed: %v\n", err)
			return err
		}
		if err := (nodeapi.AMQPConfigPublisher{Publisher: publisher, SourceNodeID: cfg.Node.ID}).PublishConfig(context.Background(), nodeConfig); err != nil {
			fmt.Fprintf(stderr, "publish config failed: %v\n", err)
			return err
		}
	}
	fmt.Fprintf(stdout, "node config saved node_id=%s rule_version=%d\n", *nodeID, *ruleVersion)
	return nil
}

func runListNodeConfig(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("list-node-config", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/server.example.yaml", "path to server config")
	nodeID := flags.String("node-id", "", "edge node id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *nodeID == "" {
		return fmt.Errorf("node-id is required")
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()
	nodeConfig, err := syncstore.New(db).GetNodeConfig(context.Background(), *nodeID)
	if err != nil {
		fmt.Fprintf(stderr, "get node config failed: %v\n", err)
		return err
	}
	encoded, _ := json.Marshal(nodeConfig)
	fmt.Fprintln(stdout, string(encoded))
	return nil
}

func runManagedPlan(args []string, stdout, stderr io.Writer) error {
	return runManagedExecutor(args, stdout, stderr, installerexec.ModePlan)
}

func runManagedApply(args []string, stdout, stderr io.Writer) error {
	return runManagedExecutor(args, stdout, stderr, installerexec.ModeApply)
}

func runManagedRepair(args []string, stdout, stderr io.Writer) error {
	return runManagedExecutor(args, stdout, stderr, installerexec.ModeRepair)
}

func runManagedUninstall(args []string, stdout, stderr io.Writer) error {
	return runManagedExecutor(args, stdout, stderr, installerexec.ModeUninstall)
}

func runInstallerAssetsCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("installer-assets-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	catalogPath := flags.String("catalog", filepath.Join("deploy", "windows", "nodebridge-assets.example.json"), "path to offline asset catalog")
	strict := flags.Bool("strict", true, "return an error when any asset is invalid")
	if err := flags.Parse(args); err != nil {
		return err
	}
	catalog, err := installerassets.LoadCatalog(*catalogPath)
	if err != nil {
		fmt.Fprintf(stderr, "load asset catalog failed: %v\n", err)
		return err
	}
	results := installerassets.ValidateCatalog(catalog)
	ok := installerassets.AllValid(results)
	out := map[string]any{
		"catalog": *catalogPath,
		"ok":      ok,
		"results": results,
	}
	if err := writeJSON(stdout, out); err != nil {
		return err
	}
	if *strict && !ok {
		return fmt.Errorf("asset catalog validation failed")
	}
	return nil
}

func runInstallerCommandPlan(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("installer-command-plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	catalogPath := flags.String("catalog", filepath.Join("deploy", "windows", "nodebridge-assets.example.json"), "path to offline asset catalog")
	if err := flags.Parse(args); err != nil {
		return err
	}
	catalog, err := installerassets.LoadCatalog(*catalogPath)
	if err != nil {
		fmt.Fprintf(stderr, "load asset catalog failed: %v\n", err)
		return err
	}
	return writeJSON(stdout, map[string]any{
		"catalog": *catalogPath,
		"version": catalog.Version,
		"steps":   installerassets.BuildCommandPlan(catalog),
	})
}

func runManagedExecutor(args []string, stdout, stderr io.Writer, mode string) error {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to config")
	manifestPath := flags.String("manifest", defaultManifestPath(), "path to install manifest")
	version := flags.String("version", "0.33.0", "installer plan version")
	installID := flags.String("install-id", "nodebridge-local", "managed install id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	req := installerexec.Request{Config: *cfg, ConfigPath: *configPath, ManifestPath: *manifestPath, Version: *version, InstallID: *installID}
	exec := installerexec.New()
	var result installerexec.Result
	switch mode {
	case installerexec.ModePlan:
		result = exec.Plan(req)
	case installerexec.ModeApply:
		result, err = exec.Apply(context.Background(), req)
	case installerexec.ModeRepair:
		result, err = exec.Repair(context.Background(), req)
	case installerexec.ModeUninstall:
		result, err = exec.Uninstall(context.Background(), req)
	default:
		err = fmt.Errorf("unsupported managed executor mode %s", mode)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s failed: %v\n", mode, err)
		if len(result.Operations) > 0 {
			writeJSON(stdout, result)
		}
		return err
	}
	return writeJSON(stdout, result)
}

func runMCPStdio(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("mcp-stdio", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to config")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	service := mcpstdio.StaticService{
		ConfigPath: *configPath,
		RulesPath:  *rulesPath,
		Config:     *cfg,
		Rules:      append([]rules.SyncRule(nil), ruleSet.Rules...),
	}
	return (mcpstdio.Server{Service: service}).Serve(context.Background(), os.Stdin, stdout)
}

func defaultManifestPath() string {
	if programData := os.Getenv("ProgramData"); programData != "" {
		return filepath.Join(programData, "NodeBridge", "install-manifest.json")
	}
	return filepath.Join(os.TempDir(), "NodeBridge", "install-manifest.json")
}

func writeJSON(stdout io.Writer, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, string(encoded))
	return nil
}

func runReady(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("sync-agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}

	fmt.Fprintf(stdout, "sync-agent ready mode=%s node_id=%s node_name=%s\n", cfg.Mode, cfg.Node.ID, cfg.Node.Name)
	return nil
}

func runMigrate(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	scope := flags.String("scope", "edge", "migration scope: edge or server")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	dir := filepath.Join("migrations", *scope)
	if err := mysqlconn.RunMigrations(context.Background(), db, dir); err != nil {
		fmt.Fprintf(stderr, "migrate failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "migrations applied scope=%s database=%s\n", *scope, cfg.MySQL.Database)
	return nil
}

func runApplyEvent(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("apply-event", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "configs/edge.example.yaml", "path to sync-agent config file")
	rulesPath := flags.String("rules", "configs/sync-rules.example.yaml", "path to sync rules file")
	eventPath := flags.String("file", "", "path to SyncEvent JSON file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *eventPath == "" {
		return fmt.Errorf("file is required")
	}

	cfg, err := appconfig.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "load config failed: %v\n", err)
		return err
	}
	evt, err := loadEvent(*eventPath)
	if err != nil {
		fmt.Fprintf(stderr, "load event failed: %v\n", err)
		return err
	}
	ruleSet, err := rules.LoadFile(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "load rules failed: %v\n", err)
		return err
	}
	rule := ruleSet.FindForNode(evt.DatabaseName, evt.TableName, evt.OriginNodeID, evt.SourceNodeID)
	if rule == nil {
		err := fmt.Errorf("sync rule not found for %s.%s", evt.DatabaseName, evt.TableName)
		fmt.Fprintln(stderr, err)
		return err
	}
	mapped, err := mapper.MapEvent(evt, *rule)
	if err != nil {
		fmt.Fprintf(stderr, "map event failed: %v\n", err)
		return err
	}

	db, err := openMySQL(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "open mysql failed: %v\n", err)
		return err
	}
	defer db.Close()

	result, err := apply.NewSQLWorker(db).Apply(context.Background(), mapped)
	if err != nil {
		fmt.Fprintf(stderr, "apply event failed: %v\n", err)
		return err
	}
	fmt.Fprintf(stdout, "event applied event_id=%s target_table=%s already_applied=%t\n", result.EventID, result.TargetTable, result.AlreadyApplied)
	return nil
}

func loadEvent(path string) (event.SyncEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return event.SyncEvent{}, fmt.Errorf("read event %q: %w", path, err)
	}
	var evt event.SyncEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return event.SyncEvent{}, fmt.Errorf("parse event %q: %w", path, err)
	}
	return evt, nil
}

func loadChange(path string) (cdc.ChangeEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cdc.ChangeEvent{}, fmt.Errorf("read change %q: %w", path, err)
	}
	var change cdc.ChangeEvent
	if err := json.Unmarshal(data, &change); err != nil {
		return cdc.ChangeEvent{}, fmt.Errorf("parse change %q: %w", path, err)
	}
	return change, nil
}

func openMySQL(cfg *appconfig.Config) (*sql.DB, error) {
	if dsn := mysqlDSNOverride(cfg.Mode); dsn != "" {
		return mysqlconn.OpenDSN(dsn)
	}
	return mysqlconn.Open(cfg.MySQL)
}

func mysqlDSNOverride(mode string) string {
	switch mode {
	case appconfig.ModeEdge:
		return os.Getenv("NODEBRIDGE_EDGE_MYSQL_DSN")
	case appconfig.ModeServer:
		return os.Getenv("NODEBRIDGE_SERVER_MYSQL_DSN")
	default:
		return ""
	}
}

func topologyForMode(mode, edges string) rabbitmq.Topology {
	if mode == appconfig.ModeServer {
		return rabbitmq.ServerTopology(splitCSV(edges))
	}
	return rabbitmq.EdgeTopology()
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func bodyPreview(body []byte, limit int) string {
	if limit <= 0 {
		limit = 300
	}
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit])
}

func workerConfig(name string, retrySeconds, maxSteps int) syncruntime.WorkerConfig {
	if retrySeconds <= 0 {
		retrySeconds = 10
	}
	return syncruntime.WorkerConfig{
		Name:          name,
		IdleInterval:  time.Duration(retrySeconds) * time.Second,
		ErrorInterval: time.Duration(retrySeconds) * time.Second,
		MaxSteps:      maxSteps,
	}
}

func syncBatchSize(value int) int {
	if value > 0 {
		return value
	}
	return syncruntime.DefaultBatchSize
}

func syncFlushInterval(millis int) time.Duration {
	if millis > 0 {
		return time.Duration(millis) * time.Millisecond
	}
	return syncruntime.DefaultFlushInterval
}

func canalConfigFromApp(cfg *appconfig.Config) canalcdc.Config {
	readerName := cfg.CDC.ReaderName
	if readerName == "" {
		readerName = cfg.Node.ID
	}
	return canalcdc.Config{
		ReaderName:  readerName,
		Address:     cfg.CDC.CanalAddr,
		Destination: cfg.CDC.Destination,
		Username:    cfg.CDC.Username,
		Password:    cfg.CDC.Password,
		Filter:      cfg.CDC.Filter,
		BatchSize:   cfg.CDC.BatchSize,
	}
}

func newCanalUploadRuntime(cfg *appconfig.Config, ruleSet *rules.RuleSet, offsetStore cdc.OffsetStore, publisher syncruntime.EventPublisher) (*syncruntime.CanalUploadRuntime, error) {
	canalConfig := canalConfigFromApp(cfg)
	client, err := canalcdc.NewWithlinClient(canalConfig)
	if err != nil {
		return nil, err
	}
	adapter, err := canalcdc.NewAdapter(canalConfig, client, offsetStore)
	if err != nil {
		return nil, err
	}
	return &syncruntime.CanalUploadRuntime{
		Source:     adapter,
		Decider:    loop.NewSuppressor(cfg.Node.ID, *ruleSet, nil),
		Normalizer: normalizer.New(normalizer.Options{NodeID: cfg.Node.ID, SchemaVersion: 1}),
		Publisher:  publisher,
		Exchange:   "edge.upload.x",
		RoutingKey: "edge.upload.cdc",
	}, nil
}

func newCanalServerDispatchRuntime(cfg *appconfig.Config, ruleSet *rules.RuleSet, offsetStore cdc.OffsetStore, publisher syncruntime.EventPublisher, store *syncstore.Store, edgeNodeIDs []string) (*syncruntime.ServerCanalDispatchRuntime, error) {
	canalConfig := canalConfigFromApp(cfg)
	client, err := canalcdc.NewWithlinClient(canalConfig)
	if err != nil {
		return nil, err
	}
	adapter, err := canalcdc.NewAdapter(canalConfig, client, offsetStore)
	if err != nil {
		return nil, err
	}
	return &syncruntime.ServerCanalDispatchRuntime{
		Source:     adapter,
		Decider:    loop.NewSuppressor(cfg.Node.ID, *ruleSet, store),
		Normalizer: normalizer.New(normalizer.Options{NodeID: cfg.Node.ID, SchemaVersion: 1}),
		Rules:      ruleSet,
		Dispatcher: syncruntime.RoutingDownlinkDispatcher{Publisher: publisher, Exchange: "server.dispatch.x"},
		EdgeNodes:  edgeNodeIDs,
		NodeStore:  store,
		EventStore: store,
	}, nil
}

func startLogWeb(ctx context.Context, config appconfig.LogWebConfig, store *status.RuntimeStore, stdout, stderr io.Writer) (func(context.Context) error, error) {
	if !config.Enable {
		return func(context.Context) error { return nil }, nil
	}
	server, err := logweb.NewServer(config, store)
	if err != nil {
		fmt.Fprintf(stderr, "log web unavailable: %v\n", err)
		return nil, err
	}
	httpServer := &http.Server{
		Addr:    server.Addr(),
		Handler: server.Handler(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	go func() {
		fmt.Fprintf(stdout, "log web listening addr=%s\n", server.Addr())
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(stderr, "log web failed: %v\n", err)
		}
	}()
	return httpServer.Shutdown, nil
}
