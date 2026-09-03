package app

import (
	"aladin/backend_v2/internal/alert"
	"aladin/backend_v2/internal/artifact"
	"aladin/backend_v2/internal/artifactref"
	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/changefeed"
	"aladin/backend_v2/internal/copilot"
	"aladin/backend_v2/internal/document"
	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/feed"
	"aladin/backend_v2/internal/file"
	"aladin/backend_v2/internal/graph"
	"aladin/backend_v2/internal/graphpane"
	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/page"
	"aladin/backend_v2/internal/providerconnection"
	"aladin/backend_v2/internal/readingposition"
	"aladin/backend_v2/internal/realtime"
	"aladin/backend_v2/internal/record"
	"aladin/backend_v2/internal/relationship"
	"aladin/backend_v2/internal/research"
	"aladin/backend_v2/internal/search"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/source"
	"aladin/backend_v2/internal/system"
	"aladin/backend_v2/internal/unfurl"
	"aladin/backend_v2/internal/watchlist"
	"aladin/backend_v2/internal/workspacesync"
)

// APIComponents is the process-owned graph consumed by cmd/api. The HTTP
// package owns its narrower request-serving interface; this contract also
// exposes the singleton loops whose lifecycle belongs to the API process.
type APIComponents interface {
	Auth() auth.AuthService
	System() system.SystemService
	Sources() source.SourceService
	Records() record.RecordService
	Artifacts() artifact.ArtifactService
	Pages() page.Service
	Files() file.FileService
	Feed() feed.FeedService
	Insights() insights.InsightService
	ProviderConnections() providerconnection.ProviderConnectionService
	Realtime() realtime.EventService
	RealtimeKeyResolver() realtime.KeyResolver
	Sync() workspacesync.SyncService
	OutboxDrainer() *changefeed.Drainer
	DocSurfaceStore() coreservice.DocSurfaceStore
	WorkspaceRuntime() coreservice.WorkspaceRuntime
	ShardBuild() coreservice.ShardBuildService
	ShardResources() shardresource.Service
	ShardGraphQL() coreservice.ShardGraphQLService
	ShardReleases() coreservice.ShardReleaseService
	ShardKV() coreservice.ShardKVService
	ShardBridge() coreservice.ShardBridgeService
	Relationships() relationship.RelationshipService
	Research() research.ResearchService
	Documents() document.DocumentService
	GraphPane() graphpane.GraphPaneService
	EntityTags() entities.EntityTagService
	ArtifactRefs() artifactref.ArtifactRefService
	EntityContext() entities.EntityContextService
	EntityList() entities.EntityListService
	Instruments() instrument.InstrumentService
	Watchlist() watchlist.Service
	ReadingPositions() readingposition.Service
	Search() search.SearchService
	Unfurl() unfurl.UnfurlService
	Bars() market.BarService
	Alerts() alert.AlertService
	Notifications() alert.NotificationService
	AlertEngine() *alert.AlertEngine
	MarketData() market.MarketDataService
	GraphReader() graph.GraphReader
	Copilot() copilot.CopilotService
}

// MCPComponents is the process-owned graph consumed by cmd/mcp. It deliberately
// excludes API lifecycle loops and HTTP-only services.
type MCPComponents interface {
	Auth() auth.AuthService
	Artifacts() artifact.ArtifactService
	PageDocuments() page.DocumentService
	Insights() insights.InsightService
	DocSurfaceStore() coreservice.DocSurfaceStore
	Preview() coreservice.PreviewService
	ShardBuild() coreservice.ShardBuildService
	ShardResources() shardresource.Service
	ShardGraphQL() coreservice.ShardGraphQLService
	ShardReleases() coreservice.ShardReleaseService
	ShardCatalog() coreservice.ShardCatalogService
	ShardBridge() coreservice.ShardBridgeService
	Documents() document.DocumentService
	EntityTags() entities.EntityTagService
	ArtifactRefs() artifactref.ArtifactRefService
	EntityContext() entities.EntityContextService
	Instruments() instrument.InstrumentService
	Watchlist() watchlist.Service
	Search() search.SearchService
	Bars() market.BarService
	QuoteSnapshots() market.QuoteSnapshotSource
	MarketInfo() market.MarketInfoService
	Alerts() alert.AlertService
}
