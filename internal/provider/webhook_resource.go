// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/resend/resend-go/v3"
)

var _ resource.Resource = &WebhookResource{}
var _ resource.ResourceWithImportState = &WebhookResource{}

func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

type WebhookResource struct {
	client *resend.Client
}

type WebhookResourceModel struct {
	Id            types.String `tfsdk:"id"`
	Endpoint      types.String `tfsdk:"endpoint"`
	Events        types.List   `tfsdk:"events"`
	Status        types.String `tfsdk:"status"`
	CreatedAt     types.String `tfsdk:"created_at"`
	SigningSecret types.String `tfsdk:"signing_secret"`
}

func (r *WebhookResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Resend webhook subscription. Resend POSTs to `endpoint` for the listed `events`; verify each request with the HMAC `signing_secret`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the webhook within Resend.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "HTTPS URL Resend will POST events to.",
				Required:            true,
			},
			"events": schema.ListAttribute{
				MarkdownDescription: "Event types to subscribe to. Common values: `email.sent`, `email.delivered`, `email.delivery_delayed`, `email.bounced`, `email.complained`, `email.opened`, `email.clicked`, `email.failed`, `email.scheduled`, `email.received`, `email.suppressed`, `contact.created`, `contact.updated`, `contact.deleted`, `domain.created`, `domain.updated`, `domain.deleted`. See https://resend.com/docs/dashboard/webhooks/event-types for the authoritative list.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Webhook status. One of `enabled`, `disabled`. Defaults to `enabled` server-side.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The date and time the webhook was created at Resend.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"signing_secret": schema.StringAttribute{
				MarkdownDescription: "HMAC signing secret used to verify webhook requests. Returned on Create and Read; treat as sensitive credential material.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *WebhookResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
}

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var events []string
	resp.Diagnostics.Append(data.Events.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.Webhooks.CreateWithContext(ctx, &resend.CreateWebhookRequest{
		Endpoint: data.Endpoint.ValueString(),
		Events:   events,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create webhook, got error: %s", err))
		return
	}
	data.Id = types.StringValue(created.Id)
	data.SigningSecret = types.StringValue(created.SigningSecret)

	// Resend creates webhooks in 'enabled' state — if the user asked for
	// 'disabled', follow up with a PATCH.
	if !data.Status.IsNull() && !data.Status.IsUnknown() && data.Status.ValueString() != "" {
		desired := data.Status.ValueString()
		if _, err := r.client.Webhooks.UpdateWithContext(ctx, created.Id, &resend.UpdateWebhookRequest{Status: &desired}); err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Webhook created but failed to apply status %q: %s", desired, err))
			return
		}
	}

	found, refreshDiags := r.refreshFromGet(ctx, &data)
	resp.Diagnostics.Append(refreshDiags...)
	if !found {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError(
				"Webhook disappeared",
				fmt.Sprintf("Resend returned 404 for webhook %s immediately after creating it. Retry the apply.", created.Id),
			)
		}
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, refreshDiags := r.refreshFromGet(ctx, &data)
	resp.Diagnostics.Append(refreshDiags...)
	if !found {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var events []string
	resp.Diagnostics.Append(data.Events.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := data.Endpoint.ValueString()
	updateReq := &resend.UpdateWebhookRequest{
		Endpoint: &endpoint,
		Events:   events,
	}
	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		s := data.Status.ValueString()
		updateReq.Status = &s
	}

	if _, err := r.client.Webhooks.UpdateWithContext(ctx, data.Id.ValueString(), updateReq); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update webhook, got error: %s", err))
		return
	}

	found, refreshDiags := r.refreshFromGet(ctx, &data)
	resp.Diagnostics.Append(refreshDiags...)
	if !found {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError(
				"Webhook disappeared",
				fmt.Sprintf("Resend returned 404 for webhook %s immediately after updating it. The webhook may have been deleted out of band.", data.Id.ValueString()),
			)
		}
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Webhooks.RemoveWithContext(ctx, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete webhook, got error: %s", err))
		return
	}
}

func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// refreshFromGet pulls the latest webhook state from Resend into data. Returns
// (false, nil) on 404 so callers can RemoveResource. Returns (false, diags) on
// other errors. Returns (true, diags) on success. signing_secret is only
// overwritten when Resend returns it (it does on Get, but not on List).
func (r *WebhookResource) refreshFromGet(ctx context.Context, data *WebhookResourceModel) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	wh, err := r.client.Webhooks.GetWithContext(ctx, data.Id.ValueString())
	if err != nil {
		if isWebhookNotFound(err) {
			return false, diags
		}
		diags.AddError("Client Error", fmt.Sprintf("Unable to read webhook, got error: %s", err))
		return false, diags
	}

	data.Endpoint = types.StringValue(wh.Endpoint)
	data.Status = types.StringValue(wh.Status)
	data.CreatedAt = types.StringValue(wh.CreatedAt)
	if wh.SigningSecret != "" {
		data.SigningSecret = types.StringValue(wh.SigningSecret)
	}
	events, eventsDiags := types.ListValueFrom(ctx, types.StringType, wh.Events)
	diags.Append(eventsDiags...)
	if eventsDiags.HasError() {
		return false, diags
	}
	data.Events = events
	return true, diags
}

// isWebhookNotFound matches Resend's 404 responses by string-sniffing the
// error message because the SDK doesn't expose a typed error. It's a hack but
// the pattern is stable: the API returns either "not_found" or a 404 status
// code somewhere in the message.
func isWebhookNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not_found") || strings.Contains(msg, "404")
}
