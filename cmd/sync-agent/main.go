package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
	"github.com/YufeiSun5/NodeBridge/internal/logweb"
	"github.com/YufeiSun5/NodeBridge/internal/loop"
	"github.com/YufeiSun5/NodeBridge/internal/mapper"
	"github.com/YufeiSun5/NodeBridge/internal/mysqlconn"
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
		case "publish-change-once":
			return runPublishChangeOnce(args[1:], stdout, stderr)
		case "canal-check":
			return runCanalCheck(args[1:], stdout, stderr)
		case "consume-once":
			return runConsumeOnce(args[1:], stdout, stderr)
		case "forward-upload-once":
			return runForwardUploadOnce(args[1:], stdout, stderr)
		case "consume-downlink-once":
			return runConsumeDownlinkOnce(args[1:], stdout, stderr)
		case "failed-events":
			return runFailedEvents(args[1:], stdout, stderr)
		case "retry-event":
			return runRetryEvent(args[1:], stdout, stderr)
		case "replay-pending-once":
			return runReplayPendingOnce(args[1:], stdout, stderr)
		case "serve-log-web":
			return runServeLogWeb(args[1:], stdout, stderr)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	group := syncruntime.WorkerGroup{
		Workers: []syncruntime.Worker{
			{
				Config: workerConfig("edge-upload", cfg.Sync.RetryIntervalSeconds, maxSteps),
				Stepper: syncruntime.EdgeUploadRuntime{
					Source: syncruntime.AMQPGetSource{
						Channel: localConn.Channel,
						Queue:   "edge.upload.cdc.q",
					},
					Publisher:  publisher,
					Consumer:   rabbitmq.Consumer{RequeueOnError: true},
					Exchange:   "server.ingress.x",
					RoutingKey: "server.ingress",
				},
				Status: store,
			},
			{
				Config: workerConfig("edge-downlink", cfg.Sync.RetryIntervalSeconds, maxSteps),
				Stepper: syncruntime.EdgeDownlinkRuntime{
					Source: syncruntime.AMQPGetSource{
						Channel: serverDownlinkConn.Channel,
						Queue:   cfg.Node.ID + ".downlink.q",
					},
					Consumer: rabbitmq.Consumer{RequeueOnError: true},
					Rules:    ruleSet,
					Worker:   apply.NewSQLWorker(db),
				},
				Status: store,
			},
		},
	}
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
	group := syncruntime.WorkerGroup{
		Workers: []syncruntime.Worker{
			{
				Config: workerConfig("server-ingress", cfg.Sync.RetryIntervalSeconds, maxSteps),
				Stepper: syncruntime.ServerIngressRuntime{
					Source: syncruntime.AMQPGetSource{
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
					EdgeNodes: edgeNodeIDs,
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
		},
	}
	if err := group.Run(ctx); err != nil && err != context.Canceled {
		return err
	}
	fmt.Fprintln(stdout, "server workers stopped")
	return nil
}

func runInitRabbitMQ(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("init-rabbitmq", flag.ContinueOnError)
	flags.SetOutput(stderr)
	amqpURL := flags.String("amqp-url", "", "RabbitMQ AMQP URL")
	mode := flags.String("mode", "edge", "topology mode: edge or server")
	edges := flags.String("edges", "edge-001", "comma-separated edge node ids for server topology")
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

	topology := topologyForMode(*mode, *edges)
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
		Consumer: rabbitmq.Consumer{RequeueOnError: *requeue},
		Rules:    ruleSet,
		Worker:   apply.NewSQLWorker(db),
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
	rule := ruleSet.Find(evt.DatabaseName, evt.TableName)
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

func canalConfigFromApp(cfg *appconfig.Config) canalcdc.Config {
	readerName := cfg.CDC.ReaderName
	if readerName == "" {
		readerName = cfg.Node.ID
	}
	return canalcdc.Config{
		ReaderName:  readerName,
		Address:     cfg.CDC.CanalAddr,
		Destination: cfg.CDC.Destination,
		BatchSize:   cfg.CDC.BatchSize,
	}
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
