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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
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
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Region            types.String `tfsdk:"region"`
	CreatedAt         types.String `tfsdk:"created_at"`
	Status            types.String `tfsdk:"status"`
	OpenTracking      types.Bool   `tfsdk:"open_tracking"`
	ClickTracking     types.Bool   `tfsdk:"click_tracking"`
	TrackingSubdomain types.String `tfsdk:"tracking_subdomain"`
	Tls               types.String `tfsdk:"tls"`
	CustomReturnPath  types.String `tfsdk:"custom_return_path"`
	Capabilities      types.Object `tfsdk:"capabilities"`
	Records           types.List   `tfsdk:"records"`
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Resend sending domain. The `records` attribute is the practical reason this resource exists — pipe it into your DNS provider's record resource (e.g. `cloudflare_record`) to verify the domain.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the domain within Resend.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The fully-qualified domain name. Immutable.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The region emails will be sent from. One of `us-east-1`, `eu-west-1`, `sa-east-1`, `ap-northeast-1`. Defaults to `us-east-1` server-side. Immutable.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The date and time the domain was created at Resend.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The verification status of the domain. One of `not_started`, `pending`, `verified`, `failed`, `partially_verified`, `partially_failed`.",
				Computed:            true,
			},
			"open_tracking": schema.BoolAttribute{
				MarkdownDescription: "Whether Resend should rewrite outbound links to track open events. Defaults to `false` server-side.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"click_tracking": schema.BoolAttribute{
				MarkdownDescription: "Whether Resend should rewrite outbound links to track click events. Defaults to `false` server-side.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"tracking_subdomain": schema.StringAttribute{
				MarkdownDescription: "The subdomain Resend uses to host tracking pixels and click redirects. Must already be a subdomain of `name`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tls": schema.StringAttribute{
				MarkdownDescription: "Outbound TLS policy. One of `enforced` (require TLS, drop on failure) or `opportunistic` (try TLS, fall back to plaintext).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"custom_return_path": schema.StringAttribute{
				MarkdownDescription: "Custom bounce subdomain (sets the `Return-Path` header). Settable only at create time — Resend's API does not return it on read, so changes here force replacement.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"capabilities": schema.SingleNestedAttribute{
				MarkdownDescription: "Resend-determined capabilities of the domain. `sending` becomes `enabled` once the domain is verified; `receiving` becomes `enabled` when MX records resolve.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"sending": schema.StringAttribute{
						MarkdownDescription: "Sending capability — `enabled` or `disabled`.",
						Computed:            true,
					},
					"receiving": schema.StringAttribute{
						MarkdownDescription: "Receiving capability — `enabled` or `disabled`.",
						Computed:            true,
					},
				},
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

func capabilitiesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"sending":   types.StringType,
		"receiving": types.StringType,
	}
}

// capabilitiesToObject converts the resendx capabilities into a Terraform
// object value. A nil pointer (the API didn't return capabilities) yields a
// null object so plans don't churn against unknown server state.
func capabilitiesToObject(c *resendx.Capabilities) (types.Object, diag.Diagnostics) {
	if c == nil {
		return types.ObjectNull(capabilitiesAttrTypes()), nil
	}
	return types.ObjectValue(capabilitiesAttrTypes(), map[string]attr.Value{
		"sending":   types.StringValue(c.Sending),
		"receiving": types.StringValue(c.Receiving),
	})
}

// resolveTrackingBool picks the most authoritative value for an
// open_tracking/click_tracking attribute. Resend's GET /domains/:id has been
// observed in production to omit these fields; the SDK decodes a missing
// field as the bool zero value (false), which would clobber a true-in-config
// value on refresh and produce permanent phantom drift. Precedence:
//  1. the resendx-decoded pointer if non-nil (unambiguous: the API sent it).
//  2. the SDK's bool if true (a true cannot be a missing-field artifact).
//  3. the prior known value — state on Read, plan on Create/Update.
//  4. false, as the documented Resend default.
func resolveTrackingBool(prior types.Bool, sdkValue bool, extValue *bool) types.Bool {
	if extValue != nil {
		return types.BoolValue(*extValue)
	}
	if sdkValue {
		return types.BoolValue(true)
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		return prior
	}
	return types.BoolValue(false)
}

// applyExtState merges the resendx-supplied fields (capabilities, tls,
// open_tracking, click_tracking) into the model. Failure to fetch leaves the
// existing values intact and surfaces a warning rather than blocking the
// apply — these are read-only enrichments.
//
// sdkOpenTracking/sdkClickTracking are the values the SDK just decoded from
// its own GET /domains/:id call; priorOpenTracking/priorClickTracking are the
// values the provider should fall back to when neither source can
// authoritatively answer (state on Read, plan on Create/Update).
//
// Defaults Unknown -> null up front so even when resendx fails or Resend
// returns no value (a fresh domain has no TLS policy), the state is always
// "known". Without this, an Optional+Computed field whose plan value is
// Unknown would still be Unknown after apply and the framework rejects it.
func (r *DomainResource) applyExtState(
	ctx context.Context,
	data *DomainResourceModel,
	sdkOpenTracking, sdkClickTracking bool,
	priorOpenTracking, priorClickTracking types.Bool,
) diag.Diagnostics {
	var diags diag.Diagnostics

	if data.Tls.IsUnknown() {
		data.Tls = types.StringNull()
	}
	if data.Capabilities.IsUnknown() {
		data.Capabilities = types.ObjectNull(capabilitiesAttrTypes())
	}

	ext, err := r.ext.GetDomain(ctx, data.Id.ValueString())
	if err != nil {
		diags.AddWarning(
			"Could not read supplemental domain fields",
			fmt.Sprintf("Resend's REST API returned an error when fetching capabilities/tls for domain %s: %s. The provider will retain the previous values for these fields.", data.Id.ValueString(), err),
		)
		// Without resendx data we still resolve tracking from the SDK +
		// prior values so refresh never overwrites a true with a phantom
		// false when the SDK alone can't tell them apart.
		data.OpenTracking = resolveTrackingBool(priorOpenTracking, sdkOpenTracking, nil)
		data.ClickTracking = resolveTrackingBool(priorClickTracking, sdkClickTracking, nil)
		return diags
	}
	if ext.Tls != "" {
		data.Tls = types.StringValue(ext.Tls)
	}
	caps, capsDiags := capabilitiesToObject(ext.Capabilities)
	diags.Append(capsDiags...)
	data.Capabilities = caps
	data.OpenTracking = resolveTrackingBool(priorOpenTracking, sdkOpenTracking, ext.OpenTracking)
	data.ClickTracking = resolveTrackingBool(priorClickTracking, sdkClickTracking, ext.ClickTracking)
	return diags
}

func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &resend.CreateDomainRequest{
		Name:   data.Name.ValueString(),
		Region: data.Region.ValueString(),
	}
	if !data.CustomReturnPath.IsNull() && !data.CustomReturnPath.IsUnknown() {
		createReq.CustomReturnPath = data.CustomReturnPath.ValueString()
	}
	if !data.TrackingSubdomain.IsNull() && !data.TrackingSubdomain.IsUnknown() {
		createReq.TrackingSubdomain = data.TrackingSubdomain.ValueString()
	}
	if !data.OpenTracking.IsNull() && !data.OpenTracking.IsUnknown() {
		v := data.OpenTracking.ValueBool()
		createReq.OpenTracking = &v
	}
	if !data.ClickTracking.IsNull() && !data.ClickTracking.IsUnknown() {
		v := data.ClickTracking.ValueBool()
		createReq.ClickTracking = &v
	}

	created, err := r.client.Domains.CreateWithContext(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create domain, got error: %s", err))
		return
	}

	// Re-read via Get instead of trusting CreateDomainResponse: the SDK's
	// CreateDomainResponse declares camelCase JSON tags (createdAt,
	// dnsProvider) which don't match Resend's actual snake_case payload, so
	// fields like CreatedAt decode as empty strings. The Domain struct that
	// Get returns has the correct tags.
	data.Id = types.StringValue(created.Id)
	domain, err := r.client.Domains.GetWithContext(ctx, created.Id)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read domain after create, got error: %s", err))
		return
	}

	// Capture the planned tracking values BEFORE the SDK Get so applyExtState
	// has a known-good fallback if Resend's API omits them from the response.
	priorOpenTracking := data.OpenTracking
	priorClickTracking := data.ClickTracking

	data.Name = types.StringValue(domain.Name)
	data.Region = types.StringValue(domain.Region)
	data.CreatedAt = types.StringValue(domain.CreatedAt)
	data.Status = types.StringValue(domain.Status)
	data.TrackingSubdomain = types.StringValue(domain.TrackingSubdomain)

	recs, recsDiags := recordsToList(ctx, domain.Records)
	resp.Diagnostics.Append(recsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Records = recs

	// Pull capabilities, tls, and tracking flags from the REST API — the SDK
	// omits capabilities/tls entirely and silently zeroes the tracking flags
	// when Resend's response omits them.
	resp.Diagnostics.Append(r.applyExtState(ctx, &data, domain.OpenTracking, domain.ClickTracking, priorOpenTracking, priorClickTracking)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorOpenTracking := data.OpenTracking
	priorClickTracking := data.ClickTracking

	domain, err := r.client.Domains.GetWithContext(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read domain, got error: %s", err))
		return
	}

	data.Name = types.StringValue(domain.Name)
	data.Region = types.StringValue(domain.Region)
	data.CreatedAt = types.StringValue(domain.CreatedAt)
	data.Status = types.StringValue(domain.Status)
	data.TrackingSubdomain = types.StringValue(domain.TrackingSubdomain)

	recs, recsDiags := recordsToList(ctx, domain.Records)
	resp.Diagnostics.Append(recsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Records = recs

	resp.Diagnostics.Append(r.applyExtState(ctx, &data, domain.OpenTracking, domain.ClickTracking, priorOpenTracking, priorClickTracking)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state DomainResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only include fields that actually changed in the PATCH body. Resend
	// rate-limits some fields (tracking_subdomain is capped at 5 changes per
	// 24h) and treats any presence of the field as a change attempt — even if
	// the value matches. Use SetOpenTracking/SetClickTracking for the booleans
	// so the SDK's MarshalJSON sends explicit false values; the struct's
	// omitempty would otherwise drop a false value.
	updateReq := &resend.UpdateDomainRequest{}
	changed := false
	if !plan.OpenTracking.Equal(state.OpenTracking) {
		changed = true
		if !plan.OpenTracking.IsNull() && !plan.OpenTracking.IsUnknown() {
			updateReq.SetOpenTracking(plan.OpenTracking.ValueBool())
		}
	}
	if !plan.ClickTracking.Equal(state.ClickTracking) {
		changed = true
		if !plan.ClickTracking.IsNull() && !plan.ClickTracking.IsUnknown() {
			updateReq.SetClickTracking(plan.ClickTracking.ValueBool())
		}
	}
	if !plan.TrackingSubdomain.Equal(state.TrackingSubdomain) {
		changed = true
		if !plan.TrackingSubdomain.IsNull() && !plan.TrackingSubdomain.IsUnknown() {
			updateReq.TrackingSubdomain = plan.TrackingSubdomain.ValueString()
		}
	}
	if !plan.Tls.Equal(state.Tls) {
		changed = true
		if !plan.Tls.IsNull() && !plan.Tls.IsUnknown() {
			updateReq.Tls = plan.Tls.ValueString()
		}
	}

	data := plan
	if changed {
		if _, err := r.client.Domains.UpdateWithContext(ctx, plan.Id.ValueString(), updateReq); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update domain, got error: %s", err))
			return
		}
	}

	// Resend's PATCH response only echoes {object, id}; re-Get to refresh the
	// rest of the state.
	domain, err := r.client.Domains.GetWithContext(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read domain after update, got error: %s", err))
		return
	}
	priorOpenTracking := data.OpenTracking
	priorClickTracking := data.ClickTracking

	data.Name = types.StringValue(domain.Name)
	data.Region = types.StringValue(domain.Region)
	data.CreatedAt = types.StringValue(domain.CreatedAt)
	data.Status = types.StringValue(domain.Status)
	data.TrackingSubdomain = types.StringValue(domain.TrackingSubdomain)

	recs, recsDiags := recordsToList(ctx, domain.Records)
	resp.Diagnostics.Append(recsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Records = recs

	resp.Diagnostics.Append(r.applyExtState(ctx, &data, domain.OpenTracking, domain.ClickTracking, priorOpenTracking, priorClickTracking)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Domains.RemoveWithContext(ctx, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete domain, got error: %s", err))
		return
	}
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
