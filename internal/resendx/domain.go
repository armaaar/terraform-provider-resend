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

// Domain mirrors the subset of GET /domains/:id fields that the official SDK
// omits from its Domain struct. Other fields (name, region, records, …) are
// already covered by resend-go/v3 and intentionally left out here.
type Domain struct {
	Tls          string        `json:"tls,omitempty"`
	Capabilities *Capabilities `json:"capabilities,omitempty"`
}

// GetDomain fetches the supplemental fields for a domain.
func (c *Client) GetDomain(ctx context.Context, id string) (*Domain, error) {
	var d Domain
	if err := c.do(ctx, "GET", "domains/"+id, nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}
