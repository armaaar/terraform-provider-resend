// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resendx

import "context"

type webhookSecretResponse struct {
	SigningSecret string `json:"signing_secret"`
}

// GetWebhookSecret fetches the signing_secret for a webhook. The official SDK's
// Webhook struct omits this field even though Resend's REST API returns it on
// GET, so we read it here to populate it on Read and after import.
func (c *Client) GetWebhookSecret(ctx context.Context, id string) (string, error) {
	var r webhookSecretResponse
	if err := c.do(ctx, "GET", "webhooks/"+id, nil, &r); err != nil {
		return "", err
	}
	return r.SigningSecret, nil
}
