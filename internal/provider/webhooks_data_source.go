// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/resend/resend-go/v3"
)

var _ datasource.DataSource = &WebhooksDataSource{}
var _ datasource.DataSourceWithConfigure = &WebhooksDataSource{}

func NewWebhooksDataSource() datasource.DataSource {
	return &WebhooksDataSource{}
}

type WebhooksDataSource struct {
	client *resend.Client
}

type WebhooksDataSourceModel struct {
	Webhooks types.List `tfsdk:"webhooks"`
}

type webhooksItem struct {
	Id        types.String `tfsdk:"id"`
	Endpoint  types.String `tfsdk:"endpoint"`
	Events    types.List   `tfsdk:"events"`
	Status    types.String `tfsdk:"status"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (d *WebhooksDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhooks"
}

func webhooksItemAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":         types.StringType,
		"endpoint":   types.StringType,
		"events":     types.ListType{ElemType: types.StringType},
		"status":     types.StringType,
		"created_at": types.StringType,
	}
}

func (d *WebhooksDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every Resend webhook in the account. The list endpoint omits `signing_secret` — fetch each webhook via `data.resend_webhook` if you need it.",
		Attributes: map[string]schema.Attribute{
			"webhooks": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All webhooks, in the order Resend returned them.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true},
						"endpoint":   schema.StringAttribute{Computed: true},
						"status":     schema.StringAttribute{Computed: true},
						"created_at": schema.StringAttribute{Computed: true},
						"events": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func (d *WebhooksDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WebhooksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WebhooksDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	all := []webhooksItem{}
	var cursor *string
	for {
		page, err := d.client.Webhooks.ListWithOptions(ctx, &resend.ListOptions{After: cursor})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list webhooks, got error: %s", err))
			return
		}
		for _, w := range page.Data {
			events, eventsDiags := types.ListValueFrom(ctx, types.StringType, w.Events)
			resp.Diagnostics.Append(eventsDiags...)
			if resp.Diagnostics.HasError() {
				return
			}
			all = append(all, webhooksItem{
				Id:        types.StringValue(w.Id),
				Endpoint:  types.StringValue(w.Endpoint),
				Events:    events,
				Status:    types.StringValue(w.Status),
				CreatedAt: types.StringValue(w.CreatedAt),
			})
		}
		if !page.HasMore || len(page.Data) == 0 {
			break
		}
		last := page.Data[len(page.Data)-1].Id
		cursor = &last
	}

	listVal, listDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: webhooksItemAttrTypes()}, all)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Webhooks = listVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
