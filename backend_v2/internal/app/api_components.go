package app

import (
	"context"
	"os"

	"aladin/backend_v2/internal/alert"
	alertpostgres "aladin/backend_v2/internal/alert/postgres"
	"aladin/backend_v2/internal/artifact"
	"aladin/backend_v2/internal/artifactref"
	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/changefeed"
	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/copilot"
	copilotpostgres "aladin/backend_v2/internal/copilot/postgres"
	"aladin/backend_v2/internal/copilotagent"
	"aladin/backend_v2/internal/document"
	"aladin/backend_v2/internal/entities"
	entitypostgres "aladin/backend_v2/internal/entities/postgres"
	"aladin/backend_v2/internal/feed"
	feedpostgres "aladin/backend_v2/internal/feed/postgres"
	"aladin/backend_v2/internal/file"
	filepostgres "aladin/backend_v2/internal/file/postgres"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/graphpane"
	graphpanepostgres "aladin/backend_v2/internal/graphpane/postgres"
	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/page"
	"aladin/backend_v2/internal/providerconnection"
	providerconnectionpostgres "aladin/backend_v2/internal/providerconnection/postgres"
	"aladin/backend_v2/internal/readingposition"
	readingpositionpostgres "aladin/backend_v2/internal/readingposition/postgres"
	"aladin/backend_v2/internal/realtime"
	"aladin/backend_v2/internal/reconciliation"
	"aladin/backend_v2/internal/record"
	"aladin/backend_v2/internal/relationship"
	relationshippostgres "aladin/backend_v2/internal/relationship/postgres"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/research"
	"aladin/backend_v2/internal/search"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/source"
	sourcepostgres "aladin/backend_v2/internal/source/postgres"
	"aladin/backend_v2/internal/system"
	systempostgres "aladin/backend_v2/internal/system/postgres"
	"aladin/backend_v2/internal/treesync"
	"aladin/backend_v2/internal/unfurl"
	"aladin/backend_v2/internal/watchlist"
	watchlistpostgres "aladin/backend_v2/internal/watchlist/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type APIProcess struct {
	sharedComponents
	system              system.SystemService
	sources             source.SourceService
	records             record.RecordService
	pages               page.Service
	files               file.FileService
	feed                feed.FeedService
	providerConnections providerconnection.ProviderConnectionService
	realtime            realtime.EventService
	realtimeKeys        realtime.KeyResolver
	sync                coreservice.SyncService
	outboxDrainer       *changefeed.Drainer
	shardKV             coreservice.ShardKVService
	relationships       relationship.RelationshipService
	graphPane           graphpane.GraphPaneService
	entityList          entities.EntityListService
	graphReader         graph.GraphReader
	readingPositions    readingposition.Service
	unfurl              unfurl.UnfurlService
	notifications       alert.NotificationService
	alertEngine         *alert.AlertEngine
	marketData          market.MarketDataService
	copilot             copilot.CopilotService
}

func NewAPIComponents(pool *pgxpool.Pool) *APIProcess {
	return NewAPIComponentsWithProviderConnections(pool, config.LoadProviderConnections(), config.DataVolumePathOrDefault())
}

func NewAPIComponentsWithProviderConnections(pool *pgxpool.Pool, providerConfig config.ProviderConnectionConfig, dataVolumePath string) *APIProcess {
	shared := buildSharedComponents(pool, dataVolumePath)

	syncRepo := repo.NewSyncPostgres(pool)
	syncSvc := reconciliation.New(syncRepo, treesync.NewTreeSyncSource(pool), watchlistpostgres.NewSyncSource(pool), repo.NewShardKVSyncSource(pool), readingpositionpostgres.NewSyncSource(pool))
	realtimeKeys := realtime.NewSubscriptionKeyResolver(
		func(ctx context.Context) (string, error) {
			principal, err := coreservice.RequirePrincipal(ctx)
			return principal.UserID, err
		},
		func(ctx context.Context) error {
			return coreservice.RequireScope(ctx, coreservice.ScopeArtifactsRead)
		},
		func(message string) error { return coreservice.BadRequest(message) },
	)
	realtime := realtime.NewService(realtimeKeys)
	outboxDrainer := changefeed.NewDrainer(syncRepo, realtime, changefeed.DefaultDrainInterval)

	nangoClient := providerconnection.NewHTTPNangoClient(providerConfig.NangoBaseURL, providerConfig.NangoSecretKey)
	nangoBackend := providerconnection.NewNangoProviderConnectionBackend(
		nangoClient,
		providerConfig.NangoBaseURL,
		providerConfig.NangoConnectBaseURL,
		providerConfig.NangoSecretKey,
	)
	providerConnections := providerconnection.NewProviderConnectionService(
		providerconnectionpostgres.NewProviderConnectionPostgres(pool),
		providerCatalog(providerConfig),
		[]providerconnection.ProviderConnectionBackend{nangoBackend},
		providerconnection.WithNangoWebhookSigningKey(providerConfig.NangoWebhookSigningKey),
	)

	quoteSvc := market.NewQuoteService(syncRepo)
	marketDataSvc := market.NewMarketDataService(market.MarketDataConfig{
		Configured:    shared.alpacaConfig.Configured(),
		StreamBaseURL: shared.alpacaConfig.StreamBaseURL,
		Feed:          shared.alpacaConfig.Feed,
		APIKey:        shared.alpacaConfig.APIKey,
		APISecret:     shared.alpacaConfig.APISecret,
	}, quoteSvc, shared.instruments, shared.quoteSnapshots)
	alertEngine := alert.NewAlertEngine(shared.alertRepo, marketDataSvc, shared.quoteSnapshots)
	if observer, ok := marketDataSvc.(interface {
		SetTickObserver(market.TickObserver)
	}); ok {
		observer.SetTickObserver(alertEngine.OnTick)
	}

	var graphReader graph.GraphReader
	if uri := os.Getenv("NEO4J_URI"); uri != "" {
		if reader, err := graph.NewProjector(uri, os.Getenv("NEO4J_USER"), os.Getenv("NEO4J_PASS")); err == nil {
			graphReader = reader
		}
	}

	copilotConfig := config.LoadCopilotAgent()
	var agentClient copilot.CopilotAgent
	if copilotConfig.URL != "" {
		agentClient = copilotagent.New(copilotConfig.URL, copilotConfig.SharedSecret)
	}
	copilotSvc := copilot.NewCopilotService(copilot.CopilotDeps{
		Store:     copilotpostgres.NewCopilotPostgres(pool),
		Agent:     agentClient,
		Realtime:  realtime,
		Model:     copilotConfig.Model,
		Effort:    copilotConfig.Effort,
		Snapshots: shared.quoteSnapshots,
		Artifacts: shared.artifacts,
		Entities:  shared.entityContext,
		Watchlist: shared.watchlist,
	})

	return &APIProcess{
		sharedComponents:    shared,
		system:              system.NewSystemService(systempostgres.NewSystemPostgres(pool)),
		sources:             source.NewSourceService(sourcepostgres.NewSourcePostgres(pool)),
		records:             record.NewRecordService(shared.recordRepo),
		pages:               page.NewService(shared.artifactRepo),
		files:               file.NewFileService(filepostgres.New(pool), shared.artifactFiles),
		feed:                feed.NewFeedService(feedpostgres.NewFeedPostgres(pool)),
		providerConnections: providerConnections,
		realtime:            realtime,
		realtimeKeys:        realtimeKeys,
		sync:                syncSvc,
		outboxDrainer:       outboxDrainer,
		shardKV:             coreservice.NewShardKVService(shared.artifacts, repo.NewShardKVPostgres(pool)),
		relationships:       relationship.NewRelationshipService(relationshippostgres.NewRelationshipPostgres(pool)),
		graphPane:           graphpane.NewGraphPaneService(graphpanepostgres.NewGraphPanePostgres(pool)),
		entityList:          entities.NewEntityListService(entitypostgres.NewEntityListPostgres(pool)),
		graphReader:         graphReader,
		readingPositions:    readingposition.NewService(readingpositionpostgres.New(pool)),
		unfurl:              unfurl.NewUnfurlService(),
		notifications:       alert.NewNotificationService(alertpostgres.NewNotificationsPostgres(pool)),
		alertEngine:         alertEngine,
		marketData:          marketDataSvc,
		copilot:             copilotSvc,
	}
}

func (c *APIProcess) Auth() auth.AuthService              { return c.auth }
func (c *APIProcess) System() system.SystemService        { return c.system }
func (c *APIProcess) Sources() source.SourceService       { return c.sources }
func (c *APIProcess) Records() record.RecordService       { return c.records }
func (c *APIProcess) Artifacts() artifact.ArtifactService { return c.artifacts }
func (c *APIProcess) Pages() page.Service                 { return c.pages }
func (c *APIProcess) Files() file.FileService             { return c.files }
func (c *APIProcess) Feed() feed.FeedService              { return c.feed }
func (c *APIProcess) Insights() insights.InsightService   { return c.insights }
func (c *APIProcess) ProviderConnections() providerconnection.ProviderConnectionService {
	return c.providerConnections
}
func (c *APIProcess) Realtime() realtime.EventService                 { return c.realtime }
func (c *APIProcess) RealtimeKeyResolver() realtime.KeyResolver       { return c.realtimeKeys }
func (c *APIProcess) Sync() coreservice.SyncService                   { return c.sync }
func (c *APIProcess) OutboxDrainer() *changefeed.Drainer              { return c.outboxDrainer }
func (c *APIProcess) DocSurfaceStore() coreservice.DocSurfaceStore    { return c.docSurfaceStore }
func (c *APIProcess) WorkspaceRuntime() coreservice.WorkspaceRuntime  { return c.workspaceRuntime }
func (c *APIProcess) ShardBuild() coreservice.ShardBuildService       { return c.shardBuild }
func (c *APIProcess) ShardResources() shardresource.Service           { return c.shardResources }
func (c *APIProcess) ShardGraphQL() coreservice.ShardGraphQLService   { return c.shardGraphQL }
func (c *APIProcess) ShardReleases() coreservice.ShardReleaseService  { return c.shardReleases }
func (c *APIProcess) ShardKV() coreservice.ShardKVService             { return c.shardKV }
func (c *APIProcess) ShardBridge() coreservice.ShardBridgeService     { return c.shardBridge }
func (c *APIProcess) Relationships() relationship.RelationshipService { return c.relationships }
func (c *APIProcess) Research() research.ResearchService              { return c.research }
func (c *APIProcess) Documents() document.DocumentService             { return c.documents }
func (c *APIProcess) GraphPane() graphpane.GraphPaneService           { return c.graphPane }
func (c *APIProcess) EntityTags() entities.EntityTagService           { return c.entityTags }
func (c *APIProcess) ArtifactRefs() artifactref.ArtifactRefService    { return c.artifactRefs }
func (c *APIProcess) EntityContext() entities.EntityContextService    { return c.entityContext }
func (c *APIProcess) EntityList() entities.EntityListService          { return c.entityList }
func (c *APIProcess) Instruments() instrument.InstrumentService       { return c.instruments }
func (c *APIProcess) Watchlist() watchlist.Service                    { return c.watchlist }
func (c *APIProcess) ReadingPositions() readingposition.Service       { return c.readingPositions }
func (c *APIProcess) Search() search.SearchService                    { return c.search }
func (c *APIProcess) Unfurl() unfurl.UnfurlService                    { return c.unfurl }
func (c *APIProcess) Bars() market.BarService                         { return c.bars }
func (c *APIProcess) Alerts() alert.AlertService                      { return c.alerts }
func (c *APIProcess) Notifications() alert.NotificationService        { return c.notifications }
func (c *APIProcess) AlertEngine() *alert.AlertEngine                 { return c.alertEngine }
func (c *APIProcess) MarketData() market.MarketDataService            { return c.marketData }
func (c *APIProcess) GraphReader() graph.GraphReader                  { return c.graphReader }
func (c *APIProcess) Copilot() copilot.CopilotService                 { return c.copilot }

func providerCatalog(providerConfig config.ProviderConnectionConfig) []providerconnection.ProviderDefinition {
	return []providerconnection.ProviderDefinition{
		{Provider: providerconnection.ProviderGoogle, Label: "Google", Backend: providerconnection.ProviderConnectionBackendNango, ProviderConfigKey: providerConfig.NangoGoogleProviderConfigKey, Description: "Connect Google as the first private-provider path. Gmail ingestion comes next.", Category: "Workspace", Capabilities: []string{"Gmail", "Drive-ready", "Calendar-ready"}},
		comingSoonProvider(providerconnection.ProviderMicrosoft, "Microsoft", "Workspace", "Outlook, OneDrive, and Teams private-source support."),
		comingSoonProvider(providerconnection.ProviderSlack, "Slack", "Communication", "Workspace channels, threads, and shared links."),
		comingSoonProvider(providerconnection.ProviderNotion, "Notion", "Knowledge base", "Pages and databases as private research context."),
		comingSoonProvider(providerconnection.ProviderGitHub, "GitHub", "Code", "Issues, pull requests, discussions, and repository context."),
		comingSoonProvider(providerconnection.ProviderLinear, "Linear", "Planning", "Issues, projects, and product planning context."),
		comingSoonProvider(providerconnection.ProviderDiscord, "Discord", "Community", "Community servers and research conversations."),
		comingSoonProvider(providerconnection.ProviderDropbox, "Dropbox", "Files", "Documents and uploaded files as private context."),
		comingSoonProvider(providerconnection.ProviderAtlassian, "Atlassian", "Planning", "Jira and Confluence work surfaces."),
		comingSoonProvider(providerconnection.ProviderFigma, "Figma", "Design", "Design files and product context."),
	}
}

func comingSoonProvider(provider, label, category, description string) providerconnection.ProviderDefinition {
	return providerconnection.ProviderDefinition{Provider: provider, Label: label, Backend: providerconnection.ProviderConnectionBackendNango, Description: description, Category: category, Capabilities: []string{"Planned"}, ComingSoon: true}
}
