package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// Unfurl is the resolved preview of an external URL — what a board link object renders,
// and what the MCP board summary hands an agent reading the board.
type Unfurl struct {
	URL         string `json:"url"`
	Domain      string `json:"domain"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SiteName    string `json:"siteName"`
	ImageURL    string `json:"imageUrl"`
	FaviconURL  string `json:"faviconUrl"`
}

type UnfurlService interface {
	// Unfurl fetches the URL (server-side — browsers can't, CORS) and extracts its
	// OpenGraph/Twitter/HTML metadata. Non-HTML targets return a synthesized preview
	// rather than an error: a link to a PDF is still a link worth showing.
	Unfurl(ctx context.Context, rawURL string) (Unfurl, error)
}

var (
	ErrInvalidUnfurlURL     = errors.New("unfurl: invalid url")
	ErrUnfurlTargetRefused  = errors.New("unfurl: target address refused")
	ErrUnfurlUpstreamFailed = errors.New("unfurl: upstream fetch failed")
)

const (
	unfurlMaxBodyBytes  = 512 << 10
	unfurlTimeout       = 6 * time.Second
	unfurlMaxRedirects  = 5
	unfurlMaxDescrRunes = 300
	unfurlUserAgent     = "Mozilla/5.0 (compatible; AladinUnfurl/1.0)"
)

type unfurler struct {
	client *http.Client
	// allowPrivate skips the literal-IP refusal in normalize — ONLY for tests, whose
	// stand-in servers are loopback by definition. The production constructor never sets it.
	allowPrivate bool
}

// NewUnfurlService builds the production unfurler: outbound-only HTTP with the private-
// address guard applied at dial time (post-DNS, so a rebinding name can't slip past a
// pre-resolve check) and re-applied on every redirect hop through the same transport.
func NewUnfurlService() UnfurlService {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrUnfurlTargetRefused, address)
			}
			ip := net.ParseIP(host)
			if ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("%w: %s", ErrUnfurlTargetRefused, host)
			}
			return nil
		},
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}
	return &unfurler{client: &http.Client{
		Transport: transport,
		Timeout:   unfurlTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= unfurlMaxRedirects {
				return fmt.Errorf("%w: too many redirects", ErrUnfurlUpstreamFailed)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect to %s", ErrUnfurlTargetRefused, req.URL.Scheme)
			}
			return nil
		},
	}}
}

// isPublicIP rejects every address an unfurl must never reach: loopback, RFC1918 +
// ULA (IsPrivate), link-local (169.254/16, fe80::/10 — cloud metadata lives here),
// multicast, unspecified, and CGNAT 100.64/10.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 { // 100.64.0.0/10
			return false
		}
		if v4[0] == 255 { // broadcast-ish
			return false
		}
	}
	return true
}

func (u *unfurler) Unfurl(ctx context.Context, rawURL string) (Unfurl, error) {
	target, err := normalizeUnfurlURL(rawURL, u.allowPrivate)
	if err != nil {
		return Unfurl{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return Unfurl{}, fmt.Errorf("%w: %v", ErrInvalidUnfurlURL, err)
	}
	req.Header.Set("User-Agent", unfurlUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")

	resp, err := u.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrUnfurlTargetRefused) || strings.Contains(err.Error(), ErrUnfurlTargetRefused.Error()) {
			return Unfurl{}, ErrUnfurlTargetRefused
		}
		return Unfurl{}, fmt.Errorf("%w: %v", ErrUnfurlUpstreamFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Unfurl{}, fmt.Errorf("%w: status %d", ErrUnfurlUpstreamFailed, resp.StatusCode)
	}

	final := resp.Request.URL
	base := Unfurl{
		URL:        final.String(),
		Domain:     displayDomain(final),
		FaviconURL: final.Scheme + "://" + final.Host + "/favicon.ico",
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		// A PDF, an image, a CSV — still a link. Name it from the path.
		base.Title = titleFromURL(final)
		return base, nil
	}

	body, err := charset.NewReader(io.LimitReader(resp.Body, unfurlMaxBodyBytes), resp.Header.Get("Content-Type"))
	if err != nil {
		body = io.LimitReader(resp.Body, unfurlMaxBodyBytes)
	}
	meta := parseUnfurlHTML(body)

	base.Title = coalesceUnfurl(meta.ogTitle, meta.twitterTitle, meta.htmlTitle, titleFromURL(final))
	base.Description = clampRunes(coalesceUnfurl(meta.ogDescription, meta.twitterDescription, meta.metaDescription), unfurlMaxDescrRunes)
	base.SiteName = meta.ogSiteName
	base.ImageURL = resolveAgainst(final, coalesceUnfurl(meta.ogImage, meta.twitterImage))
	if icon := resolveAgainst(final, meta.iconHref); icon != "" {
		base.FaviconURL = icon
	}
	return base, nil
}

// normalizeUnfurlURL accepts what a person pastes: scheme optional (https assumed),
// http/https only, no credentials smuggled in the authority.
func normalizeUnfurlURL(rawURL string, allowPrivate bool) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, ErrInvalidUnfurlURL
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidUnfurlURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: scheme %q", ErrInvalidUnfurlURL, parsed.Scheme)
	}
	if parsed.Host == "" || parsed.User != nil {
		return nil, ErrInvalidUnfurlURL
	}
	// A literal private IP gets a clear refusal here; names are judged at dial time.
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && !isPublicIP(ip) && !allowPrivate {
		return nil, ErrUnfurlTargetRefused
	}
	return parsed, nil
}

type unfurlMeta struct {
	htmlTitle          string
	metaDescription    string
	ogTitle            string
	ogDescription      string
	ogSiteName         string
	ogImage            string
	twitterTitle       string
	twitterDescription string
	twitterImage       string
	iconHref           string
}

// parseUnfurlHTML walks the document once, collecting the tags the preview needs. The
// tokenizer forgives malformed markup — real pages are never clean.
func parseUnfurlHTML(r io.Reader) unfurlMeta {
	var meta unfurlMeta
	doc, err := html.Parse(r)
	if err != nil {
		return meta
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if meta.htmlTitle == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					meta.htmlTitle = cleanText(n.FirstChild.Data)
				}
			case "meta":
				name := strings.ToLower(coalesceUnfurl(attr(n, "property"), attr(n, "name")))
				content := cleanText(attr(n, "content"))
				if content == "" {
					break
				}
				switch name {
				case "og:title":
					meta.ogTitle = content
				case "og:description":
					meta.ogDescription = content
				case "og:site_name":
					meta.ogSiteName = content
				case "og:image", "og:image:url", "og:image:secure_url":
					if meta.ogImage == "" {
						meta.ogImage = content
					}
				case "twitter:title":
					meta.twitterTitle = content
				case "twitter:description":
					meta.twitterDescription = content
				case "twitter:image", "twitter:image:src":
					if meta.twitterImage == "" {
						meta.twitterImage = content
					}
				case "description":
					meta.metaDescription = content
				}
			case "link":
				rel := strings.ToLower(attr(n, "rel"))
				if href := attr(n, "href"); href != "" {
					// Later, larger icons win over the bare "icon" only if none is set —
					// first declared wins, which in practice is the canonical one.
					if meta.iconHref == "" && (rel == "icon" || rel == "shortcut icon" || rel == "apple-touch-icon") {
						meta.iconHref = href
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return meta
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func coalesceUnfurl(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func clampRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimSpace(string(runes[:max])) + "…"
}

func displayDomain(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(host, "www.")
}

func titleFromURL(u *url.URL) string {
	if segment := path.Base(u.Path); segment != "" && segment != "/" && segment != "." {
		if decoded, err := url.PathUnescape(segment); err == nil {
			segment = decoded
		}
		return segment
	}
	return displayDomain(u)
}

func resolveAgainst(base *url.URL, href string) string {
	if href == "" {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}
