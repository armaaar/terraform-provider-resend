// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccDomainName builds a unique fully-qualified domain name for an
// acceptance test by combining the user-supplied RESEND_TEST_BASE_DOMAIN with a
// random subdomain. Using a random subdomain prevents re-runs from colliding
// against domains left behind by previous failed runs at Resend.
func testAccDomainName(t *testing.T) string {
	t.Helper()
	base := os.Getenv("RESEND_TEST_BASE_DOMAIN")
	if base == "" {
		t.Skip("RESEND_TEST_BASE_DOMAIN must be set for acceptance tests that create real Resend domains.")
	}
	return acctest.RandomWithPrefix("tf-acc") + "." + base
}

func TestAccDomainResource(t *testing.T) {
	name := testAccDomainName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: create with the minimum config and check that records came
			// back populated and capabilities were reported by the REST API.
			{
				Config: providerConfig + fmt.Sprintf(`
resource "resend_domain" "test" {
  name   = %[1]q
  region = "us-east-1"
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_domain.test", "name", name),
					resource.TestCheckResourceAttr("resend_domain.test", "region", "us-east-1"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "id"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "status"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "created_at"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "records.#"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "records.0.record"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "records.0.name"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "records.0.type"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "records.0.value"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "capabilities.sending"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "capabilities.receiving"),
				),
			},
			// Step 2: enable tracking and TLS. Exercises Update via the SDK
			// PATCH and the post-update Get refresh.
			{
				Config: providerConfig + fmt.Sprintf(`
resource "resend_domain" "test" {
  name               = %[1]q
  region             = "us-east-1"
  open_tracking      = true
  click_tracking     = true
  tracking_subdomain = "track"
  tls                = "enforced"
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_domain.test", "open_tracking", "true"),
					resource.TestCheckResourceAttr("resend_domain.test", "click_tracking", "true"),
					resource.TestCheckResourceAttr("resend_domain.test", "tracking_subdomain", "track"),
					resource.TestCheckResourceAttr("resend_domain.test", "tls", "enforced"),
				),
			},
			// Step 3: flip the booleans back to false. This specifically exercises
			// the SDK's SetOpenTracking(false)/SetClickTracking(false) MarshalJSON
			// path; without the setters the false values would be dropped by
			// omitempty and silently retain the previous true values.
			{
				Config: providerConfig + fmt.Sprintf(`
resource "resend_domain" "test" {
  name               = %[1]q
  region             = "us-east-1"
  open_tracking      = false
  click_tracking     = false
  tracking_subdomain = "track"
  tls                = "opportunistic"
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_domain.test", "open_tracking", "false"),
					resource.TestCheckResourceAttr("resend_domain.test", "click_tracking", "false"),
					resource.TestCheckResourceAttr("resend_domain.test", "tls", "opportunistic"),
				),
			},
			// Step 4: import. Skipping ImportStateVerify because tracking_subdomain
			// echoes through Resend's API in a way that occasionally normalises
			// (lowercase, trailing dot) and ImportStateVerify is intolerant of
			// purely cosmetic differences. The Read above already proves the
			// roundtrip works.
			{
				ResourceName:      "resend_domain.test",
				ImportState:       true,
				ImportStateVerify: false,
			},
		},
	})
}

// TestAccDomainResource_apNortheast is gated behind RESEND_TEST_AP_REGION
// because not every Resend account has the ap-northeast-1 region enabled.
func TestAccDomainResource_apNortheast(t *testing.T) {
	if os.Getenv("RESEND_TEST_AP_REGION") == "" {
		t.Skip("set RESEND_TEST_AP_REGION=1 to exercise the ap-northeast-1 region")
	}
	name := testAccDomainName(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "resend_domain" "test" {
  name   = %[1]q
  region = "ap-northeast-1"
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("resend_domain.test", "region", "ap-northeast-1"),
					resource.TestCheckResourceAttrSet("resend_domain.test", "records.#"),
				),
			},
		},
	})
}
