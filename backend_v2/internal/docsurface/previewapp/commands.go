package previewapp

import (
	"context"

	"aladin/backend_v2/internal/docsurface/authoring"
	"aladin/backend_v2/internal/service"
)

// PreviewCommands is the authorization-aware application boundary for the
// ephemeral renderer. PreviewService remains the sole lifecycle owner.
type PreviewCommands struct {
	authoring *authoring.Authoring
	preview   service.PreviewService
}

func New(artifacts service.ArtifactService, store service.DocSurfaceStore, build service.ShardBuildService, preview service.PreviewService) *PreviewCommands {
	return &PreviewCommands{authoring: authoring.NewAuthoring(artifacts, store, build), preview: preview}
}

func (p *PreviewCommands) Open(ctx context.Context, pageID string, channel service.BuildChannel, theme string) (service.PreviewState, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return service.PreviewState{}, err
	}
	return p.preview.Open(ctx, pageID, channel, service.PreviewOpenOptions{Theme: theme})
}

func (p *PreviewCommands) Navigate(ctx context.Context, pageID, route string) (service.PreviewState, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return service.PreviewState{}, err
	}
	return p.preview.Navigate(ctx, pageID, route)
}

func (p *PreviewCommands) Snapshot(ctx context.Context, pageID string) (service.PreviewState, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return service.PreviewState{}, err
	}
	return p.preview.Snapshot(ctx, pageID)
}

func (p *PreviewCommands) Screenshot(ctx context.Context, pageID string) ([]byte, service.PreviewState, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return nil, service.PreviewState{}, err
	}
	return p.preview.Screenshot(ctx, pageID)
}

func (p *PreviewCommands) Eval(ctx context.Context, pageID, expression string) (service.PreviewState, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return service.PreviewState{}, err
	}
	return p.preview.Eval(ctx, pageID, expression)
}

func (p *PreviewCommands) Click(ctx context.Context, pageID, selector string) (service.PreviewState, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return service.PreviewState{}, err
	}
	return p.preview.Click(ctx, pageID, selector)
}

func (p *PreviewCommands) Console(ctx context.Context, pageID string) (service.PreviewState, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return service.PreviewState{}, err
	}
	return p.preview.Console(ctx, pageID)
}

func (p *PreviewCommands) Close(ctx context.Context, pageID string) error {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return err
	}
	return p.preview.Close(ctx, pageID)
}

func (p *PreviewCommands) Restart(ctx context.Context, pageID string) error {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return err
	}
	return p.preview.Reset(ctx)
}
