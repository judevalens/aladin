package blocknote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Converter is the interface the MCP tools (and anyone else) depend on for
// markdown <-> blocks conversion. Production wires Client; tests provide
// fakes.
type Converter interface {
	MDToBlocks(ctx context.Context, markdown string) (json.RawMessage, error)
	BlocksToMD(ctx context.Context, blocks json.RawMessage) (string, error)
	BlocksToMDBatch(ctx context.Context, blocks []json.RawMessage) ([]string, error)
	Healthz(ctx context.Context) error
}

// Client is the HTTP implementation of Converter (and, M8c, Bridge) targeting
// the blocknote Node sidecar — the converter routes and the /admin/* MCP
// bridge share one base URL and one client.
type Client struct {
	baseURL     string
	http        *http.Client
	timeout     time.Duration
	adminSecret string
}

// ClientOptions tweaks Client behavior. Zero values are reasonable defaults.
type ClientOptions struct {
	// HTTPClient lets tests inject an *http.Client. Defaults to a client
	// with a 10s timeout (covers cold-jsdom + a large document).
	HTTPClient *http.Client
	// RequestTimeout is the per-call ceiling enforced via context.WithTimeout
	// if the caller didn't already set one. Default: 10s.
	RequestTimeout time.Duration
	// AdminSecret is sent as the X-Admin-Secret header on /admin/* bridge
	// calls (M8c). Empty => bridge calls will be rejected (401) by the sidecar.
	AdminSecret string
}

func NewClient(baseURL string, opts ClientOptions) *Client {
	timeout := opts.RequestTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout + 2*time.Second}
	}
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		http:        client,
		timeout:     timeout,
		adminSecret: opts.AdminSecret,
	}
}

func (c *Client) MDToBlocks(ctx context.Context, markdown string) (json.RawMessage, error) {
	var out struct {
		Blocks json.RawMessage `json:"blocks"`
	}
	if err := c.post(ctx, "/md-to-blocks", map[string]string{"markdown": markdown}, &out); err != nil {
		return nil, err
	}
	if len(out.Blocks) == 0 {
		return json.RawMessage(`[]`), nil
	}
	return out.Blocks, nil
}

func (c *Client) BlocksToMD(ctx context.Context, blocks json.RawMessage) (string, error) {
	if len(blocks) == 0 {
		return "", nil
	}
	body := json.RawMessage(append(append([]byte(`{"blocks":`), blocks...), '}'))
	var out struct {
		Markdown string `json:"markdown"`
	}
	if err := c.postRaw(ctx, "/blocks-to-md", body, &out); err != nil {
		return "", err
	}
	return out.Markdown, nil
}

func (c *Client) BlocksToMDBatch(ctx context.Context, blocks []json.RawMessage) ([]string, error) {
	if len(blocks) == 0 {
		return []string{}, nil
	}
	// Marshal [[block...], [block...]] without re-encoding each element.
	var buf bytes.Buffer
	buf.WriteString(`{"blocks":[`)
	for i, b := range blocks {
		if i > 0 {
			buf.WriteByte(',')
		}
		if len(b) == 0 {
			buf.WriteString(`[]`)
		} else {
			buf.Write(b)
		}
	}
	buf.WriteString(`]}`)
	var out struct {
		Markdowns []string `json:"markdowns"`
	}
	if err := c.postRaw(ctx, "/blocks-to-md-batch", buf.Bytes(), &out); err != nil {
		return nil, err
	}
	return out.Markdowns, nil
}

func (c *Client) Healthz(ctx context.Context) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("blocknote-converter healthz status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.postRaw(ctx, path, encoded, out)
}

func (c *Client) postRaw(ctx context.Context, path string, body []byte, out any) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("blocknote-converter %s: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return parseConverterError(path, resp.StatusCode, respBody)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("blocknote-converter %s: decode response: %w", path, err)
	}
	return nil
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.timeout)
}

// ErrConverter is returned when the converter sidecar reported an
// application-level error (4xx). Callers can errors.Is(err, ErrConverter)
// to distinguish from transport failures.
var ErrConverter = errors.New("blocknote-converter rejected the request")

type converterError struct {
	path    string
	status  int
	message string
}

func (e *converterError) Error() string {
	return fmt.Sprintf("blocknote-converter %s: status %d: %s", e.path, e.status, e.message)
}

func (e *converterError) Is(target error) bool {
	return target == ErrConverter
}

func parseConverterError(path string, status int, body []byte) error {
	var payload struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	message := payload.Error
	if message == "" {
		message = strings.TrimSpace(string(body))
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return &converterError{path: path, status: status, message: message}
}

// --- M8c MCP collab bridge ------------------------------------------------

// Bridge is the interface the MCP write tools depend on to mutate / read a
// page's live Y.Doc via the sidecar's /admin/* endpoints. *Client implements
// it; tests provide fakes.
type Bridge interface {
	ApplyOperation(ctx context.Context, op BridgeOp) (BridgeResult, error)
	GetPage(ctx context.Context, pageID string) (BridgePage, error)
}

// BridgeOp is a block operation applied to a page's live Y.Doc.
type BridgeOp struct {
	PageID    string          `json:"page_id"`
	Op        string          `json:"op"` // replace_all|replace_block|insert_blocks|delete_block
	BlockID   string          `json:"block_id,omitempty"`
	Position  *int            `json:"position,omitempty"`
	Placement string          `json:"placement,omitempty"` // before|after (insert_blocks)
	Blocks    json.RawMessage `json:"blocks,omitempty"`
	Agent     *BridgeAgent    `json:"agent,omitempty"`
}

// BridgeAgent identifies the integration applying the op (for awareness/audit).
type BridgeAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// BridgeResult is the response from an apply op: the full current block-id
// list plus the affected blocks rendered to markdown (so the agent sees its
// own edit without a follow-up read).
type BridgeResult struct {
	BlockIDs         []string `json:"blockIds"`
	AffectedBlockIDs []string `json:"affectedBlockIds"`
	Markdown         string   `json:"markdown"`
}

// BridgePage is the materialized live Y.Doc view returned by GET /admin/page/:id.
type BridgePage struct {
	Blocks   json.RawMessage `json:"blocks"`
	Markdown string          `json:"markdown"`
}

// ApplyOperation applies a block op to a page's live Y.Doc via the bridge.
func (c *Client) ApplyOperation(ctx context.Context, op BridgeOp) (BridgeResult, error) {
	body, err := json.Marshal(op)
	if err != nil {
		return BridgeResult{}, err
	}
	var out BridgeResult
	if err := c.admin(ctx, http.MethodPost, "/admin/apply", body, &out); err != nil {
		return BridgeResult{}, err
	}
	return out, nil
}

// GetPage materializes a page's live Y.Doc to blocks + markdown via the bridge.
func (c *Client) GetPage(ctx context.Context, pageID string) (BridgePage, error) {
	var out BridgePage
	path := "/admin/page/" + url.PathEscape(pageID)
	if err := c.admin(ctx, http.MethodGet, path, nil, &out); err != nil {
		return BridgePage{}, err
	}
	return out, nil
}

// admin performs an /admin/* request with the shared-secret header. Errors
// reuse the converter error mapping (errors.Is(err, ErrConverter) for 4xx).
func (c *Client) admin(ctx context.Context, method, path string, body []byte, out any) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.adminSecret != "" {
		req.Header.Set("X-Admin-Secret", c.adminSecret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("blocknote-bridge %s: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return parseConverterError(path, resp.StatusCode, respBody)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("blocknote-bridge %s: decode response: %w", path, err)
	}
	return nil
}
