// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armaaar/terraform-provider-resend/internal/resendx"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/resend/resend-go/v3"
	"github.com/stretchr/testify/require"
)

// TestRecordsToList_Empty asserts that nil and zero-length slices both produce
// an empty (not null) list value, so plans don't churn between "no records" and
// "records unknown".
func TestRecordsToList_Empty(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"nil", "empty"} {
		t.Run(name, func(t *testing.T) {
			var input []resend.Record
			if name == "empty" {
				input = []resend.Record{}
			}
			got, diags := recordsToList(context.Background(), input)
			require.False(t, diags.HasError(), diags)
			require.False(t, got.IsNull())
			require.Equal(t, 0, len(got.Elements()))
		})
	}
}

// TestRecordsToList_WithoutPriority asserts a record without a priority field
// surfaces as types.Int64Null in the resulting object.
func TestRecordsToList_WithoutPriority(t *testing.T) {
	t.Parallel()
	got, diags := recordsToList(context.Background(), []resend.Record{
		{Record: resend.RecordTypeSPF, Name: "send", Type: "TXT", Ttl: "Auto", Status: "pending", Value: "v=spf1 include:resend.com ~all"},
	})
	require.False(t, diags.HasError(), diags)
	require.Equal(t, 1, len(got.Elements()))

	var models []record
	d := got.ElementsAs(context.Background(), &models, false)
	require.False(t, d.HasError(), d)
	require.Equal(t, "SPF", models[0].Record.ValueString())
	require.True(t, models[0].Priority.IsNull())
}

// TestRecordsToList_WithPriority asserts json.Number-as-string priorities are
// parsed into Int64Value.
func TestRecordsToList_WithPriority(t *testing.T) {
	t.Parallel()
	got, diags := recordsToList(context.Background(), []resend.Record{
		{Record: resend.RecordTypeDKIM, Name: "mail", Type: "MX", Ttl: "Auto", Status: "verified", Value: "feedback-smtp.us-east-1.amazonses.com", Priority: json.Number("10")},
	})
	require.False(t, diags.HasError(), diags)

	var models []record
	d := got.ElementsAs(context.Background(), &models, false)
	require.False(t, d.HasError(), d)
	require.False(t, models[0].Priority.IsNull())
	require.Equal(t, int64(10), models[0].Priority.ValueInt64())
}

// TestRecordsToList_MalformedPriority asserts a non-numeric priority falls back
// to types.Int64Null rather than failing the conversion.
func TestRecordsToList_MalformedPriority(t *testing.T) {
	t.Parallel()
	got, diags := recordsToList(context.Background(), []resend.Record{
		{Record: resend.RecordTypeDKIM, Name: "mail", Type: "MX", Priority: json.Number("not-a-number")},
	})
	require.False(t, diags.HasError(), diags)

	var models []record
	d := got.ElementsAs(context.Background(), &models, false)
	require.False(t, d.HasError(), d)
	require.True(t, models[0].Priority.IsNull())
}

// TestCapabilitiesToObject_Nil asserts nil input produces a null object.
func TestCapabilitiesToObject_Nil(t *testing.T) {
	t.Parallel()
	got, diags := capabilitiesToObject(nil)
	require.False(t, diags.HasError(), diags)
	require.True(t, got.IsNull())
}

// TestCapabilitiesToObject_Populated asserts both fields round-trip.
func TestCapabilitiesToObject_Populated(t *testing.T) {
	t.Parallel()
	got, diags := capabilitiesToObject(&resendx.Capabilities{Sending: "enabled", Receiving: "disabled"})
	require.False(t, diags.HasError(), diags)
	require.False(t, got.IsNull())
	attrs := got.Attributes()
	sending, ok := attrs["sending"].(types.String)
	require.True(t, ok)
	require.Equal(t, "enabled", sending.ValueString())
	receiving, ok := attrs["receiving"].(types.String)
	require.True(t, ok)
	require.Equal(t, "disabled", receiving.ValueString())
}
