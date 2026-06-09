// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resendx

import "context"

// Capabilities is Resend's per-domain capability report. Sending becomes
// "enabled" once the domain is verified; receiving becomes "enabled" once the
// MX records resolve.
type Capabilities struct {
	Sending   string `json:"sending"`
	Receiving string `json:"receiving"`
}

// Domain mirrors the subset of GET /domains/:id fields that we need a separate
// view of from the official SDK. `tls` and `capabilities` are not on the SDK's
// Domain at all. `open_tracking`/`click_tracking` ARE on the SDK's Domain — but
// as plain `bool`, so a response that omits the field is indistinguishable
// from one that returns `false`, which causes permanent phantom drift on
// refresh for domains that actually have tracking enabled. Decoding into
// `*bool` here keeps "field absent" distinct from "field is false" so the
// provider can fall back to prior state instead of clobbering it.
type Domain struct {
	Tls           string        `json:"tls,omitempty"`
	Capabilities  *Capabilities `json:"capabilities,omitempty"`
	OpenTracking  *bool         `json:"open_tracking,omitempty"`
	ClickTracking *bool         `json:"click_tracking,omitempty"`
}

// GetDomain fetches the supplemental fields for a domain.
func (c *Client) GetDomain(ctx context.Context, id string) (*Domain, error) {
	var d Domain
	if err := c.do(ctx, "GET", "domains/"+id, nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
