// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/resend/resend-go/v3"
)

var _ datasource.DataSource = &WebhookDataSource{}
var _ datasource.DataSourceWithConfigure = &WebhookDataSource{}

func NewWebhookDataSource() datasource.DataSource {
	return &WebhookDataSource{}
}

type WebhookDataSource struct {
	client *resend.Client
}

type WebhookDataSourceModel struct {
	Id            types.String `tfsdk:"id"`
	Endpoint      types.String `tfsdk:"endpoint"`
	Events        types.List   `tfsdk:"events"`
	Status        types.String `tfsdk:"status"`
	CreatedAt     types.String `tfsdk:"created_at"`
	SigningSecret types.String `tfsdk:"signing_secret"`
}

func (d *WebhookDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (d *WebhookDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing Resend webhook by ID. The HMAC `signing_secret` is returned by Resend's GET endpoint, so importing-then-reading is a viable adoption path.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the webhook within Resend.",
				Required:            true,
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "HTTPS URL Resend POSTs events to.",
				Computed:            true,
			},
			"events": schema.ListAttribute{
				MarkdownDescription: "Event types subscribed to.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Webhook status (`enabled` or `disabled`).",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The date and time the webhook was created.",
				Computed:            true,
			},
			"signing_secret": schema.StringAttribute{
				MarkdownDescription: "HMAC signing secret used to verify webhook requests. Sensitive credential material.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (d *WebhookDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clients, ok := req.ProviderData.(*providerClients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected *providerClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = clients.sdk
}

func (d *WebhookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WebhookDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wh, err := d.client.Webhooks.GetWithContext(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read webhook, got error: %s", err))
		return
	}

	data.Endpoint = types.StringValue(wh.Endpoint)
	data.Status = types.StringValue(wh.Status)
	data.CreatedAt = types.StringValue(wh.CreatedAt)
	data.SigningSecret = types.StringValue(wh.SigningSecret)

	events, eventsDiags := types.ListValueFrom(ctx, types.StringType, wh.Events)
	resp.Diagnostics.Append(eventsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Events = events

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
