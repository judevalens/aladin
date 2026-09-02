package api

import (
	coreservice "aladin/backend_v2/internal/service"
)

// testDependencies is the API package's test-only implementation of its own
// consumer contract. Keeping it here prevents production composition from
// becoming a shared service locator just to make handler tests convenient.
type testDependencies struct {
	AuthSvc                coreservice.AuthService
	SystemSvc              coreservice.SystemService
	SourcesSvc             coreservice.SourceService
	RecordsSvc             coreservice.RecordService
	ArtifactsSvc           coreservice.ArtifactService
	PagesSvc               coreservice.PageService
	FilesSvc               coreservice.FileService
	FeedSvc                coreservice.FeedService
	InsightsSvc            coreservice.InsightService
	ProviderConnectionsSvc coreservice.ProviderConnectionService
	RealtimeSvc            coreservice.RealtimeEventService
	RealtimeKeys           coreservice.SubscriptionKeyResolver
	SyncSvc                coreservice.SyncService
	DocSurfaceStoreSvc     coreservice.DocSurfaceStore
	WorkspaceRuntimeSvc    coreservice.WorkspaceRuntime
	ShardBuildSvc          coreservice.ShardBuildService
	ShardResourceSvc       coreservice.ShardResourceService
	ShardGraphQLSvc        coreservice.ShardGraphQLService
	ShardReleaseSvc        coreservice.ShardReleaseService
	ShardKVSvc             coreservice.ShardKVService
	ShardBridgeSvc         coreservice.ShardBridgeService
	RelationshipsSvc       coreservice.RelationshipService
	ResearchSvc            coreservice.ResearchService
	DocumentsSvc           coreservice.DocumentService
	GraphPaneSvc           coreservice.GraphPaneService
	EntityTagsSvc          coreservice.EntityTagService
	ArtifactRefsSvc        coreservice.ArtifactRefService
	EntityContextSvc       coreservice.EntityContextService
	EntityListSvc          coreservice.EntityListService
	InstrumentsSvc         coreservice.InstrumentService
	WatchlistSvc           coreservice.WatchlistService
	ReadingPositionsSvc    coreservice.ReadingPositionService
	SearchSvc              coreservice.SearchService
	UnfurlSvc              coreservice.UnfurlService
	BarsSvc                coreservice.BarService
	AlertsSvc              coreservice.AlertService
	NotificationsSvc       coreservice.NotificationService
	MarketDataSvc          coreservice.MarketDataService
	GraphReaderSvc         coreservice.GraphReader
	CopilotSvc             coreservice.CopilotService
}

func (d testDependencies) Auth() coreservice.AuthService          { return d.AuthSvc }
func (d testDependencies) System() coreservice.SystemService      { return d.SystemSvc }
func (d testDependencies) Sources() coreservice.SourceService     { return d.SourcesSvc }
func (d testDependencies) Records() coreservice.RecordService     { return d.RecordsSvc }
func (d testDependencies) Artifacts() coreservice.ArtifactService { return d.ArtifactsSvc }
func (d testDependencies) Pages() coreservice.PageService         { return d.PagesSvc }
func (d testDependencies) Files() coreservice.FileService         { return d.FilesSvc }
func (d testDependencies) Feed() coreservice.FeedService          { return d.FeedSvc }
func (d testDependencies) Insights() coreservice.InsightService   { return d.InsightsSvc }
func (d testDependencies) ProviderConnections() coreservice.ProviderConnectionService {
	return d.ProviderConnectionsSvc
}
func (d testDependencies) Realtime() coreservice.RealtimeEventService { return d.RealtimeSvc }
func (d testDependencies) RealtimeKeyResolver() coreservice.SubscriptionKeyResolver {
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
func (d testDependencies) Relationships() coreservice.RelationshipService  { return d.RelationshipsSvc }
func (d testDependencies) Research() coreservice.ResearchService           { return d.ResearchSvc }
func (d testDependencies) Documents() coreservice.DocumentService          { return d.DocumentsSvc }
func (d testDependencies) GraphPane() coreservice.GraphPaneService         { return d.GraphPaneSvc }
func (d testDependencies) EntityTags() coreservice.EntityTagService        { return d.EntityTagsSvc }
func (d testDependencies) ArtifactRefs() coreservice.ArtifactRefService    { return d.ArtifactRefsSvc }
func (d testDependencies) EntityContext() coreservice.EntityContextService { return d.EntityContextSvc }
func (d testDependencies) EntityList() coreservice.EntityListService       { return d.EntityListSvc }
func (d testDependencies) Instruments() coreservice.InstrumentService      { return d.InstrumentsSvc }
func (d testDependencies) Watchlist() coreservice.WatchlistService         { return d.WatchlistSvc }
func (d testDependencies) ReadingPositions() coreservice.ReadingPositionService {
	return d.ReadingPositionsSvc
}
func (d testDependencies) Search() coreservice.SearchService              { return d.SearchSvc }
func (d testDependencies) Unfurl() coreservice.UnfurlService              { return d.UnfurlSvc }
func (d testDependencies) Bars() coreservice.BarService                   { return d.BarsSvc }
func (d testDependencies) Alerts() coreservice.AlertService               { return d.AlertsSvc }
func (d testDependencies) Notifications() coreservice.NotificationService { return d.NotificationsSvc }
func (d testDependencies) MarketData() coreservice.MarketDataService      { return d.MarketDataSvc }
func (d testDependencies) GraphReader() coreservice.GraphReader           { return d.GraphReaderSvc }
func (d testDependencies) Copilot() coreservice.CopilotService            { return d.CopilotSvc }

var _ Dependencies = testDependencies{}
