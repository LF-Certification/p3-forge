// Package httpc is a small bearer-token JSON client with typed errors.
package httpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/config"
)

// Sentinel errors for the HTTP statuses a command may handle. Do returns
// errors that wrap them, so callers use errors.Is.
var (
	ErrUnauthorized = errors.New(`unauthorized; run "lfed auth login"`)
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

// APIError is any other non-2xx response.
type APIError struct {
	Status int
	// Body is the FastAPI `detail` when present, else the raw body.
	Body string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("api error: HTTP %d", e.Status)
	}
	return fmt.Sprintf("api error: HTTP %d: %s", e.Status, e.Body)
}

// statusError wraps a sentinel and carries the server detail.
type statusError struct {
	sentinel error
	detail   string
}

func (e *statusError) Error() string {
	if e.detail == "" {
		return e.sentinel.Error()
	}
	return e.sentinel.Error() + ": " + e.detail
}

func (e *statusError) Unwrap() error { return e.sentinel }

// Client sends JSON requests with a bearer token.
type Client struct {
	baseURL   string
	token     string
	http      *http.Client
	userAgent string
	// baseErr is the config.CheckAPIBaseURL result for baseURL. Do
	// returns it before it sends a request.
	baseErr error
}

// debugf writes one diagnostic line to stderr when LFED_DEBUG is set. The
// Authorization header is never logged.
func debugf(format string, args ...any) {
	if config.Debug() {
		fmt.Fprintf(os.Stderr, "lfed-plugin: debug: "+format+"\n", args...)
	}
}

// Option configures New.
type Option func(*Client)

// WithHTTPClient replaces the default 30-second-timeout client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithUserAgent sets `User-Agent: lfed-plugin/<name>`.
func WithUserAgent(name string) Option {
	return func(c *Client) { c.userAgent = "lfed-plugin/" + name }
}

// New builds a Client. Trailing slashes on baseURL are stripped so paths
// can start with "/". A baseURL that fails config.CheckAPIBaseURL makes
// every Do call return that error without a request, so the token never
// goes over plain http to a remote host.
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	c.baseErr = config.CheckAPIBaseURL(c.baseURL)
	for _, o := range opts {
		o(c)
	}
	return c
}

// Do sends method path with body marshalled as JSON (nil sends no body)
// and decodes a 2xx response into out (nil discards it; 204 is fine).
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	if c.baseErr != nil {
		return c.baseErr
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	started := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		debugf("%s %s: %v", method, req.URL, err)
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	debugf("%s %s -> %d (%d bytes, %s)", method, req.URL, resp.StatusCode, len(raw), time.Since(started).Round(time.Millisecond))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return statusToError(resp.StatusCode, raw)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func statusToError(status int, raw []byte) error {
	detail := extractDetail(raw)
	var sentinel error
	switch status {
	case http.StatusUnauthorized:
		sentinel = ErrUnauthorized
	case http.StatusForbidden:
		sentinel = ErrForbidden
	case http.StatusNotFound:
		sentinel = ErrNotFound
	case http.StatusConflict:
		sentinel = ErrConflict
	default:
		return &APIError{Status: status, Body: detail}
	}
	return &statusError{sentinel: sentinel, detail: detail}
}

// extractDetail returns FastAPI's `detail` when the body is a JSON object
// with one, else the trimmed body. Validation errors (422) carry a list,
// which is kept as compact JSON.
func extractDetail(raw []byte) string {
	var envelope struct {
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Detail) > 0 {
		var s string
		if json.Unmarshal(envelope.Detail, &s) == nil {
			return s
		}
		return string(envelope.Detail)
	}
	return strings.TrimSpace(string(raw))
}
