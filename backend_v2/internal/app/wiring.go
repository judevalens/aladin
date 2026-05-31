package app

import (
	"path/filepath"

	"aladin/backend_v2/internal/config"
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
func (w wiring) Sync() coreservice.SyncService { return w.sync }
func (w wiring) OutboxDrainer() *coreservice.OutboxDrainer { return w.outboxDrainer }

func NewDependencies(pool *pgxpool.Pool) Dependencies {
	return NewDependenciesWithProviderConnections(pool, config.LoadProviderConnections())
}

func NewDependenciesWithProviderConnections(pool *pgxpool.Pool, providerConfig config.ProviderConnectionConfig) Dependencies {
	authRepo := repo.NewAuthPostgres(pool)
	sourceRepo := repo.NewSourcePostgres(pool)
	recordRepo := repo.NewRecordPostgres(pool)
	artifactRepo := repo.NewArtifactsPostgres(pool)
	artifactFiles := repo.NewFilesystemArtifactStore(uploadDir(), audioDir())
	feedRepo := repo.NewFeedPostgres(pool)
	insightRepo := repo.NewInsightPostgres(pool)
	systemRepo := repo.NewSystemPostgres(pool)
	providerConnectionRepo := repo.NewProviderConnectionPostgres(pool)
	syncRepo := repo.NewSyncPostgres(pool)
	syncSvc := coreservice.NewSyncService(syncRepo, repo.NewTreeSyncSource(pool))
	realtimeKeys := coreservice.NewSubscriptionKeyResolver()
	realtime := coreservice.NewInMemoryRealtimeEventService(realtimeKeys)
	outboxDrainer := coreservice.NewOutboxDrainer(syncRepo, realtime, 0)
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

	return wiring{
		auth:                coreservice.NewAuthService(authRepo, coreservice.NewPasswordHasher()),
		system:              coreservice.NewSystemService(systemRepo),
		sources:             coreservice.NewSourceService(sourceRepo),
		records:             coreservice.NewRecordService(recordRepo),
		artifacts:           coreservice.NewArtifactService(artifactRepo, artifactFiles, realtime),
		pages:               coreservice.NewPageService(artifactRepo, realtime),
		pageDocuments:       coreservice.NewPageDocumentService(artifactRepo),
		files:               coreservice.NewFileService(artifactRepo, artifactFiles),
		feed:                coreservice.NewFeedService(feedRepo),
		insights:            coreservice.NewInsightService(insightRepo),
		providerConnections: providerConnections,
		realtime:            realtime,
		rtKeys:              realtimeKeys,
		sync:                syncSvc,
		outboxDrainer:       outboxDrainer,
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

func uploadDir() string {
	return filepath.Join(".", "uploads")
}

func audioDir() string {
	return filepath.Join(".", "audio")
}
