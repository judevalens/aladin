package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/pipeline/blackboard"
	"aladin/backend_v2/internal/pipeline/workers"
	"aladin/backend_v2/internal/ratelimit"
	"aladin/backend_v2/internal/search"
	isync "aladin/backend_v2/internal/sync"
	"aladin/backend_v2/internal/sync/syncers"
)

func main() {
	_ = godotenv.Load()

	// Logging — JSON to stdout + file for Promtail/Loki
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	_ = os.MkdirAll("../logs", 0755)
	logFile, err := os.OpenFile("../logs/worker.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logFile = os.Stdout
	}
	w := io.MultiWriter(os.Stdout, logFile)
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})))

	cfg := config.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Postgres
	pool := db.Connect(ctx, cfg.DatabaseURL)
	defer pool.Close()

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("worker: parse redis url", "component", "worker", "err", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	// asynq client + server (shared across sync queue and pipeline handlers)
	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		slog.Error("worker: parse asynq redis url", "component", "worker", "err", err)
		os.Exit(1)
	}
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	asynqServer := asynq.NewServer(redisOpt, asynq.Config{
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			slog.Error("asynq task failed", "component", "worker", "task_type", task.Type(), "err", err)
		}),
	})

	// Repositories
	artifactRepo := db.NewArtifactRepository(pool)
	sourceRepo   := db.NewSourceRepository(pool)
	insightRepo  := db.NewInsightRepository(pool)

	// Rate limiters
	openaiLimiter := ratelimit.New(60)
	tavilyLimiter := ratelimit.New(20)

	// Search
	tavilyClient   := search.NewTavilyClient(cfg.TavilyAPIKey)
	cachedSearcher := search.NewCachedSearcher(tavilyClient, redisClient, tavilyLimiter)

	// Blackboard
	board := blackboard.New(redisClient)

	// Insight channel
	insightCh := make(chan string, 256)

	// Neo4j promoter (optional — skipped if NEO4J_URI is not set)
	var graphPromoter workers.GraphPromoter
	if cfg.Neo4jURI != "" {
		p, err := graph.NewPromoter(cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPass)
		if err != nil {
			slog.Error("neo4j: failed to create promoter", "component", "worker", "err", err)
			os.Exit(1)
		}
		defer p.Close(ctx)
		graphPromoter = p
		slog.Info("neo4j: promoter ready", "component", "worker", "uri", cfg.Neo4jURI)
	} else {
		slog.Warn("neo4j: NEO4J_URI not set — graph promotion disabled", "component", "worker")
	}

	// LLM clients
	enricher := llm.NewOpenAIEnricher(cfg.OpenAIAPIKey)
	embedder := llm.NewOpenAIEmbedder(cfg.OpenAIAPIKey)

	// Workers
	firstPassWorker := workers.NewFirstPassWorker(board, enricher, openaiLimiter)
	searchWorker    := workers.NewSearchWorker(board, cachedSearcher)
	embedWorker     := workers.NewEmbedWorker(board, embedder, openaiLimiter)
	graphWorker     := workers.NewGraphWorker(board, graphPromoter)

	// Orchestrator
	orchestrator := pipeline.NewOrchestrator(
		board, artifactRepo, insightCh, asynqClient,
		firstPassWorker, searchWorker, embedWorker, graphWorker,
	)
	if err := orchestrator.Recover(ctx); err != nil {
		slog.Warn("orchestrator: recover error", "err", err)
	}

	// Insight worker
	gen           := insights.NewGenerator(insightRepo, pool)
	insightWorker := insights.NewWorker(gen, insightCh)
	insightWorker.Start(ctx)

	// asynq mux — ingest handler kicks off pipeline; workers self-register their own queues.
	mux := asynq.NewServeMux()
	mux.HandleFunc(blackboard.TaskIngestArtifact, func(ctx context.Context, t *asynq.Task) error {
		return board.IngestHandler(orchestrator.EnqueueFirstPass)(ctx, t)
	})
	orchestrator.Start(mux)

	// Sync sourceQueue — registers handlers and starts the scheduler internally
	sourceQueue := isync.NewQueue(asynqClient, sourceRepo, artifactRepo,
		syncers.NewRedditSyncer(),
	)
	sourceQueue.RegisterHandlers(mux)
	sourceQueue.Start(ctx)

	// Start asynq server
	go func() {
		if err := asynqServer.Run(mux); err != nil {
			slog.Error("asynq server stopped", "component", "worker", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		asynqServer.Shutdown()
	}()

	slog.Info("aladin worker running — ctrl+c to stop", "component", "worker")
	<-ctx.Done()
	slog.Info("shutting down", "component", "worker")
}
