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

	"aladin/backend_v2/internal/claims"
	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/llm"
	"aladin/backend_v2/internal/pipeline"
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

	cfg, err := config.LoadWorker()
	if err != nil {
		slog.Error("worker: config load failed", "component", "worker", "err", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Postgres
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("worker: db connect failed", "component", "worker", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("worker: migrations failed", "component", "worker", "err", err)
		os.Exit(1)
	}

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("worker: parse redis url", "component", "worker", "err", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	// asynq client + server
	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		slog.Error("worker: parse asynq redis url", "component", "worker", "err", err)
		os.Exit(1)
	}
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()

	// Repositories
	recordRepo := db.NewRecordRepository(pool)
	providerStreamRepo := db.NewProviderStreamRepository(pool)
	cycleRepo := db.NewSyncCycleRepository(pool)
	sourceSubscriptionRepo := db.NewSourceSubscriptionRepository(pool)
	tenantItemMatchRepo := db.NewTenantItemMatchRepository(pool)
	insightRepo := db.NewInsightRepository(pool)

	// Rate limiters
	openaiLimiter := ratelimit.New(60)
	tavilyLimiter := ratelimit.New(20)

	// Search
	tavilyClient := search.NewTavilyClient(cfg.TavilyAPIKey)
	cachedSearcher := search.NewCachedSearcher(tavilyClient, redisClient, tavilyLimiter)

	// Neo4j promoter (optional)
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

	// Insight worker
	insightCh := make(chan string, 256)
	gen := insights.NewGenerator(insightRepo, recordRepo, pool)
	insightWorker := insights.NewWorker(gen, insightCh)
	insightWorker.Start(ctx)
	insightEnqueuer := insights.NewAsynqEnqueuer(asynqClient)

	// Pipeline
	pipelineEnqueuer := pipeline.NewAsynqEnqueuer(asynqClient)
	handler := pipeline.NewFullPipelineHandler(pipelineEnqueuer, recordRepo, insightCh).
		WithTenantItemMatches(tenantItemMatchRepo).
		WithInsightEnqueuer(insightEnqueuer)
	// Entity resolution. The embedder (vector matching + sense split) and the LLM
	// adjudicator are enabled here; both degrade gracefully if the API call fails, so a
	// missing/invalid key falls back to deterministic string-only resolution.
	entityRepo := db.NewEntityRepository(pool)
	entityResolver := entities.NewResolver(entityRepo).
		WithEmbedder(embedder).
		WithAdjudicator(llm.NewOpenAIEntityAdjudicator(cfg.OpenAIAPIKey))
	// Claim extraction (C0) — runs after entity resolution; degrades gracefully without a key.
	claimService := claims.NewService(db.NewClaimRepository(pool), llm.NewOpenAIClaimExtractor(cfg.OpenAIAPIKey))
	orch := pipeline.NewOrchestrator(handler)
	orch.Add(workers.NewGlobalFirstPassWorker(enricher, openaiLimiter))
	orch.Add(workers.NewTenantMatchWorker(recordRepo, providerStreamRepo, sourceSubscriptionRepo, tenantItemMatchRepo))
	orch.Add(workers.NewResolveEntitiesWorker(recordRepo, entityResolver))
	orch.Add(workers.NewResolveClaimsWorker(recordRepo, entityRepo, claimService))
	orch.Add(workers.NewFirstPassWorker(enricher, openaiLimiter))
	orch.Add(workers.NewSearchWorker(cachedSearcher))
	orch.Add(workers.NewEmbedWorker(embedder, openaiLimiter))
	orch.Add(workers.NewGraphWorker(graphPromoter))

	// Mux
	mux := asynq.NewServeMux()
	orch.Register(mux)
	insights.RegisterGenerateHandler(mux, gen)

	// Sync orchestrator
	seenStore := isync.NewRedisSeenStore(redisClient)
	syncEnqueuer := isync.NewAsynqEnqueuer(asynqClient)
	syncResultHandler := isync.NewRecordResultHandler(syncEnqueuer, providerStreamRepo, recordRepo, cycleRepo, seenStore)
	syncOrchestrator := isync.NewOrchestratorWithResultHandler(syncEnqueuer, providerStreamRepo, cycleRepo, isync.NewFreshnessFirstArbiter(), syncResultHandler,
		syncers.NewBlueskySyncer(seenStore),
		syncers.NewHackerNewsSyncer(seenStore),
		syncers.NewRedditSyncer(seenStore),
	)

	// asynq server — built after syncOrchestrator so we can pull queue names from syncers
	queues := map[string]int{
		pipeline.TaskGlobalFirstPass: 10,
		pipeline.TaskTenantMatch:     10,
		pipeline.TaskFirstPass:       10,
		pipeline.TaskSearch:          5,
		pipeline.TaskEmbed:           3,
		pipeline.TaskGraph:           5,
		pipeline.TaskResolveEntities: 5,
		pipeline.TaskResolveClaims:   5,
		insights.TaskGenerate:        5,
	}
	for name, weight := range syncOrchestrator.Queues() {
		queues[name] = weight
	}
	asynqServer := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    cfg.Concurrency,
		Queues:         queues,
		RetryDelayFunc: pipeline.RetryDelay,
		IsFailure:      pipeline.IsFailure,
		ErrorHandler:   isync.NewAsynqErrorHandler(providerStreamRepo, cycleRepo),
	})

	syncOrchestrator.RegisterHandlers(mux)
	syncOrchestrator.Start(ctx)

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

	slog.Info("aladin worker running — ctrl+c to stop", "component", "worker", "concurrency", cfg.Concurrency)
	<-ctx.Done()
	slog.Info("shutting down", "component", "worker")
}
