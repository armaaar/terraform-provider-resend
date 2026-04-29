// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func webhookConfig(endpoint, events, status string) string {
	statusBlock := ""
	if status != "" {
		statusBlock = fmt.Sprintf("\n  status = %q", status)
	}
	return providerConfig + fmt.Sprintf(`
resource "resend_webhook" "test" {
  endpoint = %q
  events   = %s%s
}
`, endpoint, events, statusBlock)
}

func TestAccWebhookResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: create. Asserts id + signing_secret + default status.
			{
				Config: webhookConfig("https://example.com/webhook-create", `["email.sent","email.delivered"]`, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_webhook.test", "endpoint", "https://example.com/webhook-create"),
					resource.TestCheckResourceAttr("resend_webhook.test", "events.#", "2"),
					resource.TestCheckResourceAttrSet("resend_webhook.test", "id"),
					resource.TestCheckResourceAttrSet("resend_webhook.test", "signing_secret"),
					resource.TestCheckResourceAttrSet("resend_webhook.test", "created_at"),
					resource.TestCheckResourceAttr("resend_webhook.test", "status", "enabled"),
				),
			},
			// Step 2: change endpoint.
			{
				Config: webhookConfig("https://example.com/webhook-updated", `["email.sent","email.delivered"]`, ""),
				Check:  resource.TestCheckResourceAttr("resend_webhook.test", "endpoint", "https://example.com/webhook-updated"),
			},
			// Step 3: change the events list (different size + content).
			{
				Config: webhookConfig("https://example.com/webhook-updated", `["email.bounced"]`, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_webhook.test", "events.#", "1"),
					resource.TestCheckResourceAttr("resend_webhook.test", "events.0", "email.bounced"),
				),
			},
			// Step 4: flip status to disabled.
			{
				Config: webhookConfig("https://example.com/webhook-updated", `["email.bounced"]`, "disabled"),
				Check:  resource.TestCheckResourceAttr("resend_webhook.test", "status", "disabled"),
			},
			// Step 5: import. signing_secret round-trips because the SDK's Get
			// returns it.
			{
				ResourceName:      "resend_webhook.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
