package api

import (
	"aladin/backend_v2/internal/alert"
	"aladin/backend_v2/internal/artifactref"
	"aladin/backend_v2/internal/document"
	"aladin/backend_v2/internal/feed"
	"aladin/backend_v2/internal/graphpane"
	"aladin/backend_v2/internal/insights"
	"aladin/backend_v2/internal/instrument"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/providerconnection"
	"aladin/backend_v2/internal/readingposition"
	"aladin/backend_v2/internal/realtime"
	"aladin/backend_v2/internal/record"
	"aladin/backend_v2/internal/relationship"
	"aladin/backend_v2/internal/research"
	"aladin/backend_v2/internal/search"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/source"
	"aladin/backend_v2/internal/unfurl"
	"aladin/backend_v2/internal/watchlist"
)

// testDependencies is the API package's test-only implementation of its own
// consumer contract. Keeping it here prevents production composition from
// becoming a shared service locator just to make handler tests convenient.
type testDependencies struct {
	AuthSvc                coreservice.AuthService
	SystemSvc              coreservice.SystemService
	SourcesSvc             source.SourceService
	RecordsSvc             record.RecordService
	ArtifactsSvc           coreservice.ArtifactService
	PagesSvc               coreservice.PageService
	FilesSvc               coreservice.FileService
	FeedSvc                feed.FeedService
	InsightsSvc            insights.InsightService
	ProviderConnectionsSvc providerconnection.ProviderConnectionService
	RealtimeSvc            realtime.EventService
	RealtimeKeys           realtime.KeyResolver
	SyncSvc                coreservice.SyncService
	DocSurfaceStoreSvc     coreservice.DocSurfaceStore
	WorkspaceRuntimeSvc    coreservice.WorkspaceRuntime
	ShardBuildSvc          coreservice.ShardBuildService
	ShardResourceSvc       coreservice.ShardResourceService
	ShardGraphQLSvc        coreservice.ShardGraphQLService
	ShardReleaseSvc        coreservice.ShardReleaseService
	ShardKVSvc             coreservice.ShardKVService
	ShardBridgeSvc         coreservice.ShardBridgeService
	RelationshipsSvc       relationship.RelationshipService
	ResearchSvc            research.ResearchService
	DocumentsSvc           document.DocumentService
	GraphPaneSvc           graphpane.GraphPaneService
	EntityTagsSvc          coreservice.EntityTagService
	ArtifactRefsSvc        artifactref.ArtifactRefService
	EntityContextSvc       coreservice.EntityContextService
	EntityListSvc          coreservice.EntityListService
	InstrumentsSvc         instrument.InstrumentService
	WatchlistSvc           watchlist.Service
	ReadingPositionsSvc    readingposition.Service
	SearchSvc              search.SearchService
	UnfurlSvc              unfurl.UnfurlService
	BarsSvc                market.BarService
	AlertsSvc              alert.AlertService
	NotificationsSvc       alert.NotificationService
	MarketDataSvc          market.MarketDataService
	GraphReaderSvc         coreservice.GraphReader
	CopilotSvc             coreservice.CopilotService
}

func (d testDependencies) Auth() coreservice.AuthService          { return d.AuthSvc }
func (d testDependencies) System() coreservice.SystemService      { return d.SystemSvc }
func (d testDependencies) Sources() source.SourceService          { return d.SourcesSvc }
func (d testDependencies) Records() record.RecordService          { return d.RecordsSvc }
func (d testDependencies) Artifacts() coreservice.ArtifactService { return d.ArtifactsSvc }
func (d testDependencies) Pages() coreservice.PageService         { return d.PagesSvc }
func (d testDependencies) Files() coreservice.FileService         { return d.FilesSvc }
func (d testDependencies) Feed() feed.FeedService                 { return d.FeedSvc }
func (d testDependencies) Insights() insights.InsightService      { return d.InsightsSvc }
func (d testDependencies) ProviderConnections() providerconnection.ProviderConnectionService {
	return d.ProviderConnectionsSvc
}
func (d testDependencies) Realtime() realtime.EventService { return d.RealtimeSvc }
func (d testDependencies) RealtimeKeyResolver() realtime.KeyResolver {
	return d.RealtimeKeys
}
func (d testDependencies) Sync() coreservice.SyncService                { return d.SyncSvc }
func (d testDependencies) DocSurfaceStore() coreservice.DocSurfaceStore { return d.DocSurfaceStoreSvc }
func (d testDependencies) WorkspaceRuntime() coreservice.WorkspaceRuntime {
	return d.WorkspaceRuntimeSvc
}
func (d testDependencies) ShardBuild() coreservice.ShardBuildService { return d.ShardBuildSvc }
func (d testDependencies) ShardResources() coreservice.ShardResourceService {
	return d.ShardResourceSvc
}
func (d testDependencies) ShardGraphQL() coreservice.ShardGraphQLService   { return d.ShardGraphQLSvc }
func (d testDependencies) ShardReleases() coreservice.ShardReleaseService  { return d.ShardReleaseSvc }
func (d testDependencies) ShardKV() coreservice.ShardKVService             { return d.ShardKVSvc }
func (d testDependencies) ShardBridge() coreservice.ShardBridgeService     { return d.ShardBridgeSvc }
func (d testDependencies) Relationships() relationship.RelationshipService { return d.RelationshipsSvc }
func (d testDependencies) Research() research.ResearchService              { return d.ResearchSvc }
func (d testDependencies) Documents() document.DocumentService             { return d.DocumentsSvc }
func (d testDependencies) GraphPane() graphpane.GraphPaneService           { return d.GraphPaneSvc }
func (d testDependencies) EntityTags() coreservice.EntityTagService        { return d.EntityTagsSvc }
func (d testDependencies) ArtifactRefs() artifactref.ArtifactRefService    { return d.ArtifactRefsSvc }
func (d testDependencies) EntityContext() coreservice.EntityContextService { return d.EntityContextSvc }
func (d testDependencies) EntityList() coreservice.EntityListService       { return d.EntityListSvc }
func (d testDependencies) Instruments() instrument.InstrumentService       { return d.InstrumentsSvc }
func (d testDependencies) Watchlist() watchlist.Service                    { return d.WatchlistSvc }
func (d testDependencies) ReadingPositions() readingposition.Service {
	return d.ReadingPositionsSvc
}
func (d testDependencies) Search() search.SearchService             { return d.SearchSvc }
func (d testDependencies) Unfurl() unfurl.UnfurlService             { return d.UnfurlSvc }
func (d testDependencies) Bars() market.BarService                  { return d.BarsSvc }
func (d testDependencies) Alerts() alert.AlertService               { return d.AlertsSvc }
func (d testDependencies) Notifications() alert.NotificationService { return d.NotificationsSvc }
func (d testDependencies) MarketData() market.MarketDataService     { return d.MarketDataSvc }
func (d testDependencies) GraphReader() coreservice.GraphReader     { return d.GraphReaderSvc }
func (d testDependencies) Copilot() coreservice.CopilotService      { return d.CopilotSvc }

var _ Dependencies = testDependencies{}
