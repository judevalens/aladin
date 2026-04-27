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
	Feed() coreservice.FeedService
	Insights() coreservice.InsightService
}

type StaticDependencies struct {
	SystemSvc    coreservice.SystemService
	SourcesSvc   coreservice.SourceService
	RecordsSvc   coreservice.RecordService
	ArtifactsSvc coreservice.ArtifactService
	FeedSvc      coreservice.FeedService
	InsightsSvc  coreservice.InsightService
}

func (d StaticDependencies) System() coreservice.SystemService      { return d.SystemSvc }
func (d StaticDependencies) Sources() coreservice.SourceService     { return d.SourcesSvc }
func (d StaticDependencies) Records() coreservice.RecordService     { return d.RecordsSvc }
func (d StaticDependencies) Artifacts() coreservice.ArtifactService { return d.ArtifactsSvc }
func (d StaticDependencies) Feed() coreservice.FeedService          { return d.FeedSvc }
func (d StaticDependencies) Insights() coreservice.InsightService   { return d.InsightsSvc }

type wiring struct {
	system    coreservice.SystemService
	sources   coreservice.SourceService
	records   coreservice.RecordService
	artifacts coreservice.ArtifactService
	feed      coreservice.FeedService
	insights  coreservice.InsightService
}

func (w wiring) System() coreservice.SystemService      { return w.system }
func (w wiring) Sources() coreservice.SourceService     { return w.sources }
func (w wiring) Records() coreservice.RecordService     { return w.records }
func (w wiring) Artifacts() coreservice.ArtifactService { return w.artifacts }
func (w wiring) Feed() coreservice.FeedService          { return w.feed }
func (w wiring) Insights() coreservice.InsightService   { return w.insights }

const defaultUserID = "00000000-0000-0000-0000-000000000001"

func NewDependencies(pool *pgxpool.Pool) Dependencies {
	sourceRepo := repo.NewSourcePostgres(pool)
	recordRepo := repo.NewRecordPostgres(pool)
	artifactRepo := repo.NewArtifactsPostgres(pool, defaultUserID)
	artifactFiles := repo.NewFilesystemArtifactStore(uploadDir(), audioDir())
	feedRepo := repo.NewFeedPostgres(pool)
	insightRepo := repo.NewInsightPostgres(pool)
	systemRepo := repo.NewSystemPostgres(pool)

	return wiring{
		system:    coreservice.NewSystemService(systemRepo),
		sources:   coreservice.NewSourceService(sourceRepo),
		records:   coreservice.NewRecordService(recordRepo),
		artifacts: coreservice.NewArtifactService(artifactRepo, artifactFiles),
		feed:      coreservice.NewFeedService(feedRepo),
		insights:  coreservice.NewInsightService(insightRepo),
	}
}

func uploadDir() string {
	return filepath.Join(".", "uploads")
}

func audioDir() string {
	return filepath.Join(".", "audio")
}
