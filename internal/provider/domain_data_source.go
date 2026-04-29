// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/armaaar/terraform-provider-resend/internal/resendx"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/resend/resend-go/v3"
)

var _ datasource.DataSource = &DomainDataSource{}
var _ datasource.DataSourceWithConfigure = &DomainDataSource{}

func NewDomainDataSource() datasource.DataSource {
	return &DomainDataSource{}
}

type DomainDataSource struct {
	client *resend.Client
	ext    *resendx.Client
}

type DomainDataSourceModel struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Region            types.String `tfsdk:"region"`
	CreatedAt         types.String `tfsdk:"created_at"`
	Status            types.String `tfsdk:"status"`
	OpenTracking      types.Bool   `tfsdk:"open_tracking"`
	ClickTracking     types.Bool   `tfsdk:"click_tracking"`
	TrackingSubdomain types.String `tfsdk:"tracking_subdomain"`
	Tls               types.String `tfsdk:"tls"`
	Capabilities      types.Object `tfsdk:"capabilities"`
	Records           types.List   `tfsdk:"records"`
}

func (d *DomainDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *DomainDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an existing Resend sending domain by ID. Returns the same shape as the `resend_domain` resource minus the write-only `custom_return_path` field.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the domain within Resend.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The fully-qualified domain name.",
				Computed:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The region emails are sent from.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The date and time the domain was created at Resend.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The verification status of the domain.",
				Computed:            true,
			},
			"open_tracking": schema.BoolAttribute{
				MarkdownDescription: "Whether outbound links are rewritten to track open events.",
				Computed:            true,
			},
			"click_tracking": schema.BoolAttribute{
				MarkdownDescription: "Whether outbound links are rewritten to track click events.",
				Computed:            true,
			},
			"tracking_subdomain": schema.StringAttribute{
				MarkdownDescription: "The subdomain Resend uses to host tracking pixels and click redirects.",
				Computed:            true,
			},
			"tls": schema.StringAttribute{
				MarkdownDescription: "Outbound TLS policy. One of `enforced` or `opportunistic`. Null when no policy is set.",
				Computed:            true,
			},
			"capabilities": schema.SingleNestedAttribute{
				MarkdownDescription: "Resend-determined domain capabilities.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"sending":   schema.StringAttribute{Computed: true},
					"receiving": schema.StringAttribute{Computed: true},
				},
			},
			"records": schema.ListNestedAttribute{
				MarkdownDescription: "DNS records that must exist at the domain's DNS provider for Resend to verify and send through this domain.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"record":   schema.StringAttribute{Computed: true},
						"name":     schema.StringAttribute{Computed: true},
						"type":     schema.StringAttribute{Computed: true},
						"ttl":      schema.StringAttribute{Computed: true},
						"status":   schema.StringAttribute{Computed: true},
						"value":    schema.StringAttribute{Computed: true},
						"priority": schema.Int64Attribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *DomainDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.ext = clients.ext
}

func (d *DomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := d.client.Domains.GetWithContext(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read domain, got error: %s", err))
		return
	}

	data.Name = types.StringValue(domain.Name)
	data.Region = types.StringValue(domain.Region)
	data.CreatedAt = types.StringValue(domain.CreatedAt)
	data.Status = types.StringValue(domain.Status)
	data.OpenTracking = types.BoolValue(domain.OpenTracking)
	data.ClickTracking = types.BoolValue(domain.ClickTracking)
	data.TrackingSubdomain = types.StringValue(domain.TrackingSubdomain)

	recs, recsDiags := recordsToList(ctx, domain.Records)
	resp.Diagnostics.Append(recsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Records = recs

	// Pull tls + capabilities from the REST API — the SDK omits them.
	data.Tls = types.StringNull()
	data.Capabilities = types.ObjectNull(capabilitiesAttrTypes())
	ext, err := d.ext.GetDomain(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Could not read supplemental domain fields",
			fmt.Sprintf("Resend's REST API returned an error when fetching capabilities/tls for domain %s: %s. Those fields will be null in state.", data.Id.ValueString(), err),
		)
	} else {
		if ext.Tls != "" {
			data.Tls = types.StringValue(ext.Tls)
		}
		caps, capsDiags := capabilitiesToObject(ext.Capabilities)
		resp.Diagnostics.Append(capsDiags...)
		data.Capabilities = caps
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
