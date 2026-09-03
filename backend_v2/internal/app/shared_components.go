package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/entities"
	"aladin/backend_v2/internal/market/alpaca"
	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/shardv2"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// sharedComponents contains only objects that are required by both the API and
// MCP graphs. Process-specific services are constructed in api_components.go
// and mcp_components.go, so creating MCP cannot accidentally create API loops.
type sharedComponents struct {
	auth             coreservice.AuthService
	artifacts        coreservice.ArtifactService
	insights         coreservice.InsightService
	docSurfaceStore  coreservice.DocSurfaceStore
	workspaceRuntime coreservice.WorkspaceRuntime
	shardBuild       coreservice.ShardBuildService
	shardResources   shardresource.Service
	shardGraphQL     coreservice.ShardGraphQLService
	shardReleases    coreservice.ShardReleaseService
	shardBridge      coreservice.ShardBridgeService
	documents        coreservice.DocumentService
	entityTags       coreservice.EntityTagService
	artifactRefs     coreservice.ArtifactRefService
	entityContext    coreservice.EntityContextService
	instruments      coreservice.InstrumentService
	watchlist        coreservice.WatchlistService
	search           coreservice.SearchService
	bars             coreservice.BarService
	alerts           coreservice.AlertService

	recordRepo     coreservice.RecordRepository
	artifactRepo   *repo.PostgresArtifactRepository
	artifactFiles  coreservice.ArtifactFileStore
	research       coreservice.ResearchService
	quoteSnapshots coreservice.QuoteSnapshotSource
	marketInfo     coreservice.MarketInfoService
	alertRepo      coreservice.AlertRepository
	alpacaConfig   config.AlpacaConfig
	shardStorage   *repo.ShardResourcePostgres
}

func buildSharedComponents(pool *pgxpool.Pool, dataVolumePath string) sharedComponents {
	recordRepo := repo.NewRecordPostgres(pool)
	artifactRepo := repo.NewArtifactsPostgres(pool)
	artifactFiles := NewArtifactFileStore()
	docStore := docsurface.NewStore(dataVolumePath)

	storage := repo.NewShardResourcePostgres(pool, repo.ShardResourceLimits{})
	var profiles shardv2.Registry
	var ownedStorage shardresource.Provider = storage
	releaseSvc := coreservice.NewShardReleaseService(storage, nil)
	if os.Getenv("SHARD_V2_ENABLED") == "1" {
		engine := strings.ToLower(strings.TrimSpace(os.Getenv("SHARD_DATASTORE")))
		mongoURI := strings.TrimSpace(os.Getenv("SHARD_MONGODB_URI"))
		if engine == "" {
			engine = "mongo"
		}
		if engine == "mongo" {
			if mongoURI == "" {
				panic("SHARD_DATASTORE=mongo requires SHARD_MONGODB_URI")
			}
			client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
			if err != nil {
				panic(fmt.Sprintf("configure shard MongoDB: %v", err))
			}
			ownedStorage = repo.NewShardResourceMongo(client, os.Getenv("SHARD_MONGODB_DATABASE"), repo.ShardResourceLimits{})
			slog.Info("shard v2: configured owned datastore", "engine", "mongo")
		} else if engine != "postgres" {
			panic("SHARD_DATASTORE must be mongo or postgres")
		}
		profiles = shardv2.Registry{
			"shard.documents": ownedStorage.Profile(),
			"workspace.nodes": coreservice.NewWorkspaceResourceProvider(nil).Profile(),
		}
	}

	docRuntime := docsurface.NewBuilder(docStore, filepath.Join(dataVolumePath, "cache", "esm"), profiles)
	researchSvc := coreservice.NewResearchService(repo.NewResearchPostgres(pool))
	insightsSvc := coreservice.NewInsightService(repo.NewInsightPostgres(pool))
	entityTagsSvc := coreservice.NewEntityTagService(repo.NewEntityTagPostgres(pool), entities.Normalize)
	artifactRefsSvc := coreservice.NewArtifactRefService(repo.NewArtifactRefPostgres(pool))
	contentIndexSvc := coreservice.NewContentIndexService(repo.NewContentIndexPostgres(pool))

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

	instrumentsSvc := coreservice.NewInstrumentService(repo.NewInstrumentPostgres(pool))
	if assetLookup != nil {
		instrumentsSvc = instrumentsSvc.WithAssetLookup(assetLookup)
	}
	barsSvc := coreservice.NewBarService(repo.NewBarPostgres(pool)).
		WithCorporateActions(repo.NewCorporateActionPostgres(pool))
	if barSource != nil {
		barsSvc = barsSvc.WithSource(barSource)
	}
	searchSvc := coreservice.NewSearchService(
		coreservice.NewInstrumentSearchProvider(instrumentsSvc),
		coreservice.NewEntitySearchProvider(entityTagsSvc),
		coreservice.NewArtifactSearchProvider(artifactRefsSvc),
		coreservice.NewContentSearchProvider(contentIndexSvc),
	)

	artifactsSvc := coreservice.NewArtifactService(artifactRepo, artifactFiles)
	watchlistSvc := coreservice.NewWatchlistService(repo.NewWatchlistPostgres(pool))
	alertRepo := repo.NewAlertsPostgres(pool)
	alertsSvc := coreservice.NewAlertService(alertRepo, instrumentsSvc, snapshotSource)
	entityContextSvc := coreservice.NewEntityContextService(
		repo.NewEntityContextPostgres(pool),
		db.NewEntityRepository(pool),
	)

	var resourceSvc shardresource.Service
	if os.Getenv("SHARD_V2_ENABLED") == "1" {
		workspace := coreservice.NewWorkspaceResourceProvider(coreservice.NewEntityRegistry(
			coreservice.NewArtifactEntityService(artifactsSvc),
			coreservice.NewRecordEntityService(recordRepo),
			coreservice.NewWatchlistEntityService(watchlistSvc),
			coreservice.NewResearchEntityService(researchSvc),
		))
		validators := []coreservice.ResourceStageValidator{workspace.(coreservice.ResourceStageValidator)}
		if validator, ok := ownedStorage.(coreservice.ResourceStageValidator); ok {
			validators = append(validators, validator)
		}
		releaseSvc = coreservice.NewShardReleaseService(storage, profiles, validators...)
		resourceSvc = coreservice.NewShardResourceService(artifactsSvc, storage, map[string]shardresource.Provider{
			"shard.documents": ownedStorage,
			"workspace.nodes": workspace,
		}, coreservice.ResourceServiceOptions{})
	}

	var graphQLSvc coreservice.ShardGraphQLService
	if runtimeURL, runtimeSecret := strings.TrimSpace(os.Getenv("SHARD_RUNTIME_URL")), os.Getenv("SHARD_RUNTIME_SECRET"); resourceSvc != nil && runtimeURL != "" {
		graphQLSvc = coreservice.NewShardGraphQLService(releaseSvc, resourceSvc, runtimeURL, runtimeSecret)
		if !graphQLSvc.Enabled() {
			panic("SHARD_RUNTIME_SECRET must contain at least 32 bytes")
		}
	}

	shardBuild := coreservice.NewShardBuildService(docRuntime, repo.NewShardBuildPostgres(pool), releaseSvc)
	shardBridge := coreservice.NewShardBridgeService(artifactsSvc, docStore,
		coreservice.NewEntityRegistry(
			coreservice.NewArtifactEntityService(artifactsSvc),
			coreservice.NewRecordEntityService(recordRepo),
			coreservice.NewWatchlistEntityService(watchlistSvc),
			coreservice.NewResearchEntityService(researchSvc),
		))

	return sharedComponents{
		auth:             coreservice.NewAuthService(repo.NewAuthPostgres(pool), coreservice.NewPasswordHasher()),
		artifacts:        artifactsSvc,
		insights:         insightsSvc,
		docSurfaceStore:  docStore,
		workspaceRuntime: docRuntime,
		shardBuild:       shardBuild,
		shardResources:   resourceSvc,
		shardGraphQL:     graphQLSvc,
		shardReleases:    releaseSvc,
		shardBridge:      shardBridge,
		documents:        coreservice.NewDocumentService(repo.NewDocumentPostgres(pool, artifactFiles)),
		entityTags:       entityTagsSvc,
		artifactRefs:     artifactRefsSvc,
		entityContext:    entityContextSvc,
		instruments:      instrumentsSvc,
		watchlist:        watchlistSvc,
		search:           searchSvc,
		bars:             barsSvc,
		alerts:           alertsSvc,
		recordRepo:       recordRepo,
		artifactRepo:     artifactRepo,
		artifactFiles:    artifactFiles,
		research:         researchSvc,
		quoteSnapshots:   snapshotSource,
		marketInfo:       marketInfo,
		alertRepo:        alertRepo,
		alpacaConfig:     alpacaCfg,
		shardStorage:     storage,
	}
}

// NewArtifactFileStore is shared with worker ingestion so every process resolves
// uploaded resources using the same directory convention.
func NewArtifactFileStore() *repo.FilesystemArtifactStore {
	return repo.NewFilesystemArtifactStore(uploadDir(), audioDir())
}

func uploadDir() string { return filepath.Join(".", "uploads") }
func audioDir() string  { return filepath.Join(".", "audio") }
