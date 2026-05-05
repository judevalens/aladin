package app

import (
	"path/filepath"

	"aladin/backend_v2/internal/repo"
	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies interface {
	System() coreservice.SystemService
	Sources() coreservice.SourceService
	Records() coreservice.RecordService
	Artifacts() coreservice.ArtifactService
	Pages() coreservice.PageService
	Files() coreservice.FileService
	Feed() coreservice.FeedService
	Insights() coreservice.InsightService
	Realtime() coreservice.RealtimeEventService
	RealtimeKeyResolver() coreservice.SubscriptionKeyResolver
}

type StaticDependencies struct {
	SystemSvc    coreservice.SystemService
	SourcesSvc   coreservice.SourceService
	RecordsSvc   coreservice.RecordService
	ArtifactsSvc coreservice.ArtifactService
	PagesSvc     coreservice.PageService
	FilesSvc     coreservice.FileService
	FeedSvc      coreservice.FeedService
	InsightsSvc  coreservice.InsightService
	RealtimeSvc  coreservice.RealtimeEventService
	RealtimeKeys coreservice.SubscriptionKeyResolver
}

func (d StaticDependencies) System() coreservice.SystemService      { return d.SystemSvc }
func (d StaticDependencies) Sources() coreservice.SourceService     { return d.SourcesSvc }
func (d StaticDependencies) Records() coreservice.RecordService     { return d.RecordsSvc }
func (d StaticDependencies) Artifacts() coreservice.ArtifactService { return d.ArtifactsSvc }
func (d StaticDependencies) Pages() coreservice.PageService         { return d.PagesSvc }
func (d StaticDependencies) Files() coreservice.FileService         { return d.FilesSvc }
func (d StaticDependencies) Feed() coreservice.FeedService          { return d.FeedSvc }
func (d StaticDependencies) Insights() coreservice.InsightService   { return d.InsightsSvc }
func (d StaticDependencies) Realtime() coreservice.RealtimeEventService {
	return d.RealtimeSvc
}
func (d StaticDependencies) RealtimeKeyResolver() coreservice.SubscriptionKeyResolver {
	return d.RealtimeKeys
}

type wiring struct {
	system    coreservice.SystemService
	sources   coreservice.SourceService
	records   coreservice.RecordService
	artifacts coreservice.ArtifactService
	pages     coreservice.PageService
	files     coreservice.FileService
	feed      coreservice.FeedService
	insights  coreservice.InsightService
	realtime  coreservice.RealtimeEventService
	rtKeys    coreservice.SubscriptionKeyResolver
}

func (w wiring) System() coreservice.SystemService      { return w.system }
func (w wiring) Sources() coreservice.SourceService     { return w.sources }
func (w wiring) Records() coreservice.RecordService     { return w.records }
func (w wiring) Artifacts() coreservice.ArtifactService { return w.artifacts }
func (w wiring) Pages() coreservice.PageService         { return w.pages }
func (w wiring) Files() coreservice.FileService         { return w.files }
func (w wiring) Feed() coreservice.FeedService          { return w.feed }
func (w wiring) Insights() coreservice.InsightService   { return w.insights }
func (w wiring) Realtime() coreservice.RealtimeEventService {
	return w.realtime
}
func (w wiring) RealtimeKeyResolver() coreservice.SubscriptionKeyResolver {
	return w.rtKeys
}

const defaultUserID = "00000000-0000-0000-0000-000000000001"

func NewDependencies(pool *pgxpool.Pool) Dependencies {
	sourceRepo := repo.NewSourcePostgres(pool)
	recordRepo := repo.NewRecordPostgres(pool)
	artifactRepo := repo.NewArtifactsPostgres(pool, defaultUserID)
	artifactFiles := repo.NewFilesystemArtifactStore(uploadDir(), audioDir())
	feedRepo := repo.NewFeedPostgres(pool)
	insightRepo := repo.NewInsightPostgres(pool)
	systemRepo := repo.NewSystemPostgres(pool)
	realtimeKeys := coreservice.NewSubscriptionKeyResolver(defaultUserID)
	realtime := coreservice.NewInMemoryRealtimeEventService(realtimeKeys)

	return wiring{
		system:    coreservice.NewSystemService(systemRepo),
		sources:   coreservice.NewSourceService(sourceRepo),
		records:   coreservice.NewRecordService(recordRepo),
		artifacts: coreservice.NewArtifactService(artifactRepo, artifactFiles, realtime),
		pages:     coreservice.NewPageService(artifactRepo, realtime),
		files:     coreservice.NewFileService(artifactRepo, artifactFiles),
		feed:      coreservice.NewFeedService(feedRepo),
		insights:  coreservice.NewInsightService(insightRepo),
		realtime:  realtime,
		rtKeys:    realtimeKeys,
	}
}

func uploadDir() string {
	return filepath.Join(".", "uploads")
}

func audioDir() string {
	return filepath.Join(".", "audio")
}
