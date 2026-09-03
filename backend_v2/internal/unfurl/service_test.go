package unfurl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// testUnfurler bypasses the private-address guard so httptest servers (loopback by
// definition) can stand in for the public web. The guard itself is tested separately
// against the REAL constructor below.
func testUnfurler(t *testing.T, handler http.Handler) (*unfurler, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.Timeout = 5 * time.Second
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= unfurlMaxRedirects {
			return fmt.Errorf("%w: too many redirects", ErrUnfurlUpstreamFailed)
		}
		return nil
	}
	return &unfurler{client: client, allowPrivate: true}, srv
}

func TestUnfurlReadsOpenGraphOverFallbacks(t *testing.T) {
	page := `<!doctype html><html><head>
		<title>HTML title — worse</title>
		<meta name="description" content="meta description — worse">
		<meta property="og:title" content="Momentum Crashes">
		<meta property="og:description" content="Momentum strategies crash in panic states.">
		<meta property="og:site_name" content="SSRN">
		<meta property="og:image" content="/img/preview.png">
		<link rel="icon" href="/static/icon.svg">
	</head><body>hi</body></html>`
	u, srv := testUnfurler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))

	got, err := u.Unfurl(context.Background(), srv.URL+"/papers/momentum")
	if err != nil {
		t.Fatalf("unfurl: %v", err)
	}
	if got.Title != "Momentum Crashes" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.Description != "Momentum strategies crash in panic states." {
		t.Fatalf("description = %q", got.Description)
	}
	if got.SiteName != "SSRN" {
		t.Fatalf("siteName = %q", got.SiteName)
	}
	if !strings.HasSuffix(got.ImageURL, "/img/preview.png") || !strings.HasPrefix(got.ImageURL, "http") {
		t.Fatalf("imageUrl = %q (want absolute)", got.ImageURL)
	}
	if !strings.HasSuffix(got.FaviconURL, "/static/icon.svg") {
		t.Fatalf("faviconUrl = %q", got.FaviconURL)
	}
	parsed, _ := url.Parse(srv.URL)
	if got.Domain != parsed.Hostname() {
		t.Fatalf("domain = %q want %q", got.Domain, parsed.Hostname())
	}
}

func TestUnfurlFallsBackToHTMLTitleAndMetaDescription(t *testing.T) {
	page := `<html><head><title>  Plain   Page  </title>
		<meta name="description" content="Just a description."></head><body></body></html>`
	u, srv := testUnfurler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))

	got, err := u.Unfurl(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unfurl: %v", err)
	}
	if got.Title != "Plain Page" {
		t.Fatalf("title = %q (whitespace should collapse)", got.Title)
	}
	if got.Description != "Just a description." {
		t.Fatalf("description = %q", got.Description)
	}
	if !strings.HasSuffix(got.FaviconURL, "/favicon.ico") {
		t.Fatalf("faviconUrl default = %q", got.FaviconURL)
	}
}

func TestUnfurlNonHTMLSynthesizesFromURL(t *testing.T) {
	u, srv := testUnfurler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4"))
	}))

	got, err := u.Unfurl(context.Background(), srv.URL+"/papers/momentum-crashes.pdf")
	if err != nil {
		t.Fatalf("unfurl: %v", err)
	}
	if got.Title != "momentum-crashes.pdf" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.Description != "" || got.ImageURL != "" {
		t.Fatalf("non-html should carry no description/image: %+v", got)
	}
}

func TestUnfurlFollowsRedirectsAndReportsFinalURL(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvURL+"/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Landed</title></head></html>`))
	})
	u, srv := testUnfurler(t, mux)
	srvURL = srv.URL

	got, err := u.Unfurl(context.Background(), srv.URL+"/old")
	if err != nil {
		t.Fatalf("unfurl: %v", err)
	}
	if !strings.HasSuffix(got.URL, "/new") {
		t.Fatalf("URL = %q (want the final hop)", got.URL)
	}
	if got.Title != "Landed" {
		t.Fatalf("title = %q", got.Title)
	}
}

func TestUnfurlUpstreamErrorStatuses(t *testing.T) {
	u, srv := testUnfurler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	_, err := u.Unfurl(context.Background(), srv.URL)
	if !errors.Is(err, ErrUnfurlUpstreamFailed) {
		t.Fatalf("want ErrUnfurlUpstreamFailed, got %v", err)
	}
}

func TestUnfurlClampsLongDescriptions(t *testing.T) {
	long := strings.Repeat("word ", 200)
	page := `<html><head><meta property="og:description" content="` + long + `"></head></html>`
	u, srv := testUnfurler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	got, err := u.Unfurl(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unfurl: %v", err)
	}
	if runes := []rune(got.Description); len(runes) > unfurlMaxDescrRunes+1 {
		t.Fatalf("description length %d > cap", len(runes))
	}
	if !strings.HasSuffix(got.Description, "…") {
		t.Fatalf("clamped description should end with ellipsis: %q", got.Description)
	}
}

func TestNormalizeUnfurlURL(t *testing.T) {
	cases := []struct {
		in      string
		wantErr error
		want    string
	}{
		{in: "example.com/page", want: "https://example.com/page"},
		{in: "  https://example.com  ", want: "https://example.com"},
		{in: "ftp://example.com", wantErr: ErrInvalidUnfurlURL},
		{in: "javascript:alert(1)", wantErr: ErrInvalidUnfurlURL},
		{in: "https://user:pw@example.com", wantErr: ErrInvalidUnfurlURL},
		{in: "", wantErr: ErrInvalidUnfurlURL},
		{in: "https://127.0.0.1/admin", wantErr: ErrUnfurlTargetRefused},
		{in: "https://10.0.0.8", wantErr: ErrUnfurlTargetRefused},
		{in: "https://169.254.169.254/latest/meta-data", wantErr: ErrUnfurlTargetRefused},
		{in: "https://[::1]:8000", wantErr: ErrUnfurlTargetRefused},
		{in: "https://100.64.1.2", wantErr: ErrUnfurlTargetRefused},
	}
	for _, tc := range cases {
		got, err := normalizeUnfurlURL(tc.in, false)
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("%q: want %v, got %v", tc.in, tc.wantErr, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got.String() != tc.want {
			t.Fatalf("%q → %q, want %q", tc.in, got.String(), tc.want)
		}
	}
}

// The REAL constructor must refuse loopback at dial time — this is the guard that stands
// between a pasted URL and the sandbox's own services (and, in prod, cloud metadata).
func TestUnfurlServiceRefusesLoopbackDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("the guard let a loopback request through")
	}))
	defer srv.Close()

	// The literal IP is refused before any dial; a NAME resolving to loopback must be
	// refused by the dialer's Control. httptest URLs are literal IPs, so exercise the
	// dial path via "localhost" on the same port.
	parsed, _ := url.Parse(srv.URL)
	_, err := NewUnfurlService().Unfurl(context.Background(), "http://localhost:"+parsed.Port())
	if err == nil {
		t.Fatal("want refusal, got success")
	}
	if !errors.Is(err, ErrUnfurlTargetRefused) && !strings.Contains(err.Error(), ErrUnfurlTargetRefused.Error()) {
		t.Fatalf("want target-refused, got %v", err)
	}
}

func TestIsPublicIP(t *testing.T) {
	private := []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "255.255.255.255", "::1", "fe80::1", "fc00::1"}
	for _, raw := range private {
		if isPublicIP(net.ParseIP(raw)) {
			t.Fatalf("%s should be refused", raw)
		}
	}
	public := []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946", "8.8.8.8"}
	for _, raw := range public {
		if !isPublicIP(net.ParseIP(raw)) {
			t.Fatalf("%s should be allowed", raw)
		}
	}
}
