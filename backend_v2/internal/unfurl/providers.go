package unfurl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Fixed provider endpoints only; never follow an arbitrary oEmbed URL supplied in HTML.
// Both requests use the same dial-time private-address / redirect guard as generic URLs.
type previewProvider struct {
	name, endpoint, lookupURL, videoID string
}

var youtubeVideoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var youtubePlaylistID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var redditPostID = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func previewProviderForURL(raw string) *previewProvider {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return previewProviderFor(u)
}

func previewProviderFor(u *url.URL) *previewProvider {
	if u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || (u.Port() != "" && u.Port() != "443" && u.Port() != "80") {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com", "www.youtube-nocookie.com", "youtu.be", "www.youtu.be":
		id := ""
		if host == "youtu.be" || host == "www.youtu.be" {
			if len(parts) == 1 {
				id = parts[0]
			}
		} else if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "live" || parts[0] == "embed" || parts[0] == "v") {
			id = parts[1]
		}
		if youtubeVideoID.MatchString(id) {
			return &previewProvider{name: "YouTube", endpoint: "https://www.youtube.com/oembed", lookupURL: "https://www.youtube.com/watch?v=" + id, videoID: id}
		}
		if u.Path == "/playlist" && youtubePlaylistID.MatchString(u.Query().Get("list")) {
			return &previewProvider{name: "YouTube", endpoint: "https://www.youtube.com/oembed", lookupURL: "https://www.youtube.com/playlist?" + url.Values{"list": {u.Query().Get("list")}}.Encode()}
		}
	case "reddit.com", "www.reddit.com", "old.reddit.com", "new.reddit.com", "m.reddit.com", "redd.it":
		path := u.Path
		if host == "redd.it" && len(parts) == 1 && redditPostID.MatchString(parts[0]) {
			path = "/comments/" + parts[0] + "/"
		} else if !((len(parts) >= 4 && parts[0] == "r" && (parts[2] == "comments" || parts[2] == "s") && redditPostID.MatchString(parts[3])) ||
			(len(parts) >= 2 && parts[0] == "comments" && redditPostID.MatchString(parts[1]))) {
			return nil
		}
		lookup := &url.URL{Scheme: "https", Host: "www.reddit.com", Path: path}
		return &previewProvider{name: "Reddit", endpoint: "https://www.reddit.com/oembed", lookupURL: lookup.String()}
	}
	return nil
}

func (u *unfurler) providerPreview(ctx context.Context, original *url.URL, provider *previewProvider) (Unfurl, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	query := url.Values{"url": {provider.lookupURL}, "format": {"json"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return Unfurl{}, err
	}
	req.Header.Set("User-Agent", unfurlUserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := u.client.Do(req)
	if err != nil {
		return Unfurl{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Unfurl{}, fmt.Errorf("%w: provider status %d", ErrUnfurlUpstreamFailed, resp.StatusCode)
	}
	var payload struct {
		Title     string `json:"title"`
		Author    string `json:"author_name"`
		Thumbnail string `json:"thumbnail_url"`
	}
	// We intentionally never render the returned embed HTML or execute its scripts.
	if err := json.NewDecoder(io.LimitReader(resp.Body, unfurlMaxBodyBytes)).Decode(&payload); err != nil {
		return Unfurl{}, err
	}
	title := cleanText(payload.Title)
	if genericProviderTitle(title, provider.name) {
		return Unfurl{}, fmt.Errorf("%w: empty provider metadata", ErrUnfurlUpstreamFailed)
	}
	preview := Unfurl{
		URL: original.String(), Domain: displayDomain(original), SiteName: provider.name,
		Title: clampRunes(title, 500), ImageURL: resolveAgainst(original, payload.Thumbnail),
		FaviconURL: "https://www.youtube.com/favicon.ico",
	}
	if author := cleanText(payload.Author); author != "" {
		preview.Description = clampRunes("By "+author, unfurlMaxDescrRunes)
	}
	if provider.name == "Reddit" {
		preview.FaviconURL = "https://www.reddit.com/favicon.ico"
		parts := strings.Split(strings.Trim(original.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "r" {
			preview.Description = strings.TrimSpace(preview.Description + " · r/" + parts[1])
			preview.Description = strings.TrimPrefix(preview.Description, "· ")
		}
	}
	return preview, nil
}

func genericProviderTitle(title, provider string) bool {
	value := strings.ToLower(cleanText(title))
	value = strings.TrimSuffix(value, " - "+strings.ToLower(provider))
	switch value {
	case "", "youtube", "youtube.com", "www.youtube.com", "reddit", "reddit.com", "www.reddit.com", "reddit - dive into anything", "dive into anything", "blocked", "access denied", "whoa there, pardner!":
		return true
	}
	return strings.HasPrefix(value, "before you continue") || strings.HasPrefix(value, "you've been blocked") || strings.HasPrefix(value, "log in") || strings.HasPrefix(value, "sign in")
}

func providerFallback(original *url.URL, provider *previewProvider) Unfurl {
	preview := Unfurl{URL: original.String(), Domain: displayDomain(original), SiteName: provider.name, PreviewStatus: "partial",
		Title: provider.name + " link", Description: "Preview unavailable. Open the source or refresh the preview."}
	if provider.name == "YouTube" {
		preview.Title = "YouTube playlist"
		preview.FaviconURL = "https://www.youtube.com/favicon.ico"
		if provider.videoID != "" {
			preview.Title = "YouTube video"
			preview.ImageURL = "https://i.ytimg.com/vi/" + provider.videoID + "/hqdefault.jpg"
		}
	} else {
		preview.Title = "Reddit discussion"
		preview.FaviconURL = "https://www.reddit.com/favicon.ico"
	}
	return preview
}
