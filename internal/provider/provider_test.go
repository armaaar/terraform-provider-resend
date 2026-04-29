// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/require"
)

var providerConfig = fmt.Sprintf(`
provider "resend" {
  api_key = "%s"
}
`, os.Getenv("RESEND_API_KEY"))

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"resend": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck is intended to be passed as the PreCheck callback on a
// resource.TestCase so acceptance tests fail fast when the API key is not set.
// It is intentionally lowercase so `go test ./...` does not auto-discover it
// as a test (the prior name TestAccPreCheck made `go test` fail without a
// real key).
//
//nolint:unused // wired into resource.TestCase{PreCheck:} in the acceptance test phase
func testAccPreCheck(t *testing.T) {
	require.NotEmpty(t, os.Getenv("RESEND_API_KEY"))
}
