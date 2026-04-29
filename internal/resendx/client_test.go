// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resendx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-key", "1.2.3").WithBaseURL(srv.URL)
}

func TestClient_GetDomain_OK(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/domains/dom_123", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "terraform-provider-resend/1.2.3", r.Header.Get("User-Agent"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "dom_123",
			"tls": "enforced",
			"capabilities": { "sending": "enabled", "receiving": "disabled" }
		}`))
	})

	d, err := c.GetDomain(context.Background(), "dom_123")
	require.NoError(t, err)
	require.Equal(t, "enforced", d.Tls)
	require.NotNil(t, d.Capabilities)
	require.Equal(t, "enabled", d.Capabilities.Sending)
	require.Equal(t, "disabled", d.Capabilities.Receiving)
}

func TestClient_GetDomain_NotFound(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"name":"not_found","message":"Domain dom_404 not found"}`))
	})

	_, err := c.GetDomain(context.Background(), "dom_404")
	require.Error(t, err)
	require.True(t, IsNotFound(err))
	require.Contains(t, err.Error(), "Domain dom_404 not found")
}

func TestClient_5xx_NotMisclassifiedAsNotFound(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.GetDomain(context.Background(), "dom_x")
	require.Error(t, err)
	require.False(t, IsNotFound(err))
}
