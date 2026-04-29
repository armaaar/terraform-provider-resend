// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccApiKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "resend_api_key" "test" {
  name = "tf-acc-test"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_api_key.test", "name", "tf-acc-test"),
					resource.TestCheckResourceAttrSet("resend_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("resend_api_key.test", "token"),
				),
			},
			// Refresh-only step: exercises Read on its own and asserts no diff.
			// This is what would have caught the previous no-op Read in CI.
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_api_key.test", "name", "tf-acc-test"),
				),
			},
			// Import — token cannot round-trip (Resend never re-exposes it), so
			// ImportStateVerify is off.
			{
				ResourceName:            "resend_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token"},
			},
		},
	})
}
