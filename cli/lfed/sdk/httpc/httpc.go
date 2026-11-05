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

var (
	ErrUnauthorized = errors.New(`unauthorized; run "lfed auth login"`)
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

type APIError struct {
	Status int

	Body string
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("api error: HTTP %d", e.Status)
	}
	return fmt.Sprintf("api error: HTTP %d: %s", e.Status, e.Body)
}

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

type Client struct {
	baseURL   string
	token     string
	http      *http.Client
	userAgent string

	baseErr error
}

func debugf(format string, args ...any) {
	if config.Debug() {
		fmt.Fprintf(os.Stderr, "lfed-plugin: debug: "+format+"\n", args...)
	}
}

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

func WithUserAgent(name string) Option {
	return func(c *Client) { c.userAgent = "lfed-plugin/" + name }
}

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
