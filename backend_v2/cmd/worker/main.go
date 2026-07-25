package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	"aladin/backend_v2/internal/pageingest"
	"aladin/backend_v2/internal/pipeline"
	"aladin/backend_v2/internal/pipeline/workers"
	"aladin/backend_v2/internal/ratelimit"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/safego"
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

	// LLM clients
	enricher := llm.NewOpenAIEnricher(cfg.OpenAIAPIKey)
	embedder := llm.NewOpenAIEmbedder(cfg.OpenAIAPIKey)

	// Insight generation — driven by tenant_match via asynq (per matching KG).
	gen := insights.NewGenerator(insightRepo, recordRepo, pool)
	insightEnqueuer := insights.NewAsynqEnqueuer(asynqClient)

	// Pipeline
	pipelineEnqueuer := pipeline.NewAsynqEnqueuer(asynqClient)
	handler := pipeline.NewFullPipelineHandler(pipelineEnqueuer, recordRepo).
		WithTenantItemMatches(tenantItemMatchRepo).
		WithInsightEnqueuer(insightEnqueuer)
	// Entity resolution. The embedder (vector matching + sense split) and the LLM
	// adjudicator are enabled here; both degrade gracefully if the API call fails, so a
	// missing/invalid key falls back to deterministic string-only resolution.
	entityRepo := db.NewEntityRepository(pool)
	entityResolver := entities.NewResolver(entityRepo).
		WithEmbedder(embedder).
		WithAdjudicator(llm.NewOpenAIEntityAdjudicator(cfg.OpenAIAPIKey))
	// Claim extraction + resolution — runs after entity resolution. The embedder +
	// adjudicator give polarity-aware resolution (negations → deny mentions); all degrade
	// gracefully without a key.
	claimService := claims.NewService(db.NewClaimRepository(pool), llm.NewOpenAIClaimExtractor(cfg.OpenAIAPIKey)).
		WithEmbedder(embedder).
		WithAdjudicator(llm.NewOpenAIClaimAdjudicator(cfg.OpenAIAPIKey))
	// Discourse sweep — PARKED per the trading pivot (TRADING_PRD.md D4). The
	// internal/discourse code and the /api/insights read path stay; only the ambient
	// LLM-burning ticker is off. Revive by re-wiring discourse.NewService + a ticker here.
	// Reaper (ambient) — re-drives records stranded in a non-terminal status (e.g. a
	// capture whose enrichment enqueue was lost to a crash). Idempotent: deterministic task
	// ids make re-enqueuing a still-queued task a no-op.
	reaper := pipeline.NewReaper(recordRepo, pipelineEnqueuer)
	safego.Loop(ctx, "worker.reaper", func(ctx context.Context) {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := reaper.Sweep(ctx); err != nil {
					slog.Error("reaper: sweep failed", "component", "reaper", "err", err)
				} else if n > 0 {
					slog.Info("reaper: re-drove stuck records", "component", "reaper", "count", n)
				}
			}
		}
	})

	// You-stream ingestion (Y1) — fold authored pages into the engine once they've been idle
	// ~10 min. Reuses the embedder-enabled claimService so authored claims get embedded
	// (bridge-eligible) like records. The ambient sweep enqueues a coalescing per-page task;
	// the worker reads live page state with revision + idle guards (supersede-on-new-edit).
	ingestWorker := pageingest.NewWorker(
		repo.NewArtifactsPostgres(pool),
		claims.NewAuthoredExtractor(claimService, entityRepo),
		asynqClient,
	).WithEmitter(repo.NewNodeEmitter(pool))
	safego.Loop(ctx, "worker.pageingest", func(ctx context.Context) {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := ingestWorker.Sweep(ctx, 50); err != nil {
					slog.Error("pageingest: sweep failed", "component", "pageingest", "err", err)
				} else if n > 0 {
					slog.Info("pageingest: sweep enqueued", "component", "pageingest", "pages", n)
				}
			}
		}
	})

	// Graph projection (optional) — project the entity/claim layer into Neo4j (the connection
	// lens). Built only when NEO4J_URI is set; otherwise the stage is never enqueued and the
	// pipeline is unaffected.
	var graphProjector *graph.Projector
	if cfg.Neo4jURI != "" {
		gp, err := graph.NewProjector(cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPass)
		if err != nil {
			slog.Error("neo4j: projector init failed", "component", "worker", "err", err)
			os.Exit(1)
		}
		defer gp.Close(ctx)
		graphProjector = gp
		handler.WithGraphProjection(true)
		// Also use the graph as the second claim-retrieval channel: claim resolution now fuses
		// pgvector candidates with graph-neighbour candidates (RRF) before adjudicating.
		claimService.WithGraphCandidates(gp)
		slog.Info("neo4j: graph projection + claim retrieval enabled", "component", "worker", "uri", cfg.Neo4jURI)
	} else {
		slog.Info("neo4j: NEO4J_URI unset — graph projection disabled", "component", "worker")
	}

	// Low-confidence entity search (optional) — resolve the entities the first pass was UNSURE
	// about via web search → context → resolver. Built only when TAVILY_API_KEY is set; the
	// CachedSearcher owns rate limiting, so the worker takes no limiter.
	var lowConfWorker *workers.ResolveLowConfidenceWorker
	var webSearcher search.Searcher
	if cfg.TavilyAPIKey != "" {
		searcher := search.NewCachedSearcher(search.NewTavilyClient(cfg.TavilyAPIKey), redisClient, ratelimit.New(20))
		webSearcher = searcher
		lowConfWorker = workers.NewResolveLowConfidenceWorker(recordRepo, entityResolver, searcher, nil)
		handler.WithLowConfidenceSearch(true)
		slog.Info("search: low-confidence entity search enabled", "component", "worker")
	} else {
		slog.Info("search: TAVILY_API_KEY unset — low-confidence search disabled", "component", "worker")
	}

	// Entity judge sweep (ambient, P1.3) — resolves editor-minted placeholders and the
	// ambiguous merge band: deterministic auto-merge on strong same-kind matches, LLM
	// judge for the rest, web-search escalation on unsure (when Tavily is configured).
	// Every layer degrades gracefully: no key → deterministic tier only; unsure rows
	// stay proposed for manual review. One LLM judgment per pair, ever.
	judgeSweeper := entities.NewJudgeSweeper(entityRepo, llm.NewOpenAIEntityAdjudicator(cfg.OpenAIAPIKey))
	if webSearcher != nil {
		judgeSweeper = judgeSweeper.WithSearcher(webSearcher)
	}
	safego.Loop(ctx, "worker.judge", func(ctx context.Context) {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := judgeSweeper.Sweep(ctx, 50); err != nil {
					slog.Error("entities: judge sweep failed", "component", "entities", "err", err)
				} else if n > 0 {
					slog.Info("entities: judge sweep complete", "component", "entities", "acted", n)
				}
			}
		}
	})

	// Outbox retention — prune drained/replayed events past the retention window so the durable log
	// (dominated by ephemeral live market-quote app_events) can't grow unbounded, which also keeps
	// the tight 50ms drain's xid-range scan cheap. Clients offline longer than the window fall back
	// to a cold-start snapshot; the drain reads forward + re-seeds at the horizon on restart.
	outboxSync := repo.NewSyncPostgres(pool)
	outboxRetention := 5 * time.Minute
	if v := os.Getenv("OUTBOX_RETENTION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			outboxRetention = d
		}
	}
	safego.Loop(ctx, "worker.outbox-prune", func(ctx context.Context) {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := outboxSync.PruneOutbox(ctx, outboxRetention, 5000); err != nil {
					slog.Error("outbox prune failed", "component", "worker", "err", err)
				} else if n > 0 {
					slog.Info("outbox pruned", "component", "worker", "deleted", n, "older_than", outboxRetention.String())
				}
			}
		}
	})

	// Worker heartbeat — the worker owns Redis/Asynq, so it writes real queue counts + a liveness
	// timestamp to Postgres for the (Redis-free) api's WorkerStatus to read (replacing fabricated
	// zeros; a stale beat = worker down/wedged).
	inspector := asynq.NewInspector(redisOpt)
	defer inspector.Close()
	safego.Loop(ctx, "worker.heartbeat", func(ctx context.Context) {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		beat := func() {
			payload, _ := json.Marshal(collectQueueStats(inspector))
			if _, err := pool.Exec(ctx, `UPDATE worker_heartbeat SET updated_at = now(), stats = $1::jsonb WHERE id = 1`, payload); err != nil {
				slog.Warn("worker heartbeat write failed", "component", "worker", "err", err)
			}
		}
		beat() // beat immediately on start
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				beat()
			}
		}
	})

	orch := pipeline.NewOrchestrator(handler)
	orch.Add(workers.NewGlobalFirstPassWorker(enricher, openaiLimiter))
	orch.Add(workers.NewTenantMatchWorker(recordRepo, providerStreamRepo, sourceSubscriptionRepo, tenantItemMatchRepo))
	orch.Add(workers.NewResolveEntitiesWorker(recordRepo, entityResolver))
	orch.Add(workers.NewResolveClaimsWorker(recordRepo, entityRepo, claimService).
		WithSignalEmitter(repo.NewSignalSyncRepo(pool)))
	orch.Add(workers.NewEmbedWorker(recordRepo, embedder, openaiLimiter))
	if lowConfWorker != nil {
		orch.Add(lowConfWorker)
	}
	if graphProjector != nil {
		orch.Add(workers.NewGraphProjectWorker(repo.NewGraphProjectionPostgres(pool), graphProjector))
	}

	// Mux
	mux := asynq.NewServeMux()
	orch.Register(mux)
	insights.RegisterGenerateHandler(mux, gen)
	pageingest.RegisterHandler(mux, ingestWorker)

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
		pipeline.TaskGlobalFirstPass:      10,
		pipeline.TaskTenantMatch:          10,
		pipeline.TaskEmbed:                3,
		pipeline.TaskResolveEntities:      5,
		pipeline.TaskResolveClaims:        5,
		pipeline.TaskResolveLowConfidence: 3,
		pipeline.TaskGraphProject:         3,
		insights.TaskGenerate:             5,
		pageingest.TaskIngestAuthoredPage: 3,
	}
	for name, weight := range syncOrchestrator.Queues() {
		queues[name] = weight
	}
	asynqServer := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    cfg.Concurrency,
		Queues:         queues,
		RetryDelayFunc: pipeline.RetryDelay,
		IsFailure:      pipeline.IsFailure,
		ErrorHandler:   isync.NewAsynqErrorHandler(recordRepo, providerStreamRepo, cycleRepo),
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

// collectQueueStats aggregates Asynq queue counts across all queues for the worker heartbeat.
func collectQueueStats(insp *asynq.Inspector) map[string]any {
	queues, err := insp.Queues()
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var pending, active, scheduled, retry, archived, completed, processed, failed int
	for _, q := range queues {
		info, err := insp.GetQueueInfo(q)
		if err != nil {
			continue
		}
		pending += info.Pending
		active += info.Active
		scheduled += info.Scheduled
		retry += info.Retry
		archived += info.Archived
		completed += info.Completed
		processed += info.Processed
		failed += info.Failed
	}
	return map[string]any{
		"queues": len(queues), "pending": pending, "active": active, "scheduled": scheduled,
		"retry": retry, "archived": archived, "completed": completed, "processed": processed, "failed": failed,
	}
}
