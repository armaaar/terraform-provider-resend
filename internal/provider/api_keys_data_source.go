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

var _ datasource.DataSource = &ApiKeysDataSource{}
var _ datasource.DataSourceWithConfigure = &ApiKeysDataSource{}

func NewApiKeysDataSource() datasource.DataSource {
	return &ApiKeysDataSource{}
}

type ApiKeysDataSource struct {
	client *resend.Client
}

type ApiKeysDataSourceModel struct {
	ApiKeys types.List `tfsdk:"api_keys"`
}

type apiKeysItem struct {
	Id         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	CreatedAt  types.String `tfsdk:"created_at"`
	LastUsedAt types.String `tfsdk:"last_used_at"`
}

func (d *ApiKeysDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_keys"
}

func apiKeysItemAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":           types.StringType,
		"name":         types.StringType,
		"created_at":   types.StringType,
		"last_used_at": types.StringType,
	}
}

func (d *ApiKeysDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every Resend API key in the account. The token is never returned (Resend only exposes it at creation time).",
		Attributes: map[string]schema.Attribute{
			"api_keys": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All API keys, in the order Resend returned them.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true},
						"name":         schema.StringAttribute{Computed: true},
						"created_at":   schema.StringAttribute{Computed: true},
						"last_used_at": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ApiKeysDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ApiKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApiKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	all := []apiKeysItem{}
	var cursor *string
	for {
		page, err := d.client.ApiKeys.ListWithOptions(ctx, &resend.ListOptions{After: cursor})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list api keys, got error: %s", err))
			return
		}
		for _, k := range page.Data {
			item := apiKeysItem{
				Id:         types.StringValue(k.Id),
				Name:       types.StringValue(k.Name),
				CreatedAt:  types.StringValue(k.CreatedAt),
				LastUsedAt: types.StringNull(),
			}
			if k.LastUsedAt != nil {
				item.LastUsedAt = types.StringValue(*k.LastUsedAt)
			}
			all = append(all, item)
		}
		if !page.HasMore || len(page.Data) == 0 {
			break
		}
		last := page.Data[len(page.Data)-1].Id
		cursor = &last
	}

	listVal, listDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: apiKeysItemAttrTypes()}, all)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ApiKeys = listVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
