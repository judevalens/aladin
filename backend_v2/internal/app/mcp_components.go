package app

import (
	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/docsurface"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MCPProcess struct {
	sharedComponents
	pageDocuments coreservice.PageDocumentService
	preview       coreservice.PreviewService
	shardCatalog  coreservice.ShardCatalogService
}

func NewMCPComponents(pool *pgxpool.Pool) *MCPProcess {
	return NewMCPComponentsWithDataVolume(pool, config.DataVolumePathOrDefault())
}

func NewMCPComponentsWithDataVolume(pool *pgxpool.Pool, dataVolumePath string) *MCPProcess {
	shared := buildSharedComponents(pool, dataVolumePath)
	var catalog coreservice.ShardCatalogService
	if shared.shardResources != nil {
		catalog = coreservice.NewShardCatalogService(shared.shardStorage, shared.shardResources)
	}
	preview := docsurface.NewPreviewSessions(
		shared.docSurfaceStore,
		shared.workspaceRuntime,
		docsurface.PreviewOptions{
			Builder:   shared.shardBuild,
			Resources: shared.shardResources,
			Releases:  shared.shardReleases,
			GraphQL:   shared.shardGraphQL,
		},
	)
	return &MCPProcess{
		sharedComponents: shared,
		pageDocuments:    coreservice.NewPageDocumentService(shared.artifactRepo),
		preview:          preview,
		shardCatalog:     catalog,
	}
}

func (c *MCPProcess) Auth() coreservice.AuthService                    { return c.auth }
func (c *MCPProcess) Artifacts() coreservice.ArtifactService           { return c.artifacts }
func (c *MCPProcess) PageDocuments() coreservice.PageDocumentService   { return c.pageDocuments }
func (c *MCPProcess) Insights() coreservice.InsightService             { return c.insights }
func (c *MCPProcess) DocSurfaceStore() coreservice.DocSurfaceStore     { return c.docSurfaceStore }
func (c *MCPProcess) Preview() coreservice.PreviewService              { return c.preview }
func (c *MCPProcess) ShardBuild() coreservice.ShardBuildService        { return c.shardBuild }
func (c *MCPProcess) ShardResources() coreservice.ShardResourceService { return c.shardResources }
func (c *MCPProcess) ShardGraphQL() coreservice.ShardGraphQLService    { return c.shardGraphQL }
func (c *MCPProcess) ShardReleases() coreservice.ShardReleaseService   { return c.shardReleases }
func (c *MCPProcess) ShardCatalog() coreservice.ShardCatalogService    { return c.shardCatalog }
func (c *MCPProcess) ShardBridge() coreservice.ShardBridgeService      { return c.shardBridge }
func (c *MCPProcess) Documents() coreservice.DocumentService           { return c.documents }
func (c *MCPProcess) EntityTags() coreservice.EntityTagService         { return c.entityTags }
func (c *MCPProcess) ArtifactRefs() coreservice.ArtifactRefService     { return c.artifactRefs }
func (c *MCPProcess) EntityContext() coreservice.EntityContextService  { return c.entityContext }
func (c *MCPProcess) Instruments() coreservice.InstrumentService       { return c.instruments }
func (c *MCPProcess) Watchlist() coreservice.WatchlistService          { return c.watchlist }
func (c *MCPProcess) Search() coreservice.SearchService                { return c.search }
func (c *MCPProcess) Bars() coreservice.BarService                     { return c.bars }
func (c *MCPProcess) QuoteSnapshots() coreservice.QuoteSnapshotSource  { return c.quoteSnapshots }
func (c *MCPProcess) MarketInfo() coreservice.MarketInfoService        { return c.marketInfo }
func (c *MCPProcess) Alerts() coreservice.AlertService                 { return c.alerts }
