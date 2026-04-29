// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package resendx wraps the parts of the Resend REST API that the official Go
// SDK at github.com/resend/resend-go/v3 does not expose, so the provider can
// surface fields like domain capabilities and webhook signing secrets on Read.
package resendx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://api.resend.com"

// Client is the HTTP supplement client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	userAgent  string
}

// New creates a Client. The version is interpolated into the User-Agent header,
// which Resend's API rejects requests without.
func New(apiKey, version string) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		userAgent:  fmt.Sprintf("terraform-provider-resend/%s", version),
	}
}

// WithBaseURL overrides the API base URL — used by tests pointing at httptest.
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = strings.TrimRight(baseURL, "/")
	return c
}

// WithHTTPClient swaps the underlying http.Client. Tests rarely need this; the
// default client honours per-request context.
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	c.httpClient = h
	return c
}

// APIError is returned for non-2xx responses. Resend's error envelope varies
// across endpoints, so the optional fields are decoded on a best-effort basis.
type APIError struct {
	StatusCode int    `json:"-"`
	Name       string `json:"name,omitempty"`
	Message    string `json:"message,omitempty"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("resend api %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("resend api %d", e.StatusCode)
}

// IsNotFound reports whether err is a 404 from Resend.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		rdr = bytes.NewReader(b)
	}

	url := c.baseURL + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		_ = json.NewDecoder(resp.Body).Decode(apiErr)
		return apiErr
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
