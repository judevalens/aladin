package docsurface

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/service"
)

type VerificationReport struct {
	OK                bool                `json:"ok"`
	Channel           string              `json:"channel"`
	RendererAvailable bool                `json:"renderer_available"`
	Warning           string              `json:"warning,omitempty"`
	ManifestProblems  []string            `json:"manifest_problems,omitempty"`
	Refs              *ReferenceSummary   `json:"refs,omitempty"`
	Routes            []VerificationRoute `json:"routes,omitempty"`
}

type VerificationRoute struct {
	Route          string         `json:"route"`
	OK             bool           `json:"ok"`
	Mounted        bool           `json:"mounted"`
	AnchorsFound   map[string]int `json:"anchors_found,omitempty"`
	AnchorsMissing []string       `json:"anchors_missing,omitempty"`
	Exceptions     []string       `json:"exceptions,omitempty"`
	ConsoleErrors  []string       `json:"console_errors,omitempty"`
	EscapingLinks  []string       `json:"escaping_links,omitempty"`
	NavigateError  string         `json:"navigate_error,omitempty"`
}

type ReferenceSummary struct {
	OK           bool     `json:"ok"`
	Total        int      `json:"total"`
	Missing      []string `json:"missing,omitempty"`
	UnknownKind  []string `json:"unknown_kind,omitempty"`
	Unobservable []string `json:"unobservable,omitempty"`
}

// Verification owns renderer-driven evidence collection. It has no transport
// or publication responsibility: callers decide how to expose or gate a report.
type Verification struct {
	store   service.DocSurfaceStore
	preview service.PreviewService
}

func NewVerification(store service.DocSurfaceStore, preview service.PreviewService) *Verification {
	return &Verification{store: store, preview: preview}
}

func (v *Verification) Verify(ctx context.Context, pageID string, channel service.BuildChannel, strictConsole bool, built *service.BuildResult) (VerificationReport, error) {
	report := VerificationReport{Channel: string(channel)}
	data, readErr := v.store.ReadFile(ctx, pageID, ManifestFileName)
	if built != nil && len(built.Contract) > 0 {
		data, readErr = built.Files[ManifestFileName], nil
	}
	if readErr == nil {
		report.ManifestProblems = ValidateManifestBytes(data)
		if len(report.ManifestProblems) > 0 {
			return report, nil
		}
	}
	byRoute, routes := v.ManifestAnchorsByRoute(ctx, pageID, data)
	first, err := v.preview.Open(ctx, pageID, channel, service.PreviewOpenOptions{Build: built})
	if err != nil {
		if IsRendererUnavailable(err) {
			report.Warning = "renderer unavailable — nothing was verified; preview the routes manually before relying on this build."
			return report, nil
		}
		return report, err
	}
	report.RendererAvailable = true

	check := func(route string, state service.PreviewState) VerificationRoute {
		result := VerificationRoute{Route: route, Mounted: state.Mounted, Exceptions: state.Exceptions}
		if errs, err := v.preview.ConsoleErrors(ctx, pageID); err == nil {
			result.ConsoleErrors = errs
		}
		if links, err := v.preview.EscapingLinks(ctx, pageID); err == nil {
			result.EscapingLinks = links
		}
		declared := byRoute[route]
		if len(declared) > 0 {
			if counts, err := v.preview.CheckAnchors(ctx, pageID, declared); err == nil {
				result.AnchorsFound = counts
				for _, id := range declared {
					if counts[id] == 0 {
						result.AnchorsMissing = append(result.AnchorsMissing, id)
					}
				}
			}
		}
		result.OK = result.Mounted && len(result.Exceptions) == 0 && len(result.AnchorsMissing) == 0 &&
			len(result.EscapingLinks) == 0 && (!strictConsole || len(result.ConsoleErrors) == 0)
		return result
	}

	firstRoute := firstNonEmpty(routeOf(first.URL), "#/")
	report.Routes = append(report.Routes, check(firstRoute, first))
	for _, route := range routes {
		if route == firstRoute {
			continue
		}
		state, err := v.preview.Navigate(ctx, pageID, route)
		if err != nil {
			if IsRendererUnavailable(err) {
				report.RendererAvailable = false
				report.Warning = "renderer unavailable mid-verification — the remaining routes were not checked."
				return report, nil
			}
			report.Routes = append(report.Routes, VerificationRoute{Route: route, NavigateError: firstLine(err.Error())})
			continue
		}
		report.Routes = append(report.Routes, check(route, state))
	}
	report.OK = len(report.ManifestProblems) == 0
	for _, route := range report.Routes {
		if !route.OK {
			report.OK = false
		}
	}
	return report, nil
}

func FailureSummary(report VerificationReport) string {
	if report.OK || !report.RendererAvailable {
		return ""
	}
	var lines []string
	for _, problem := range report.ManifestProblems {
		lines = append(lines, "anchors.json: "+problem)
	}
	for _, route := range report.Routes {
		if route.OK {
			continue
		}
		switch {
		case route.NavigateError != "":
			lines = append(lines, route.Route+" (navigate error: "+route.NavigateError+")")
		case !route.Mounted:
			lines = append(lines, route.Route+" (did not mount)")
		case len(route.Exceptions) > 0:
			lines = append(lines, fmt.Sprintf("%s (%d uncaught exception(s): %s)", route.Route, len(route.Exceptions), firstLine(route.Exceptions[0])))
		case len(route.EscapingLinks) > 0:
			lines = append(lines, fmt.Sprintf("%s (link(s) navigate off the shard and will 401 when clicked: %s — use hash routes such as href=\"#/section\")", route.Route, strings.Join(route.EscapingLinks, ", ")))
		case len(route.AnchorsMissing) > 0:
			lines = append(lines, fmt.Sprintf("%s (declared anchors not in the DOM: %s)", route.Route, strings.Join(route.AnchorsMissing, ", ")))
		case len(route.ConsoleErrors) > 0:
			lines = append(lines, fmt.Sprintf("%s (%d console error(s): %s)", route.Route, len(route.ConsoleErrors), firstLine(route.ConsoleErrors[0])))
		}
	}
	return strings.Join(lines, "\n  - ")
}

func (v *Verification) ManifestAnchorsByRoute(ctx context.Context, pageID string, snapshot ...[]byte) (map[string][]string, []string) {
	data, err := v.store.ReadFile(ctx, pageID, ManifestFileName)
	if len(snapshot) > 0 {
		data, err = snapshot[0], nil
	}
	if err != nil {
		return nil, []string{"#/"}
	}
	manifest, err := ParseManifest(data)
	if err != nil {
		return nil, []string{"#/"}
	}
	byRoute := map[string][]string{}
	seen := map[string]bool{}
	var routes []string
	for _, anchor := range manifest.Anchors {
		route := strings.TrimSpace(anchor.Route)
		if route == "" {
			continue
		}
		if !seen[route] {
			seen[route] = true
			routes = append(routes, route)
		}
		if id := strings.TrimSpace(anchor.ID); id != "" {
			byRoute[route] = append(byRoute[route], id)
		}
	}
	if len(routes) == 0 {
		return byRoute, []string{"#/"}
	}
	return byRoute, routes
}

func routeOf(url string) string {
	if index := strings.IndexByte(url, '#'); index >= 0 {
		return url[index:]
	}
	return ""
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = value[:index]
	}
	value = strings.TrimSpace(value)
	if len(value) > 160 {
		value = value[:160] + "…"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
