package blocknote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// Client is the HTTP implementation of Converter targeting the
// blocknote-converter Node sidecar.
type Client struct {
	baseURL string
	http    *http.Client
	timeout time.Duration
}

// ClientOptions tweaks Client behavior. Zero values are reasonable defaults.
type ClientOptions struct {
	// HTTPClient lets tests inject an *http.Client. Defaults to a client
	// with a 10s timeout (covers cold-jsdom + a large document).
	HTTPClient *http.Client
	// RequestTimeout is the per-call ceiling enforced via context.WithTimeout
	// if the caller didn't already set one. Default: 10s.
	RequestTimeout time.Duration
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
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    client,
		timeout: timeout,
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
