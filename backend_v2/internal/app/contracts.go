package app

import (
	"aladin/backend_v2/internal/changefeed"
	"aladin/backend_v2/internal/readingposition"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/watchlist"
)

// APIComponents is the process-owned graph consumed by cmd/api. The HTTP
// package owns its narrower request-serving interface; this contract also
// exposes the singleton loops whose lifecycle belongs to the API process.
type APIComponents interface {
	Auth() coreservice.AuthService
	System() coreservice.SystemService
	Sources() coreservice.SourceService
	Records() coreservice.RecordService
	Artifacts() coreservice.ArtifactService
	Pages() coreservice.PageService
	Files() coreservice.FileService
	Feed() coreservice.FeedService
	Insights() coreservice.InsightService
	ProviderConnections() coreservice.ProviderConnectionService
	Realtime() coreservice.RealtimeEventService
	RealtimeKeyResolver() coreservice.SubscriptionKeyResolver
	Sync() coreservice.SyncService
	OutboxDrainer() *changefeed.Drainer
	DocSurfaceStore() coreservice.DocSurfaceStore
	WorkspaceRuntime() coreservice.WorkspaceRuntime
	ShardBuild() coreservice.ShardBuildService
	ShardResources() shardresource.Service
	ShardGraphQL() coreservice.ShardGraphQLService
	ShardReleases() coreservice.ShardReleaseService
	ShardKV() coreservice.ShardKVService
	ShardBridge() coreservice.ShardBridgeService
	Relationships() coreservice.RelationshipService
	Research() coreservice.ResearchService
	Documents() coreservice.DocumentService
	GraphPane() coreservice.GraphPaneService
	EntityTags() coreservice.EntityTagService
	ArtifactRefs() coreservice.ArtifactRefService
	EntityContext() coreservice.EntityContextService
	EntityList() coreservice.EntityListService
	Instruments() coreservice.InstrumentService
	Watchlist() watchlist.Service
	ReadingPositions() readingposition.Service
	Search() coreservice.SearchService
	Unfurl() coreservice.UnfurlService
	Bars() coreservice.BarService
	Alerts() coreservice.AlertService
	Notifications() coreservice.NotificationService
	AlertEngine() *coreservice.AlertEngine
	MarketData() coreservice.MarketDataService
	GraphReader() coreservice.GraphReader
	Copilot() coreservice.CopilotService
}

// MCPComponents is the process-owned graph consumed by cmd/mcp. It deliberately
// excludes API lifecycle loops and HTTP-only services.
type MCPComponents interface {
	Auth() coreservice.AuthService
	Artifacts() coreservice.ArtifactService
	PageDocuments() coreservice.PageDocumentService
	Insights() coreservice.InsightService
	DocSurfaceStore() coreservice.DocSurfaceStore
	Preview() coreservice.PreviewService
	ShardBuild() coreservice.ShardBuildService
	ShardResources() shardresource.Service
	ShardGraphQL() coreservice.ShardGraphQLService
	ShardReleases() coreservice.ShardReleaseService
	ShardCatalog() coreservice.ShardCatalogService
	ShardBridge() coreservice.ShardBridgeService
	Documents() coreservice.DocumentService
	EntityTags() coreservice.EntityTagService
	ArtifactRefs() coreservice.ArtifactRefService
	EntityContext() coreservice.EntityContextService
	Instruments() coreservice.InstrumentService
	Watchlist() watchlist.Service
	Search() coreservice.SearchService
	Bars() coreservice.BarService
	QuoteSnapshots() coreservice.QuoteSnapshotSource
	MarketInfo() coreservice.MarketInfoService
	Alerts() coreservice.AlertService
}
