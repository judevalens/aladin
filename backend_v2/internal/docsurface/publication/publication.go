package publication

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"

	docsurface "aladin/backend_v2/internal/docsurface"
	"aladin/backend_v2/internal/docsurface/authoring"
	"aladin/backend_v2/internal/docsurface/verification"
	"aladin/backend_v2/internal/service"
)

type PublishResult struct {
	OK        bool
	ServedURL string
	Verified  bool
	Warning   string
}

type AuthoringMode string

const (
	AuthoringLegacy      AuthoringMode = "legacy"
	AuthoringResources   AuthoringMode = "resources"
	AuthoringUnavailable AuthoringMode = "unavailable"
)

type AuthoringContext struct {
	Mode            AuthoringMode
	Contract        string
	ContractMissing bool
	Files           []string
	Anchors         string
	IndexTSX        string
}

// Publication owns verification-gated activation. The same exact published
// build is passed into verification and release activation, preventing a
// transport adapter from publishing stale or unchecked bytes.
type Publication struct {
	artifacts    service.ArtifactService
	store        service.DocSurfaceStore
	build        service.ShardBuildService
	bridge       service.ShardBridgeService
	releases     service.ShardReleaseService
	authoring    *authoring.Authoring
	verification *verification.Verification
}

func NewPublication(artifacts service.ArtifactService, store service.DocSurfaceStore, build service.ShardBuildService, preview service.PreviewService, bridge service.ShardBridgeService, releases service.ShardReleaseService) *Publication {
	return &Publication{
		artifacts: artifacts, store: store, build: build, bridge: bridge, releases: releases,
		authoring: authoring.NewAuthoring(artifacts, store, build), verification: verification.NewVerification(store, preview),
	}
}

func (p *Publication) Verify(ctx context.Context, pageID string, channel service.BuildChannel, strictConsole bool) (verification.VerificationReport, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return verification.VerificationReport{}, err
	}
	report, err := p.verification.Verify(ctx, pageID, channel, strictConsole, nil)
	if err != nil {
		return verification.VerificationReport{}, err
	}
	if p.bridge != nil {
		refs, err := p.bridge.CheckRefs(ctx, pageID)
		if err != nil {
			return verification.VerificationReport{}, err
		}
		report.Refs = &verification.ReferenceSummary{OK: refs.OK, Total: refs.Total, Missing: refs.Missing, UnknownKind: refs.UnknownKind, Unobservable: refs.Unobservable}
		if !refs.OK {
			report.OK = false
		}
	}
	return report, nil
}

func (p *Publication) AuthoringContext(ctx context.Context, pageID string) (AuthoringContext, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return AuthoringContext{}, err
	}
	out := AuthoringContext{Mode: AuthoringLegacy}
	contract, err := p.store.ReadFile(ctx, pageID, "contract.json")
	hasContract := err == nil
	if err != nil && !errors.Is(err, service.ErrNotFound) && !os.IsNotExist(err) {
		return AuthoringContext{}, err
	}
	if hasContract {
		out.Contract = string(contract)
		if p.releases == nil || !p.releases.Enabled() {
			out.Mode = AuthoringUnavailable
		} else {
			out.Mode = AuthoringResources
		}
	} else if p.releases != nil {
		for _, channel := range []service.BuildChannel{service.ChannelDraft, service.ChannelPublished} {
			release, err := p.releases.Active(ctx, pageID, channel)
			if err == nil {
				out.Mode, out.Contract, out.ContractMissing = AuthoringResources, string(release.Source), true
				break
			}
			if service.ResourceErrorCode(err) == "unsupported-capability" {
				out.Mode = AuthoringUnavailable
				break
			}
			if !errors.Is(err, service.ErrNotFound) {
				return AuthoringContext{}, err
			}
		}
	}
	if entries, err := p.store.ListDir(ctx, pageID, ""); err == nil {
		for _, entry := range entries {
			if entry.Name == authoring.HistoryDir || entry.Name == "dist" {
				continue
			}
			name := entry.Name
			if entry.IsDir {
				name += "/"
			}
			out.Files = append(out.Files, name)
		}
		sort.Strings(out.Files)
	}
	if data, err := p.store.ReadFile(ctx, pageID, docsurface.ManifestFileName); err == nil {
		out.Anchors = string(data)
	}
	if data, err := p.store.ReadFile(ctx, pageID, "index.tsx"); err == nil {
		out.IndexTSX = string(data)
	}
	return out, nil
}

func (p *Publication) Publish(ctx context.Context, pageID, rawSummary string) (PublishResult, error) {
	if err := p.authoring.RequireApp(ctx, pageID); err != nil {
		return PublishResult{}, err
	}
	var staged *service.BuildResult
	if p.build != nil {
		result, err := p.build.Build(ctx, pageID, service.ChannelPublished)
		if err != nil {
			return PublishResult{}, err
		}
		if !result.OK {
			return PublishResult{}, service.BadRequest("publish blocked — the published build failed:\n" + result.Log)
		}
		staged = &result
	} else if _, err := p.store.ReadFile(ctx, pageID, docsurface.BuildMetaPath); err != nil {
		return PublishResult{}, service.BadRequest("no successful build found — run build_app first")
	}
	report, err := p.verification.Verify(ctx, pageID, service.ChannelPublished, false, staged)
	if err != nil {
		return PublishResult{}, err
	}
	if problems := verification.FailureSummary(report); problems != "" {
		return PublishResult{}, service.BadRequest("publish blocked — verification failed:\n  - " + problems +
			"\nRun verify_app for the full report, fix, then publish_app again.")
	}
	if staged != nil && len(staged.Contract) > 0 && !report.RendererAvailable {
		return PublishResult{}, service.BadRequest("Resource-backed publication requires renderer verification; previous release remains active")
	}
	verified, warning := report.RendererAvailable, report.Warning
	if p.bridge != nil && (staged == nil || len(staged.Contract) == 0) {
		refs, err := p.bridge.CheckRefs(ctx, pageID)
		if err != nil {
			return PublishResult{}, err
		}
		if !refs.OK {
			var problems []string
			if len(refs.Missing) > 0 {
				problems = append(problems, "not found: "+strings.Join(refs.Missing, ", "))
			}
			if len(refs.UnknownKind) > 0 {
				problems = append(problems, "unknown kind: "+strings.Join(refs.UnknownKind, ", "))
			}
			return PublishResult{}, service.BadRequest("publish blocked — anchors.json refs don't resolve (" + strings.Join(problems, "; ") + "). Fix the ids or drop them from refs.")
		}
		if len(refs.Unobservable) > 0 {
			unobservable := "these refs can be read but never update live (their kind emits no change events): " + strings.Join(refs.Unobservable, ", ")
			if warning == "" {
				warning = unobservable
			} else {
				warning += "; " + unobservable
			}
		}
	}
	if staged != nil && len(staged.Contract) > 0 {
		if p.releases == nil {
			return PublishResult{}, service.BadRequest("Resource release activation unavailable")
		}
		if err := p.releases.Activate(ctx, pageID, service.ChannelPublished, staged.BuildID); err != nil {
			return PublishResult{}, err
		}
	}
	if summary := strings.TrimSpace(rawSummary); summary != "" {
		if _, err := p.artifacts.Update(ctx, pageID, service.ArtifactPatch{Summary: &summary}); err != nil {
			return PublishResult{}, err
		}
	}
	if p.build != nil {
		_, _ = p.build.Build(ctx, pageID, service.ChannelDraft)
	}
	return PublishResult{OK: true, ServedURL: "/content/" + pageID + "/", Verified: verified, Warning: warning}, nil
}
