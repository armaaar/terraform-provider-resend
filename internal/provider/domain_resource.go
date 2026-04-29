// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/armaaar/terraform-provider-resend/internal/resendx"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/resend/resend-go/v3"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

// DomainResource defines the resource implementation.
type DomainResource struct {
	client *resend.Client
	ext    *resendx.Client
}

type record struct {
	Record   types.String `tfsdk:"record"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Value    types.String `tfsdk:"value"`
	Ttl      types.String `tfsdk:"ttl"`
	Status   types.String `tfsdk:"status"`
	Priority types.Int64  `tfsdk:"priority"`
}

// DomainResourceModel describes the resource data model.
type DomainResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Region      types.String `tfsdk:"region"`
	CreatedAt   types.String `tfsdk:"created_at"`
	Status      types.String `tfsdk:"status"`
	DnsProvider types.String `tfsdk:"dns_provider"`
	Records     types.List   `tfsdk:"records"`
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Add a new Domain.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the domain within Resend.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the domain you want to create",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The region where emails will be sent from. Possible values: `us-east-1` | `eu-west-1` | `sa-east-1`",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The date and time the domain was created",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The status of the domain. TODO: find out possible values",
				Computed:            true,
			},
			"dns_provider": schema.StringAttribute{
				MarkdownDescription: "The DNS provider used to configure the domain.",
				Computed:            true,
			},
			"records": schema.ListNestedAttribute{
				MarkdownDescription: "DNS records that must exist at the domain's DNS provider for Resend to verify and send through this domain. Pipe these straight into `cloudflare_record` (or your DNS provider of choice) with `for_each`.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"record": schema.StringAttribute{
							MarkdownDescription: "Resend's record class — one of `SPF`, `DKIM`, `Tracking`, `TrackingCAA`.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The hostname.",
							Computed:            true,
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "The DNS record type (e.g. `MX`, `TXT`, `CNAME`).",
							Computed:            true,
						},
						"ttl": schema.StringAttribute{
							MarkdownDescription: "The TTL.",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "The verification status of this record at Resend's last check.",
							Computed:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "The record value.",
							Computed:            true,
						},
						"priority": schema.Int64Attribute{
							MarkdownDescription: "Priority for MX records; null for record types without a priority.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

// recordObjectType is the Terraform type backing a single records[] element.
func recordObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"record":   types.StringType,
		"name":     types.StringType,
		"type":     types.StringType,
		"ttl":      types.StringType,
		"status":   types.StringType,
		"value":    types.StringType,
		"priority": types.Int64Type,
	}}
}

// recordsToList converts the SDK's []resend.Record into a Terraform list value.
// nil/empty input produces an empty list (not null) so plans don't churn.
func recordsToList(ctx context.Context, recs []resend.Record) (types.List, diag.Diagnostics) {
	models := make([]record, 0, len(recs))
	for _, r := range recs {
		m := record{
			Record:   types.StringValue(r.Record),
			Name:     types.StringValue(r.Name),
			Type:     types.StringValue(r.Type),
			Ttl:      types.StringValue(r.Ttl),
			Status:   types.StringValue(r.Status),
			Value:    types.StringValue(r.Value),
			Priority: types.Int64Null(),
		}
		if r.Priority != "" {
			if p, err := r.Priority.Int64(); err == nil {
				m.Priority = types.Int64Value(p)
			}
		}
		models = append(models, m)
	}
	return types.ListValueFrom(ctx, recordObjectType(), models)
}

func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*providerClients)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerClients, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = clients.sdk
	r.ext = clients.ext
}

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.Domains.CreateWithContext(ctx, &resend.CreateDomainRequest{
		Name:   data.Name.ValueString(),
		Region: data.Region.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create domain, got error: %s", err))
		return
	}
	data.Id = types.StringValue(domain.Id)
	data.CreatedAt = types.StringValue(domain.CreatedAt)
	data.Status = types.StringValue(domain.Status)
	data.DnsProvider = types.StringValue(domain.DnsProvider)
	data.Region = types.StringValue(domain.Region)

	recs, recsDiags := recordsToList(ctx, domain.Records)
	resp.Diagnostics.Append(recsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Records = recs

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.Domains.GetWithContext(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read domain, got error: %s", err))
		return
	}
	data.Name = types.StringValue(domain.Name)
	data.Region = types.StringValue(domain.Region)
	data.CreatedAt = types.StringValue(domain.CreatedAt)
	data.Status = types.StringValue(domain.Status)

	recs, recsDiags := recordsToList(ctx, domain.Records)
	resp.Diagnostics.Append(recsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Records = recs

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DomainResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update domain, got error: %s", "not implemented"))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Domains.RemoveWithContext(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete domain, got error: %s", err))
		return
	}

}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
