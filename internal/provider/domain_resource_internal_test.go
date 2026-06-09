// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

// TestResolveTrackingBool pins the precedence rule that fixes the phantom
// drift bug: when Resend's GET /domains/:id omits open_tracking/click_tracking,
// the SDK decodes them as bool zero (false). Without this resolution the
// provider would clobber a true-in-state value on refresh and cause a
// permanent "false -> true" diff on every plan.
func TestResolveTrackingBool(t *testing.T) {
	t.Parallel()

	bTrue := true
	bFalse := false

	cases := []struct {
		name     string
		prior    types.Bool
		sdkValue bool
		extValue *bool
		want     types.Bool
	}{
		{
			name:     "ext non-nil wins over everything",
			prior:    types.BoolValue(false),
			sdkValue: false,
			extValue: &bTrue,
			want:     types.BoolValue(true),
		},
		{
			name:     "ext non-nil false also wins",
			prior:    types.BoolValue(true),
			sdkValue: true,
			extValue: &bFalse,
			want:     types.BoolValue(false),
		},
		{
			name:     "sdk true beats prior when ext absent",
			prior:    types.BoolValue(false),
			sdkValue: true,
			extValue: nil,
			want:     types.BoolValue(true),
		},
		{
			// This is the regression scenario: state said true, SDK
			// decoded false (because the API omitted the field), ext
			// also returned no value. Must keep prior to avoid drift.
			name:     "prior true preserved when sdk reports false and ext absent",
			prior:    types.BoolValue(true),
			sdkValue: false,
			extValue: nil,
			want:     types.BoolValue(true),
		},
		{
			name:     "prior false preserved when sdk reports false and ext absent",
			prior:    types.BoolValue(false),
			sdkValue: false,
			extValue: nil,
			want:     types.BoolValue(false),
		},
		{
			name:     "null prior falls back to false default",
			prior:    types.BoolNull(),
			sdkValue: false,
			extValue: nil,
			want:     types.BoolValue(false),
		},
		{
			name:     "unknown prior falls back to false default",
			prior:    types.BoolUnknown(),
			sdkValue: false,
			extValue: nil,
			want:     types.BoolValue(false),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolveTrackingBool(tc.prior, tc.sdkValue, tc.extValue)
			require.True(t, tc.want.Equal(got), "want %s, got %s", tc.want, got)
		})
	}
}
