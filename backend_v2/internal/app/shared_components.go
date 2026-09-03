package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"aladin/backend_v2/internal/alert"
	alertpostgres "aladin/backend_v2/internal/alert/postgres"
	"aladin/backend_v2/internal/artifact"
	artifactpostgres "aladin/backend_v2/internal/artifact/postgres"
	"aladin/backend_v2/internal/artifactref"
	artifactrefpostgres "aladin/backend_v2/internal/artifactref/postgres"
	"aladin/backend_v2/internal/auth"
	authpostgres "aladin/backend_v2/internal/auth/postgres"
	"aladin/backend_v2/internal/config"
	"aladin/backend_v2/internal/db"
	"aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/document"
	documentpostgres "aladin/backend_v2/internal/document/postgres"
	"aladin/backend_v2/internal/entities"
	entitypostgres "aladin/backend_v2/internal/entities/postgres"
	"aladin/backend_v2/internal/file"
	"aladin/backend_v2/internal/insights"
	insightspostgres "aladin/backend_v2/internal/insights/postgres"
	"aladin/backend_v2/internal/instrument"
	instrumentpostgres "aladin/backend_v2/internal/instrument/postgres"
	"aladin/backend_v2/internal/market"
	"aladin/backend_v2/internal/market/alpaca"
	marketpostgres "aladin/backend_v2/internal/market/postgres"
	"aladin/backend_v2/internal/record"
	recordpostgres "aladin/backend_v2/internal/record/postgres"
	"aladin/backend_v2/internal/repo"
	"aladin/backend_v2/internal/research"
	researchpostgres "aladin/backend_v2/internal/research/postgres"
	"aladin/backend_v2/internal/search"
	searchpostgres "aladin/backend_v2/internal/search/postgres"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/shardresource"
	shardstorage "aladin/backend_v2/internal/shardresource/storage"
	"aladin/backend_v2/internal/shardv2"
	"aladin/backend_v2/internal/watchlist"
	watchlistpostgres "aladin/backend_v2/internal/watchlist/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// sharedComponents contains only objects that are required by both the API and
// MCP graphs. Process-specific services are constructed in api_components.go
// and mcp_components.go, so creating MCP cannot accidentally create API loops.
type sharedComponents struct {
	auth             auth.AuthService
	artifacts        artifact.ArtifactService
	insights         insights.InsightService
	docSurfaceStore  coreservice.DocSurfaceStore
	workspaceRuntime coreservice.WorkspaceRuntime
	shardBuild       coreservice.ShardBuildService
	shardResources   shardresource.Service
	shardGraphQL     coreservice.ShardGraphQLService
	shardReleases    coreservice.ShardReleaseService
	shardBridge      coreservice.ShardBridgeService
	documents        document.DocumentService
	entityTags       entities.EntityTagService
	artifactRefs     artifactref.ArtifactRefService
	entityContext    entities.EntityContextService
	instruments      instrument.InstrumentService
	watchlist        watchlist.Service
	search           search.SearchService
	bars             market.BarService
	alerts           alert.AlertService

	recordRepo     record.RecordRepository
	artifactRepo   *artifactpostgres.PostgresArtifactRepository
	artifactFiles  artifact.ArtifactFileStore
	research       research.ResearchService
	quoteSnapshots market.QuoteSnapshotSource
	marketInfo     market.MarketInfoService
	alertRepo      alert.AlertRepository
	alpacaConfig   config.AlpacaConfig
	shardStorage   *shardstorage.ShardResourcePostgres
}

func buildSharedComponents(pool *pgxpool.Pool, dataVolumePath string) sharedComponents {
	recordRepo := recordpostgres.NewRecordPostgres(pool)
	artifactRepo := artifactpostgres.NewArtifactsPostgres(pool)
	artifactFiles := NewArtifactFileStore()
	docStore := docsurface.NewStore(dataVolumePath)

	storage := shardstorage.NewShardResourcePostgres(pool, shardstorage.ShardResourceLimits{})
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
			ownedStorage = shardstorage.NewShardResourceMongo(client, os.Getenv("SHARD_MONGODB_DATABASE"), shardstorage.ShardResourceLimits{})
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
	researchSvc := research.NewResearchService(researchpostgres.NewResearchPostgres(pool))
	insightsSvc := insights.NewInsightService(insightspostgres.NewInsightPostgres(pool))
	entityTagsSvc := entities.NewEntityTagService(entitypostgres.NewEntityTagPostgres(pool), entities.Normalize)
	artifactRefsSvc := artifactref.NewArtifactRefService(artifactrefpostgres.NewArtifactRefPostgres(pool))
	contentIndexSvc := search.NewContentIndexService(searchpostgres.NewContentIndexPostgres(pool))

	alpacaCfg := config.LoadAlpaca()
	var barSource market.BarSource
	var assetLookup instrument.AssetLookup
	var snapshotSource market.QuoteSnapshotSource
	var marketInfo market.MarketInfoService
	if alpacaCfg.Configured() {
		restClient := alpaca.NewClient(alpacaCfg.APIKey, alpacaCfg.APISecret, alpacaCfg.TradingBaseURL, alpacaCfg.DataBaseURL)
		barSource = alpacaBarSource{c: restClient}
		assetLookup = alpacaAssetLookup{c: restClient}
		snapshotSource = alpacaSnapshotSource{c: restClient}
		marketInfo = alpacaMarketInfo{c: restClient, paper: strings.Contains(alpacaCfg.TradingBaseURL, "paper")}
	}

	instrumentsSvc := instrument.NewInstrumentService(instrumentpostgres.NewInstrumentPostgres(pool))
	if assetLookup != nil {
		instrumentsSvc = instrumentsSvc.WithAssetLookup(assetLookup)
	}
	barsSvc := market.NewBarService(marketpostgres.NewBarPostgres(pool)).
		WithCorporateActions(marketpostgres.NewCorporateActionPostgres(pool))
	if barSource != nil {
		barsSvc = barsSvc.WithSource(barSource)
	}
	searchSvc := search.NewSearchService(
		search.NewInstrumentSearchProvider(instrumentsSvc),
		search.NewEntitySearchProvider(entityTagsSvc),
		search.NewArtifactSearchProvider(artifactRefsSvc),
		search.NewContentSearchProvider(contentIndexSvc),
	)

	artifactsSvc := artifact.NewArtifactService(artifactRepo, artifactFiles)
	watchlistSvc := watchlist.NewService(watchlistpostgres.New(pool))
	alertRepo := alertpostgres.NewAlertsPostgres(pool)
	alertsSvc := alert.NewAlertService(alertRepo, instrumentsSvc, snapshotSource)
	entityContextSvc := entities.NewEntityContextService(
		entitypostgres.NewEntityContextPostgres(pool),
		db.NewEntityRepository(pool),
	)

	var resourceSvc shardresource.Service
	if os.Getenv("SHARD_V2_ENABLED") == "1" {
		workspace := coreservice.NewWorkspaceResourceProvider(coreservice.NewEntityRegistry(
			coreservice.NewArtifactEntityService(artifactsSvc),
			coreservice.NewRecordEntityService(recordRepo),
			coreservice.NewWatchlistEntityService(watchlistSvc),
			research.NewEntityService(researchSvc),
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
			research.NewEntityService(researchSvc),
		))

	return sharedComponents{
		auth:             auth.NewAuthService(authpostgres.NewAuthPostgres(pool), auth.NewPasswordHasher()),
		artifacts:        artifactsSvc,
		insights:         insightsSvc,
		docSurfaceStore:  docStore,
		workspaceRuntime: docRuntime,
		shardBuild:       shardBuild,
		shardResources:   resourceSvc,
		shardGraphQL:     graphQLSvc,
		shardReleases:    releaseSvc,
		shardBridge:      shardBridge,
		documents:        document.NewDocumentService(documentpostgres.NewDocumentPostgres(pool, artifactFiles)),
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
func NewArtifactFileStore() *file.FilesystemArtifactStore {
	return file.NewFilesystemArtifactStore(uploadDir(), audioDir())
}

func uploadDir() string { return filepath.Join(".", "uploads") }
func audioDir() string  { return filepath.Join(".", "audio") }
