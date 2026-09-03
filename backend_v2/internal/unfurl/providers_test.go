package unfurl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type unfurlRoundTripper func(*http.Request) (*http.Response, error)

func (f unfurlRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func unfurlResponse(r *http.Request, status int, contentType, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}

func TestPreviewProviderURLVariants(t *testing.T) {
	for _, raw := range []string{
		"https://youtu.be/jNQXAC9IVRw?si=tracking&t=24", "https://www.youtube.com/watch?v=jNQXAC9IVRw&list=ignored",
		"https://m.youtube.com/shorts/jNQXAC9IVRw", "https://www.youtube.com/live/jNQXAC9IVRw",
		"https://www.youtube-nocookie.com/embed/jNQXAC9IVRw", "https://youtube.com/v/jNQXAC9IVRw",
	} {
		p := previewProviderForURL(raw)
		if p == nil || p.lookupURL != "https://www.youtube.com/watch?v=jNQXAC9IVRw" {
			t.Fatalf("%s: %+v", raw, p)
		}
	}
	for _, raw := range []string{"https://old.reddit.com/r/algotrading/comments/abc123/a_post/comment456/?share_id=x", "https://redd.it/abc123", "https://www.reddit.com/r/algotrading/s/ABC123"} {
		p := previewProviderForURL(raw)
		if p == nil || p.name != "Reddit" || !strings.HasPrefix(p.lookupURL, "https://www.reddit.com/") || strings.Contains(p.lookupURL, "share_id") {
			t.Fatalf("%s: %+v", raw, p)
		}
	}
	for _, raw := range []string{"https://youtube.com.evil.example/watch?v=jNQXAC9IVRw", "https://evilyoutube.com/watch?v=jNQXAC9IVRw", "https://youtube.com:9999/watch?v=jNQXAC9IVRw", "https://youtube.com/watch?v=bad", "https://user:pass@reddit.com/r/a/comments/abc/title", "https://reddit.com.evil.example/r/a/comments/abc/title"} {
		if p := previewProviderForURL(raw); p != nil {
			t.Fatalf("unexpected provider for %s: %+v", raw, p)
		}
	}
}

func TestUnfurlProviderMetadata(t *testing.T) {
	for _, tc := range []struct{ name, input, host, payload, title, description string }{
		{"youtube", "https://youtu.be/jNQXAC9IVRw?t=24", "www.youtube.com", `{"title":"Real video title","author_name":"A channel","thumbnail_url":"https://i.ytimg.com/vi/jNQXAC9IVRw/hqdefault.jpg","html":"<script>not executed</script>"}`, "Real video title", "By A channel"},
		{"reddit", "https://old.reddit.com/r/algotrading/comments/abc123/a_post/comment456/", "www.reddit.com", `{"title":"A real discussion","author_name":"researcher","type":"rich"}`, "A real discussion", "By researcher · r/algotrading"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			u := &unfurler{client: &http.Client{Transport: unfurlRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != tc.host || r.URL.Path != "/oembed" || r.URL.Query().Get("format") != "json" {
					t.Fatalf("wrong endpoint %s", r.URL)
				}
				if r.Header.Get("User-Agent") != unfurlUserAgent {
					t.Fatal("missing user agent")
				}
				return unfurlResponse(r, 200, "application/json", tc.payload), nil
			})}}
			got, err := u.Unfurl(context.Background(), tc.input)
			if err != nil || got.Title != tc.title || got.Description != tc.description || got.URL != tc.input || got.PreviewStatus != "" || calls != 1 {
				t.Fatalf("got %+v, %v (%d calls)", got, err, calls)
			}
		})
	}
}

func TestUnfurlProviderFallbacks(t *testing.T) {
	for _, status := range []int{403, 404, 429, 500, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			u := &unfurler{client: &http.Client{Transport: unfurlRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/oembed" {
					return unfurlResponse(r, status, "application/json", `{"title":"YouTube"}`), nil
				}
				return unfurlResponse(r, 200, "text/html", `<title>YouTube</title>`), nil
			})}}
			got, err := u.Unfurl(context.Background(), "https://youtube.com/shorts/jNQXAC9IVRw?si=test")
			if err != nil || got.Title != "YouTube video" || got.PreviewStatus != "partial" || got.ImageURL == "" {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestUnfurlProviderHTMLFallback(t *testing.T) {
	u := &unfurler{client: &http.Client{Transport: unfurlRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oembed" {
			return unfurlResponse(r, 200, "text/html", `not JSON`), nil
		}
		return unfurlResponse(r, 200, "text/html", `<meta property="og:title" content="Actual discussion title"><meta property="og:description" content="Some useful context">`), nil
	})}}
	got, err := u.Unfurl(context.Background(), "https://reddit.com/r/algotrading/comments/abc123/title/")
	if err != nil || got.Title != "Actual discussion title" || got.Description != "Some useful context" || got.PreviewStatus != "" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestUnfurlRedditBlockedDoesNotPretendToBeReady(t *testing.T) {
	u := &unfurler{client: &http.Client{Transport: unfurlRoundTripper(func(r *http.Request) (*http.Response, error) {
		return unfurlResponse(r, 403, "text/html", `<title>Blocked</title>`), nil
	})}}
	got, err := u.Unfurl(context.Background(), "https://reddit.com/r/algotrading/comments/abc123/title/")
	if err != nil || got.Title != "Reddit discussion" || got.PreviewStatus != "partial" || got.ImageURL != "" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestUnfurlRedditShareRedirect(t *testing.T) {
	u := &unfurler{client: &http.Client{Transport: unfurlRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/oembed" {
			if strings.Contains(r.URL.Query().Get("url"), "/s/") {
				return unfurlResponse(r, 404, "application/json", `{}`), nil
			}
			return unfurlResponse(r, 200, "application/json", `{"title":"Shared discussion"}`), nil
		}
		if strings.Contains(r.URL.Path, "/s/") {
			response := unfurlResponse(r, 302, "text/html", "")
			response.Header.Set("Location", "https://www.reddit.com/r/algotrading/comments/abc123/title/")
			return response, nil
		}
		return unfurlResponse(r, 200, "text/html", `<title>Reddit</title>`), nil
	})}}
	got, err := u.Unfurl(context.Background(), "https://reddit.com/r/algotrading/s/ABC123")
	if err != nil || got.Title != "Shared discussion" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

// Opt-in smoke test, never a network dependency for CI.
func TestUnfurlLiveProviders(t *testing.T) {
	if os.Getenv("ALADIN_TEST_LIVE_UNFURL") != "1" {
		t.Skip("set ALADIN_TEST_LIVE_UNFURL=1 to probe public sample URLs")
	}
	for _, raw := range []string{
		"https://youtu.be/jNQXAC9IVRw?t=2",
		"https://www.reddit.com/r/reddit/comments/135shj5/making_it_easier_to_share_your_favorite_reddit/",
	} {
		got, err := NewUnfurlService().Unfurl(context.Background(), raw)
		if err != nil || got.Title == "" || got.PreviewStatus != "" {
			t.Fatalf("%s: %+v, %v", raw, got, err)
		}
		parsed, _ := url.Parse(raw)
		t.Logf("%s: %q; image=%t", parsed.Hostname(), got.Title, got.ImageURL != "")
	}
}
