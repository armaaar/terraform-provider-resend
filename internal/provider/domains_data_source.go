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

var _ datasource.DataSource = &DomainsDataSource{}
var _ datasource.DataSourceWithConfigure = &DomainsDataSource{}

func NewDomainsDataSource() datasource.DataSource {
	return &DomainsDataSource{}
}

type DomainsDataSource struct {
	client *resend.Client
}

type DomainsDataSourceModel struct {
	Domains types.List `tfsdk:"domains"`
}

type domainsItem struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Region            types.String `tfsdk:"region"`
	CreatedAt         types.String `tfsdk:"created_at"`
	Status            types.String `tfsdk:"status"`
	OpenTracking      types.Bool   `tfsdk:"open_tracking"`
	ClickTracking     types.Bool   `tfsdk:"click_tracking"`
	TrackingSubdomain types.String `tfsdk:"tracking_subdomain"`
}

func (d *DomainsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func domainsItemAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                 types.StringType,
		"name":               types.StringType,
		"region":             types.StringType,
		"created_at":         types.StringType,
		"status":             types.StringType,
		"open_tracking":      types.BoolType,
		"click_tracking":     types.BoolType,
		"tracking_subdomain": types.StringType,
	}
}

func (d *DomainsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every Resend sending domain in the account. The list endpoint omits per-domain `records`, `capabilities` and `tls` — fetch each domain via `data.resend_domain` if you need those.",
		Attributes: map[string]schema.Attribute{
			"domains": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All domains, in the order Resend returned them.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                 schema.StringAttribute{Computed: true},
						"name":               schema.StringAttribute{Computed: true},
						"region":             schema.StringAttribute{Computed: true},
						"created_at":         schema.StringAttribute{Computed: true},
						"status":             schema.StringAttribute{Computed: true},
						"open_tracking":      schema.BoolAttribute{Computed: true},
						"click_tracking":     schema.BoolAttribute{Computed: true},
						"tracking_subdomain": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *DomainsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	all := []domainsItem{}
	var cursor *string
	for {
		page, err := d.client.Domains.ListWithOptions(ctx, &resend.ListOptions{After: cursor})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list domains, got error: %s", err))
			return
		}
		for _, dm := range page.Data {
			all = append(all, domainsItem{
				Id:                types.StringValue(dm.Id),
				Name:              types.StringValue(dm.Name),
				Region:            types.StringValue(dm.Region),
				CreatedAt:         types.StringValue(dm.CreatedAt),
				Status:            types.StringValue(dm.Status),
				OpenTracking:      types.BoolValue(dm.OpenTracking),
				ClickTracking:     types.BoolValue(dm.ClickTracking),
				TrackingSubdomain: types.StringValue(dm.TrackingSubdomain),
			})
		}
		if !page.HasMore || len(page.Data) == 0 {
			break
		}
		last := page.Data[len(page.Data)-1].Id
		cursor = &last
	}

	listVal, listDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: domainsItemAttrTypes()}, all)
	resp.Diagnostics.Append(listDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Domains = listVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
