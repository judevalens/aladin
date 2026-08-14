package app

import (
	"os"
	"path/filepath"
	"strings"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/copilotagent"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/market/alpaca"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies interface {
	Auth() coreservice.AuthService
	System() coreservice.SystemService
	Sources() coreservice.SourceService
	Records() coreservice.RecordService
	Artifacts() coreservice.ArtifactService
	Pages() coreservice.PageService
	PageDocuments() coreservice.PageDocumentService
	Files() coreservice.FileService
	Feed() coreservice.FeedService
	Insights() coreservice.InsightService
	ProviderConnections() coreservice.ProviderConnectionService
	Realtime() coreservice.RealtimeEventService
	RealtimeKeyResolver() coreservice.SubscriptionKeyResolver
	Sync() coreservice.SyncService
	// OutboxDrainer is the CDC live-delivery loop (may be nil in tests). Started
	// by the server at boot.
	OutboxDrainer() *coreservice.OutboxDrainer
	// DocSurfaceStore + WorkspaceRuntime back the "app" artifact surface: the
	// MCP process writes files + builds; the API process serves built dist.
	DocSurfaceStore() coreservice.DocSurfaceStore
	WorkspaceRuntime() coreservice.WorkspaceRuntime
	// Preview drives interactive headless inspection of built app pages (MCP only).
	Preview() coreservice.PreviewService
	// ShardBuild runs builds while recording status + emitting build-status events.
	ShardBuild() coreservice.ShardBuildService
	// Relationships is the additive cross-world edge layer (artifacts ↔ records ↔ insights).
	Relationships() coreservice.RelationshipService
	// Research is the research bench's write/read side (RESEARCH_SURFACE_PRD §5).
	Research() coreservice.ResearchService

	// Documents reads ingested file content (INGESTION_PRD).
	Documents() coreservice.DocumentService

	// GraphPane assembles the "On the graph" side pane for an entity.
	GraphPane() coreservice.GraphPaneService
	// EntityTags backs the tag / @entity picker: entity search + artifact↔entity links.
	EntityTags() coreservice.EntityTagService
	// ArtifactRefs backs the `#` cross-reference picker: page/shard search + links.
	ArtifactRefs() coreservice.ArtifactRefService
	// EntityContext backs the Entity Context surface: one entity's identity, its typed
	// edges, the verbatim material accreted under it, and its pending merge decisions.
	EntityContext() coreservice.EntityContextService
	// EntityList backs the Entities index: browse/search the entity registry.
	EntityList() coreservice.EntityListService
	// Instruments backs ticker search (the command-box typeahead) over the securities registry.
	Instruments() coreservice.InstrumentService
	// Watchlist backs the Markets surface: the tickers a user is tracking.
	Watchlist() coreservice.WatchlistService
	// Search is the global command-box search: federates instruments + entities + artifacts.
	Search() coreservice.SearchService
	// Bars is the OHLCV history store behind the ticker chart.
	Bars() coreservice.BarService
	// QuoteSnapshots is the live-quote snapshot source (nil without market-data keys).
	QuoteSnapshots() coreservice.QuoteSnapshotSource
	// MarketInfo is the read-only market-intelligence surface: news, screeners, and the
	// (paper/live) account's balance + positions (nil without Alpaca keys).
	MarketInfo() coreservice.MarketInfoService
	// Alerts backs price alerts (the alert engine that evaluates them runs only in the api process).
	Alerts() coreservice.AlertService
	// Notifications is the reusable durable per-user inbox primitive.
	Notifications() coreservice.NotificationService
	// AlertEngine evaluates alerts on live ticks — Start()ed only in cmd/api (nil in Static).
	AlertEngine() *coreservice.AlertEngine
	// MarketData is the demand-driven live-quote hub (one Alpaca WS → outbox fan-out).
	MarketData() coreservice.MarketDataService
	// GraphReader reads the Neo4j connection lens. Nil when Neo4j isn't configured.
	GraphReader() coreservice.GraphReader
	// Copilot is the in-app agentic LLM interface (streaming, tool-grounded on Aladin data).
	Copilot() coreservice.CopilotService
}

type StaticDependencies struct {
	AuthSvc                coreservice.AuthService
	SystemSvc              coreservice.SystemService
	SourcesSvc             coreservice.SourceService
	RecordsSvc             coreservice.RecordService
	ArtifactsSvc           coreservice.ArtifactService
	PagesSvc               coreservice.PageService
	PageDocumentsSvc       coreservice.PageDocumentService
	FilesSvc               coreservice.FileService
	FeedSvc                coreservice.FeedService
	InsightsSvc            coreservice.InsightService
	ProviderConnectionsSvc coreservice.ProviderConnectionService
	RealtimeSvc            coreservice.RealtimeEventService
	RealtimeKeys           coreservice.SubscriptionKeyResolver
	SyncSvc                coreservice.SyncService
	OutboxDrainerSvc       *coreservice.OutboxDrainer
	DocSurfaceStoreSvc     coreservice.DocSurfaceStore
	WorkspaceRuntimeSvc    coreservice.WorkspaceRuntime
	PreviewSvc             coreservice.PreviewService
	ShardBuildSvc          coreservice.ShardBuildService
	RelationshipsSvc       coreservice.RelationshipService
	GraphPaneSvc           coreservice.GraphPaneService
	ResearchSvc            coreservice.ResearchService
	DocumentsSvc           coreservice.DocumentService
	EntityTagsSvc          coreservice.EntityTagService
	ArtifactRefsSvc        coreservice.ArtifactRefService
	EntityContextSvc       coreservice.EntityContextService
	EntityListSvc          coreservice.EntityListService
	GraphReaderSvc         coreservice.GraphReader
	InstrumentsSvc         coreservice.InstrumentService
	WatchlistSvc           coreservice.WatchlistService
	SearchSvc              coreservice.SearchService
	BarsSvc                coreservice.BarService
	QuoteSnapshotsSvc      coreservice.QuoteSnapshotSource
	MarketInfoSvc          coreservice.MarketInfoService
	AlertsSvc              coreservice.AlertService
	NotificationsSvc       coreservice.NotificationService
	AlertEngineSvc         *coreservice.AlertEngine
	MarketDataSvc          coreservice.MarketDataService
	CopilotSvc             coreservice.CopilotService
}

func (d StaticDependencies) Auth() coreservice.AuthService          { return d.AuthSvc }
func (d StaticDependencies) System() coreservice.SystemService      { return d.SystemSvc }
func (d StaticDependencies) Sources() coreservice.SourceService     { return d.SourcesSvc }
func (d StaticDependencies) Records() coreservice.RecordService     { return d.RecordsSvc }
func (d StaticDependencies) Artifacts() coreservice.ArtifactService { return d.ArtifactsSvc }
func (d StaticDependencies) Pages() coreservice.PageService         { return d.PagesSvc }
func (d StaticDependencies) PageDocuments() coreservice.PageDocumentService {
	return d.PageDocumentsSvc
}
func (d StaticDependencies) Files() coreservice.FileService       { return d.FilesSvc }
func (d StaticDependencies) Feed() coreservice.FeedService        { return d.FeedSvc }
func (d StaticDependencies) Insights() coreservice.InsightService { return d.InsightsSvc }
func (d StaticDependencies) ProviderConnections() coreservice.ProviderConnectionService {
	return d.ProviderConnectionsSvc
}
func (d StaticDependencies) Realtime() coreservice.RealtimeEventService {
	return d.RealtimeSvc
}
func (d StaticDependencies) RealtimeKeyResolver() coreservice.SubscriptionKeyResolver {
	return d.RealtimeKeys
}
func (d StaticDependencies) Sync() coreservice.SyncService { return d.SyncSvc }
func (d StaticDependencies) OutboxDrainer() *coreservice.OutboxDrainer {
	return d.OutboxDrainerSvc
}
func (d StaticDependencies) DocSurfaceStore() coreservice.DocSurfaceStore {
	return d.DocSurfaceStoreSvc
}
func (d StaticDependencies) WorkspaceRuntime() coreservice.WorkspaceRuntime {
	return d.WorkspaceRuntimeSvc
}
func (d StaticDependencies) Preview() coreservice.PreviewService {
	return d.PreviewSvc
}
func (d StaticDependencies) ShardBuild() coreservice.ShardBuildService {
	return d.ShardBuildSvc
}
func (d StaticDependencies) Relationships() coreservice.RelationshipService {
	return d.RelationshipsSvc
}
func (d StaticDependencies) Research() coreservice.ResearchService { return d.ResearchSvc }

func (d StaticDependencies) Documents() coreservice.DocumentService { return d.DocumentsSvc }

func (d StaticDependencies) GraphPane() coreservice.GraphPaneService {
	return d.GraphPaneSvc
}
func (d StaticDependencies) EntityTags() coreservice.EntityTagService {
	return d.EntityTagsSvc
}
func (d StaticDependencies) ArtifactRefs() coreservice.ArtifactRefService {
	return d.ArtifactRefsSvc
}
func (d StaticDependencies) EntityContext() coreservice.EntityContextService {
	return d.EntityContextSvc
}
func (d StaticDependencies) EntityList() coreservice.EntityListService {
	return d.EntityListSvc
}
func (d StaticDependencies) Instruments() coreservice.InstrumentService {
	return d.InstrumentsSvc
}
func (d StaticDependencies) Watchlist() coreservice.WatchlistService {
	return d.WatchlistSvc
}
func (d StaticDependencies) Search() coreservice.SearchService {
	return d.SearchSvc
}
func (d StaticDependencies) Bars() coreservice.BarService {
	return d.BarsSvc
}
func (d StaticDependencies) QuoteSnapshots() coreservice.QuoteSnapshotSource {
	return d.QuoteSnapshotsSvc
}
func (d StaticDependencies) MarketInfo() coreservice.MarketInfoService {
	return d.MarketInfoSvc
}
func (d StaticDependencies) Alerts() coreservice.AlertService { return d.AlertsSvc }
func (d StaticDependencies) Notifications() coreservice.NotificationService {
	return d.NotificationsSvc
}
func (d StaticDependencies) AlertEngine() *coreservice.AlertEngine { return d.AlertEngineSvc }
func (d StaticDependencies) MarketData() coreservice.MarketDataService {
	return d.MarketDataSvc
}
func (d StaticDependencies) GraphReader() coreservice.GraphReader {
	return d.GraphReaderSvc
}
func (d StaticDependencies) Copilot() coreservice.CopilotService {
	return d.CopilotSvc
}

type wiring struct {
	auth                coreservice.AuthService
	system              coreservice.SystemService
	sources             coreservice.SourceService
	records             coreservice.RecordService
	artifacts           coreservice.ArtifactService
	pages               coreservice.PageService
	pageDocuments       coreservice.PageDocumentService
	files               coreservice.FileService
	feed                coreservice.FeedService
	insights            coreservice.InsightService
	providerConnections coreservice.ProviderConnectionService
	realtime            coreservice.RealtimeEventService
	rtKeys              coreservice.SubscriptionKeyResolver
	sync                coreservice.SyncService
	outboxDrainer       *coreservice.OutboxDrainer
	docSurfaceStore     coreservice.DocSurfaceStore
	workspaceRuntime    coreservice.WorkspaceRuntime
	preview             coreservice.PreviewService
	shardBuild          coreservice.ShardBuildService
	relationships       coreservice.RelationshipService
	graphPane           coreservice.GraphPaneService
	research            coreservice.ResearchService
	documents           coreservice.DocumentService
	entityTags          coreservice.EntityTagService
	artifactRefs        coreservice.ArtifactRefService
	entityContext       coreservice.EntityContextService
	entityList          coreservice.EntityListService
	graphReader         coreservice.GraphReader
	instruments         coreservice.InstrumentService
	watchlist           coreservice.WatchlistService
	search              coreservice.SearchService
	bars                coreservice.BarService
	quoteSnapshots      coreservice.QuoteSnapshotSource
	marketInfo          coreservice.MarketInfoService
	alerts              coreservice.AlertService
	notifications       coreservice.NotificationService
	alertEngine         *coreservice.AlertEngine
	marketData          coreservice.MarketDataService
	copilot             coreservice.CopilotService
}

func (w wiring) Auth() coreservice.AuthService          { return w.auth }
func (w wiring) System() coreservice.SystemService      { return w.system }
func (w wiring) Sources() coreservice.SourceService     { return w.sources }
func (w wiring) Records() coreservice.RecordService     { return w.records }
func (w wiring) Artifacts() coreservice.ArtifactService { return w.artifacts }
func (w wiring) Pages() coreservice.PageService         { return w.pages }
func (w wiring) PageDocuments() coreservice.PageDocumentService {
	return w.pageDocuments
}
func (w wiring) Files() coreservice.FileService       { return w.files }
func (w wiring) Feed() coreservice.FeedService        { return w.feed }
func (w wiring) Insights() coreservice.InsightService { return w.insights }
func (w wiring) ProviderConnections() coreservice.ProviderConnectionService {
	return w.providerConnections
}
func (w wiring) Realtime() coreservice.RealtimeEventService {
	return w.realtime
}
func (w wiring) RealtimeKeyResolver() coreservice.SubscriptionKeyResolver {
	return w.rtKeys
}
func (w wiring) Sync() coreservice.SyncService                  { return w.sync }
func (w wiring) OutboxDrainer() *coreservice.OutboxDrainer      { return w.outboxDrainer }
func (w wiring) DocSurfaceStore() coreservice.DocSurfaceStore   { return w.docSurfaceStore }
func (w wiring) WorkspaceRuntime() coreservice.WorkspaceRuntime { return w.workspaceRuntime }
func (w wiring) Preview() coreservice.PreviewService            { return w.preview }
func (w wiring) ShardBuild() coreservice.ShardBuildService      { return w.shardBuild }
func (w wiring) Relationships() coreservice.RelationshipService { return w.relationships }
func (w wiring) GraphPane() coreservice.GraphPaneService        { return w.graphPane }
func (w wiring) Research() coreservice.ResearchService          { return w.research }
func (w wiring) Documents() coreservice.DocumentService         { return w.documents }
func (w wiring) EntityTags() coreservice.EntityTagService       { return w.entityTags }
func (w wiring) ArtifactRefs() coreservice.ArtifactRefService   { return w.artifactRefs }
func (w wiring) EntityContext() coreservice.EntityContextService {
	return w.entityContext
}
func (w wiring) EntityList() coreservice.EntityListService {
	return w.entityList
}
func (w wiring) GraphReader() coreservice.GraphReader       { return w.graphReader }
func (w wiring) Instruments() coreservice.InstrumentService { return w.instruments }
func (w wiring) Watchlist() coreservice.WatchlistService    { return w.watchlist }
func (w wiring) Search() coreservice.SearchService          { return w.search }
func (w wiring) Bars() coreservice.BarService               { return w.bars }
func (w wiring) QuoteSnapshots() coreservice.QuoteSnapshotSource {
	return w.quoteSnapshots
}
func (w wiring) MarketInfo() coreservice.MarketInfoService      { return w.marketInfo }
func (w wiring) Alerts() coreservice.AlertService               { return w.alerts }
func (w wiring) Notifications() coreservice.NotificationService { return w.notifications }
func (w wiring) AlertEngine() *coreservice.AlertEngine          { return w.alertEngine }
func (w wiring) MarketData() coreservice.MarketDataService      { return w.marketData }
func (w wiring) Copilot() coreservice.CopilotService            { return w.copilot }

func NewDependencies(pool *pgxpool.Pool) Dependencies {
	return NewDependenciesWithProviderConnections(pool, config.LoadProviderConnections(), config.DataVolumePathOrDefault())
}

func NewDependenciesWithProviderConnections(pool *pgxpool.Pool, providerConfig config.ProviderConnectionConfig, dataVolumePath string) Dependencies {
	authRepo := repo.NewAuthPostgres(pool)
	sourceRepo := repo.NewSourcePostgres(pool)
	recordRepo := repo.NewRecordPostgres(pool)
	artifactRepo := repo.NewArtifactsPostgres(pool)
	artifactFiles := NewArtifactFileStore()
	docStore := docsurface.NewStore(dataVolumePath)
	docRuntime := docsurface.NewBuilder(docStore, filepath.Join(dataVolumePath, "cache", "esm"))
	docPreview := docsurface.NewPreviewSessions(docStore, docRuntime, docsurface.PreviewOptions{})
	shardBuild := coreservice.NewShardBuildService(docRuntime, repo.NewShardBuildPostgres(pool))
	feedRepo := repo.NewFeedPostgres(pool)
	insightRepo := repo.NewInsightPostgres(pool)
	relationshipRepo := repo.NewRelationshipPostgres(pool)
	graphPaneRepo := repo.NewGraphPanePostgres(pool)
	entityTagRepo := repo.NewEntityTagPostgres(pool)
	artifactRefRepo := repo.NewArtifactRefPostgres(pool)
	systemRepo := repo.NewSystemPostgres(pool)

	// Graph reader (optional) — the Neo4j connection lens. Nil when NEO4J_URI is unset, and
	// handlers degrade to a clear "graph not configured" response.
	var graphReader coreservice.GraphReader
	if uri := os.Getenv("NEO4J_URI"); uri != "" {
		if rd, err := graph.NewProjector(uri, os.Getenv("NEO4J_USER"), os.Getenv("NEO4J_PASS")); err == nil {
			graphReader = rd
		}
	}
	providerConnectionRepo := repo.NewProviderConnectionPostgres(pool)
	syncRepo := repo.NewSyncPostgres(pool)
	syncSvc := coreservice.NewSyncService(syncRepo, repo.NewTreeSyncSource(pool), repo.NewWatchlistSyncSource(pool))
	realtimeKeys := coreservice.NewSubscriptionKeyResolver()
	realtime := coreservice.NewInMemoryRealtimeEventService(realtimeKeys)
	outboxDrainer := coreservice.NewOutboxDrainer(syncRepo, realtime, coreservice.DefaultDrainInterval)
	nangoClient := coreservice.NewHTTPNangoClient(providerConfig.NangoBaseURL, providerConfig.NangoSecretKey)
	nangoBackend := coreservice.NewNangoProviderConnectionBackend(
		nangoClient,
		providerConfig.NangoBaseURL,
		providerConfig.NangoConnectBaseURL,
		providerConfig.NangoSecretKey,
	)
	providerConnections := coreservice.NewProviderConnectionService(
		providerConnectionRepo,
		providerCatalog(providerConfig),
		[]coreservice.ProviderConnectionBackend{nangoBackend},
		coreservice.WithNangoWebhookSigningKey(providerConfig.NangoWebhookSigningKey),
	)

	// Alpaca REST client (nil without keys) feeds the read-through caches: bars and instruments
	// fill from the vendor on a local miss/stale rather than bulk backfill.
	alpacaCfg := config.LoadAlpaca()
	var barSource coreservice.BarSource
	var assetLookup coreservice.AssetLookup
	var snapshotSource coreservice.QuoteSnapshotSource
	var marketInfo coreservice.MarketInfoService
	if alpacaCfg.Configured() {
		restClient := alpaca.NewClient(alpacaCfg.APIKey, alpacaCfg.APISecret, alpacaCfg.TradingBaseURL, alpacaCfg.DataBaseURL)
		barSource = alpacaBarSource{c: restClient}
		assetLookup = alpacaAssetLookup{c: restClient}
		snapshotSource = alpacaSnapshotSource{c: restClient}
		marketInfo = alpacaMarketInfo{c: restClient, paper: strings.Contains(alpacaCfg.TradingBaseURL, "paper")}
	}

	// Global search federates the existing search services (instruments + entities +
	// artifacts), so they're built as vars here and shared with the search providers.
	instrumentsSvc := coreservice.NewInstrumentService(repo.NewInstrumentPostgres(pool))
	if assetLookup != nil {
		instrumentsSvc = instrumentsSvc.WithAssetLookup(assetLookup)
	}
	// Adjust-on-read (TRADING_PRD §5): raw bars + the corporate-actions log. Without the log the
	// service serves raw bars, where a split reads as a crash.
	barsSvc := coreservice.NewBarService(repo.NewBarPostgres(pool)).
		WithCorporateActions(repo.NewCorporateActionPostgres(pool))
	if barSource != nil {
		barsSvc = barsSvc.WithSource(barSource)
	}
	entityTagsSvc := coreservice.NewEntityTagService(entityTagRepo, entities.Normalize)
	artifactRefsSvc := coreservice.NewArtifactRefService(artifactRefRepo)
	searchSvc := coreservice.NewSearchService(
		coreservice.NewInstrumentSearchProvider(instrumentsSvc),
		coreservice.NewEntitySearchProvider(entityTagsSvc),
		coreservice.NewArtifactSearchProvider(artifactRefsSvc),
	)

	// Live market data: the quote hub publishes upstream Alpaca ticks to the outbox (broadcast
	// 'market' stream); the OutboxDrainer fans them out. No keys ⇒ the hub is a no-op stream.
	quoteSvc := coreservice.NewQuoteService(syncRepo)
	marketDataSvc := coreservice.NewMarketDataService(coreservice.MarketDataConfig{
		Configured:    alpacaCfg.Configured(),
		StreamBaseURL: alpacaCfg.StreamBaseURL,
		Feed:          alpacaCfg.Feed,
		APIKey:        alpacaCfg.APIKey,
		APISecret:     alpacaCfg.APISecret,
	}, quoteSvc, instrumentsSvc, snapshotSource)

	// Services the copilot exposes as read tools are shared with it, so build them as vars.
	insightsSvc := coreservice.NewInsightService(insightRepo)
	artifactsSvc := coreservice.NewArtifactService(artifactRepo, artifactFiles)
	pagesSvc := coreservice.NewPageService(artifactRepo)
	watchlistSvc := coreservice.NewWatchlistService(repo.NewWatchlistPostgres(pool))
	// Notifications (reusable durable inbox) + price alerts. The alert ENGINE that evaluates
	// alerts on live ticks is started only in cmd/api (it needs the market hub); these services
	// are the CRUD surface, available in both api and mcp processes. alertsSvc snapshot-seeds
	// armed state via snapshotSource (nil-safe: alerts still create, just always-armed).
	notificationsSvc := coreservice.NewNotificationService(repo.NewNotificationsPostgres(pool))
	alertRepo := repo.NewAlertsPostgres(pool)
	alertsSvc := coreservice.NewAlertService(alertRepo, instrumentsSvc, snapshotSource)
	// The alert engine: a singleton tick consumer. Built here (deps for both api + mcp) but
	// Start()ed only in cmd/api (it needs the running market stream). The market hub is its
	// demand source (Subscribe/Unsubscribe) AND its tick source (SetTickObserver). SetTickObserver
	// is harmless without Start (mcp never runs the stream, so no ticks flow).
	alertEngine := coreservice.NewAlertEngine(alertRepo, marketDataSvc, snapshotSource)
	if obs, ok := marketDataSvc.(interface {
		SetTickObserver(coreservice.TickObserver)
	}); ok {
		obs.SetTickObserver(alertEngine.OnTick)
	}
	entityContextSvc := coreservice.NewEntityContextService(
		repo.NewEntityContextPostgres(pool),
		db.NewEntityRepository(pool), // merge decisions land in the shared registry
	)

	// Copilot — the in-app agentic LLM interface. The agent loop runs in the copilot-agent
	// sidecar (Claude Agent SDK), consuming tools from the Go MCP server; this service
	// orchestrates turns + streams events. nil client (COPILOT_AGENT_URL="") ⇒ the endpoint
	// degrades cleanly.
	caCfg := config.LoadCopilotAgent()
	var agentClient coreservice.CopilotAgent
	if caCfg.URL != "" {
		agentClient = copilotagent.New(caCfg.URL, caCfg.SharedSecret)
	}
	copilotSvc := coreservice.NewCopilotService(coreservice.CopilotDeps{
		Store:     repo.NewCopilotPostgres(pool),
		Agent:     agentClient,
		Realtime:  realtime,
		Model:     caCfg.Model,
		Effort:    caCfg.Effort,
		Snapshots: snapshotSource,
		Artifacts: artifactsSvc,
		Entities:  entityContextSvc,
		Watchlist: watchlistSvc,
	})

	return wiring{
		auth:                coreservice.NewAuthService(authRepo, coreservice.NewPasswordHasher()),
		system:              coreservice.NewSystemService(systemRepo),
		sources:             coreservice.NewSourceService(sourceRepo),
		records:             coreservice.NewRecordService(recordRepo),
		artifacts:           artifactsSvc,
		pages:               pagesSvc,
		pageDocuments:       coreservice.NewPageDocumentService(artifactRepo),
		files:               coreservice.NewFileService(artifactRepo, artifactFiles),
		feed:                coreservice.NewFeedService(feedRepo),
		insights:            insightsSvc,
		providerConnections: providerConnections,
		realtime:            realtime,
		rtKeys:              realtimeKeys,
		sync:                syncSvc,
		outboxDrainer:       outboxDrainer,
		docSurfaceStore:     docStore,
		workspaceRuntime:    docRuntime,
		preview:             docPreview,
		shardBuild:          shardBuild,
		relationships:       coreservice.NewRelationshipService(relationshipRepo),
		graphPane:           coreservice.NewGraphPaneService(graphPaneRepo),
		research:            coreservice.NewResearchService(repo.NewResearchPostgres(pool)),
		documents:           coreservice.NewDocumentService(repo.NewDocumentPostgres(pool, artifactFiles)),
		entityTags:          entityTagsSvc,
		artifactRefs:        artifactRefsSvc,
		entityContext:       entityContextSvc,
		entityList:          coreservice.NewEntityListService(repo.NewEntityListPostgres(pool)),
		graphReader:         graphReader,
		instruments:         instrumentsSvc,
		watchlist:           watchlistSvc,
		search:              searchSvc,
		bars:                barsSvc,
		quoteSnapshots:      snapshotSource,
		marketInfo:          marketInfo,
		alerts:              alertsSvc,
		notifications:       notificationsSvc,
		alertEngine:         alertEngine,
		marketData:          marketDataSvc,
		copilot:             copilotSvc,
	}
}

func providerCatalog(providerConfig config.ProviderConnectionConfig) []coreservice.ProviderDefinition {
	return []coreservice.ProviderDefinition{
		{
			Provider:          coreservice.ProviderGoogle,
			Label:             "Google",
			Backend:           coreservice.ProviderConnectionBackendNango,
			ProviderConfigKey: providerConfig.NangoGoogleProviderConfigKey,
			Description:       "Connect Google as the first private-provider path. Gmail ingestion comes next.",
			Category:          "Workspace",
			Capabilities:      []string{"Gmail", "Drive-ready", "Calendar-ready"},
		},
		comingSoonProvider(coreservice.ProviderMicrosoft, "Microsoft", "Workspace", "Outlook, OneDrive, and Teams private-source support."),
		comingSoonProvider(coreservice.ProviderSlack, "Slack", "Communication", "Workspace channels, threads, and shared links."),
		comingSoonProvider(coreservice.ProviderNotion, "Notion", "Knowledge base", "Pages and databases as private research context."),
		comingSoonProvider(coreservice.ProviderGitHub, "GitHub", "Code", "Issues, pull requests, discussions, and repository context."),
		comingSoonProvider(coreservice.ProviderLinear, "Linear", "Planning", "Issues, projects, and product planning context."),
		comingSoonProvider(coreservice.ProviderDiscord, "Discord", "Community", "Community servers and research conversations."),
		comingSoonProvider(coreservice.ProviderDropbox, "Dropbox", "Files", "Documents and uploaded files as private context."),
		comingSoonProvider(coreservice.ProviderAtlassian, "Atlassian", "Planning", "Jira and Confluence work surfaces."),
		comingSoonProvider(coreservice.ProviderFigma, "Figma", "Design", "Design files and product context."),
	}
}

func comingSoonProvider(provider, label, category, description string) coreservice.ProviderDefinition {
	return coreservice.ProviderDefinition{
		Provider:     provider,
		Label:        label,
		Backend:      coreservice.ProviderConnectionBackendNango,
		Description:  description,
		Category:     category,
		Capabilities: []string{"Planned"},
		ComingSoon:   true,
	}
}

// NewArtifactFileStore builds the artifact resource store. Exported because the WORKER
// needs the identical directories to read what the API wrote — ingestion opens files the
// upload path saved, and two processes computing these paths separately is a drift bug
// waiting to happen.
//
// Note the paths are CWD-relative (pre-existing): api and worker must run from the same
// working directory. Sharing the constructor at least guarantees they agree on the shape.
func NewArtifactFileStore() *repo.FilesystemArtifactStore {
	return repo.NewFilesystemArtifactStore(uploadDir(), audioDir())
}

func uploadDir() string {
	return filepath.Join(".", "uploads")
}

func audioDir() string {
	return filepath.Join(".", "audio")
}
