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

var _ datasource.DataSource = &ApiKeyDataSource{}
var _ datasource.DataSourceWithConfigure = &ApiKeyDataSource{}

func NewApiKeyDataSource() datasource.DataSource {
	return &ApiKeyDataSource{}
}

type ApiKeyDataSource struct {
	client *resend.Client
}

type ApiKeyDataSourceModel struct {
	Id         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	CreatedAt  types.String `tfsdk:"created_at"`
	LastUsedAt types.String `tfsdk:"last_used_at"`
}

func (d *ApiKeyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (d *ApiKeyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing Resend API key by ID. Resend has no GET endpoint for individual keys, so this paginates the list endpoint and filters by id. The token itself is never returned by Resend after creation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the API key within Resend.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The API key name.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The date and time the key was created.",
				Computed:            true,
			},
			"last_used_at": schema.StringAttribute{
				MarkdownDescription: "The date and time the key was last used. Null if the key has never been used.",
				Computed:            true,
			},
		},
	}
}

func (d *ApiKeyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ApiKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ApiKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.Id.ValueString()
	var cursor *string
	for {
		page, err := d.client.ApiKeys.ListWithOptions(ctx, &resend.ListOptions{After: cursor})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list api keys, got error: %s", err))
			return
		}
		for i := range page.Data {
			if page.Data[i].Id == id {
				data.Name = types.StringValue(page.Data[i].Name)
				data.CreatedAt = types.StringValue(page.Data[i].CreatedAt)
				if page.Data[i].LastUsedAt != nil {
					data.LastUsedAt = types.StringValue(*page.Data[i].LastUsedAt)
				} else {
					data.LastUsedAt = types.StringNull()
				}
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}
		if !page.HasMore || len(page.Data) == 0 {
			resp.Diagnostics.AddError(
				"API key not found",
				fmt.Sprintf("No API key with id %q exists in the Resend account.", id),
			)
			return
		}
		last := page.Data[len(page.Data)-1].Id
		cursor = &last
	}
}
