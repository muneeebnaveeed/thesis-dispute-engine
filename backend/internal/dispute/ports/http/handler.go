// Package disputehttp serves the dispute API; routes come from docs/api/openapi.yaml via the generated strict server.
package disputehttp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/notice"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/ports/http/oapi"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/money"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/tenant"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/websession"
)

// Readiness reports dependency health; a non-nil error makes /readyz answer 503.
type Readiness func(ctx context.Context) error

// Handler implements the generated strict server.
type Handler struct {
	svc      *application.Service
	ready    Readiness
	sessions SessionStore
	tenants  TenantDirectory
	keys     KeyManager
}

// SessionStore is the opaque blob store behind /internal/sessions; nil disables those routes with a 404.
type SessionStore interface {
	Put(ctx context.Context, id uuid.UUID, b websession.Blob) error
	Get(ctx context.Context, id uuid.UUID) (websession.Blob, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteMatching(ctx context.Context, sel websession.Selector) (int64, error)
}

// TenantDirectory backs /internal/tenants; nil disables it with a 404.
type TenantDirectory interface {
	ActiveTenants(ctx context.Context) ([]disputepg.TenantSummary, error)
}

// WithTenants enables the internal tenant listing.
func WithTenants(d TenantDirectory) Option { return func(h *Handler) { h.tenants = d } }

// KeyManager backs /tenant-keys, always within the caller's tenant; nil disables it with a 404.
type KeyManager interface {
	ListForTenant(ctx context.Context, tenantID uuid.UUID) ([]disputepg.KeyRecord, error)
	Issue(ctx context.Context, tenantID uuid.UUID, label string, expiresAt *time.Time) (disputepg.KeyRecord, string, error)
	Revoke(ctx context.Context, tenantID, id uuid.UUID) error
}

// WithKeys enables tenant-key self-service.
func WithKeys(k KeyManager) Option { return func(h *Handler) { h.keys = k } }

// Option configures Mount.
type Option func(*Handler)

// WithSessions enables the internal session endpoints.
func WithSessions(s SessionStore) Option { return func(h *Handler) { h.sessions = s } }

var _ oapi.StrictServerInterface = (*Handler)(nil)

var (
	errMalformed = errs.New(errs.Invalid, "malformed-request", "the request could not be parsed")
	errContract  = errs.New(errs.Invalid, "contract-violation", "the request does not match the API contract")
)

// kin-openapi leaves "uuid" unvalidated by default and its optional pattern excludes v7; register a version-agnostic one.
var registerFormats = sync.OnceFunc(func() {
	openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(
		`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`))
})

// Mount registers every spec route on mux, with spec validation and route-named spans.
func Mount(mux *http.ServeMux, svc *application.Service, ready Readiness, opts ...Option) error {
	registerFormats()
	spec, err := oapi.GetSpec()
	if err != nil {
		return fmt.Errorf("disputehttp: load spec: %w", err)
	}
	spec.Servers = nil
	h := &Handler{svc: svc, ready: ready}
	for _, o := range opts {
		o(h)
	}
	strict := oapi.NewStrictHandlerWithOptions(h, nil, oapi.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			fail(w, r, detailed(errMalformed, err))
		},
		ResponseErrorHandlerFunc: fail,
	})
	oapi.HandlerWithOptions(strict, oapi.StdHTTPServerOptions{
		BaseRouter: mux,
		Middlewares: []oapi.MiddlewareFunc{
			httpserver.NameSpanByRoute,
			nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
				// Called only for operations the spec secures; auth.Bearer has already resolved the key by then.
				Options: openapi3filter.Options{AuthenticationFunc: func(ctx context.Context, in *openapi3filter.AuthenticationInput) error {
					return auth.Required(ctx, in.SecuritySchemeName)
				}},
				ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, _ nethttpmiddleware.ErrorHandlerOpts) {
					var sec *openapi3filter.SecurityRequirementsError
					if errors.As(err, &sec) {
						fail(w, r, auth.ErrUnauthenticated)
						return
					}
					fail(w, r, contractViolation(err))
				},
			}),
		},
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) { fail(w, r, detailed(errMalformed, err)) },
	})
	specJSON, err := spec.MarshalJSON()
	if err != nil {
		return fmt.Errorf("disputehttp: encode spec: %w", err)
	}
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(specJSON)
	})
	return nil
}

// PutSession stores the frontend server's opaque session blob.
func (h *Handler) PutSession(ctx context.Context, req oapi.PutSessionRequestObject) (oapi.PutSessionResponseObject, error) {
	if h.sessions == nil {
		return nil, application.ErrNotFound
	}
	var tenantID *uuid.UUID
	if req.Body.TenantId != nil {
		t := *req.Body.TenantId
		tenantID = &t
	}
	blob := websession.Blob{TenantID: tenantID, Ciphertext: req.Body.Ciphertext, ExpiresAt: req.Body.ExpiresAt}
	if req.Body.Subject != nil {
		blob.Subject = *req.Body.Subject
	}
	if req.Body.Sid != nil {
		blob.Sid = *req.Body.Sid
	}
	if err := h.sessions.Put(ctx, req.SessionId, blob); err != nil {
		return nil, err
	}
	return oapi.PutSession204Response{}, nil
}

// GetSession returns a live blob.
func (h *Handler) GetSession(ctx context.Context, req oapi.GetSessionRequestObject) (oapi.GetSessionResponseObject, error) {
	if h.sessions == nil {
		return nil, application.ErrNotFound
	}
	b, err := h.sessions.Get(ctx, req.SessionId)
	if err != nil {
		return nil, err
	}
	out := oapi.GetSession200JSONResponse{Ciphertext: b.Ciphertext, ExpiresAt: b.ExpiresAt}
	if b.TenantID != nil {
		t := *b.TenantID
		out.TenantId = &t
	}
	if b.Subject != "" {
		out.Subject = &b.Subject
	}
	if b.Sid != "" {
		out.Sid = &b.Sid
	}
	return out, nil
}

// DeleteSessions ends sessions by realm session id, or by tenant and subject.
func (h *Handler) DeleteSessions(ctx context.Context, req oapi.DeleteSessionsRequestObject) (oapi.DeleteSessionsResponseObject, error) {
	if h.sessions == nil {
		return nil, application.ErrNotFound
	}
	sel := websession.Selector{}
	if req.Params.Sid != nil {
		sel.Sid = *req.Params.Sid
	}
	if req.Params.TenantId != nil {
		t := *req.Params.TenantId
		sel.TenantID = &t
	}
	if req.Params.Subject != nil {
		sel.Subject = *req.Params.Subject
	}
	if sel.Sid == "" && sel.TenantID == nil {
		return nil, errs.New(errs.Invalid, "contract-violation", "give sid, or tenantId with an optional subject")
	}
	n, err := h.sessions.DeleteMatching(ctx, sel)
	if err != nil {
		return nil, err
	}
	return oapi.DeleteSessions200JSONResponse{Deleted: int(n)}, nil
}

// ListTenants is the sign-in page's directory.
func (h *Handler) ListTenants(ctx context.Context, _ oapi.ListTenantsRequestObject) (oapi.ListTenantsResponseObject, error) {
	if h.tenants == nil {
		return nil, application.ErrNotFound
	}
	list, err := h.tenants.ActiveTenants(ctx)
	if err != nil {
		return nil, err
	}
	out := make(oapi.ListTenants200JSONResponse, 0, len(list))
	for _, t := range list {
		item := oapi.TenantSummary{Id: t.ID, Slug: t.Slug, Name: t.Name, EmailDomains: t.EmailDomains}
		if t.Issuer != "" {
			issuer := t.Issuer
			item.Issuer = &issuer
		}
		out = append(out, item)
	}
	return out, nil
}

// DeleteSession removes a session.
func (h *Handler) DeleteSession(ctx context.Context, req oapi.DeleteSessionRequestObject) (oapi.DeleteSessionResponseObject, error) {
	if h.sessions == nil {
		return nil, application.ErrNotFound
	}
	if err := h.sessions.Delete(ctx, req.SessionId); err != nil {
		return nil, err
	}
	return oapi.DeleteSession204Response{}, nil
}

// keyView maps a record onto the contract shape.
func keyView(r disputepg.KeyRecord, now time.Time) oapi.TenantKey {
	return oapi.TenantKey{Id: r.ID, Prefix: r.Prefix, Label: r.Label, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt,
		ExpiresAt: r.ExpiresAt, RevokedAt: r.RevokedAt, Status: oapi.TenantKeyStatus(r.Status(now))}
}

// tenantAdmin is the guard shared by the key operations: an analyst token with the tenant-admin role, and a tenant.
func (h *Handler) tenantAdmin(ctx context.Context) (uuid.UUID, error) {
	if h.keys == nil {
		return uuid.Nil, application.ErrNotFound
	}
	if err := auth.RequireRole(ctx, auth.RoleTenantAdmin); err != nil {
		return uuid.Nil, err
	}
	id, ok := tenant.IDFrom(ctx)
	if !ok {
		return uuid.Nil, tenant.ErrMissing
	}
	return id, nil
}

// ListTenantKeys shows a tenant admin their tenant's keys.
func (h *Handler) ListTenantKeys(ctx context.Context, _ oapi.ListTenantKeysRequestObject) (oapi.ListTenantKeysResponseObject, error) {
	tid, err := h.tenantAdmin(ctx)
	if err != nil {
		return nil, err
	}
	recs, err := h.keys.ListForTenant(ctx, tid)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make(oapi.ListTenantKeys200JSONResponse, 0, len(recs))
	for _, r := range recs {
		out = append(out, keyView(r, now))
	}
	return out, nil
}

// CreateTenantKey issues a key and returns the secret exactly once.
func (h *Handler) CreateTenantKey(ctx context.Context, req oapi.CreateTenantKeyRequestObject) (oapi.CreateTenantKeyResponseObject, error) {
	tid, err := h.tenantAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body.ExpiresAt != nil && !req.Body.ExpiresAt.After(time.Now()) {
		return nil, errs.New(errs.Invalid, "contract-violation", "expiresAt must be in the future").
			WithFields(errs.FieldError{Field: "/expiresAt", Message: "must be in the future"})
	}
	rec, secret, err := h.keys.Issue(ctx, tid, strings.TrimSpace(req.Body.Label), req.Body.ExpiresAt)
	if err != nil {
		return nil, err
	}
	v := keyView(rec, time.Now())
	return oapi.CreateTenantKey201JSONResponse{Id: v.Id, Prefix: v.Prefix, Label: v.Label, CreatedAt: v.CreatedAt,
		LastUsedAt: v.LastUsedAt, ExpiresAt: v.ExpiresAt, RevokedAt: v.RevokedAt, Status: oapi.IssuedTenantKeyStatus(v.Status), Secret: secret}, nil
}

// RevokeTenantKey ends one of the tenant's keys.
func (h *Handler) RevokeTenantKey(ctx context.Context, req oapi.RevokeTenantKeyRequestObject) (oapi.RevokeTenantKeyResponseObject, error) {
	tid, err := h.tenantAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.keys.Revoke(ctx, tid, req.KeyId); err != nil {
		return nil, err
	}
	return oapi.RevokeTenantKey204Response{}, nil
}

// GetHealthz is liveness only.
func (h *Handler) GetHealthz(context.Context, oapi.GetHealthzRequestObject) (oapi.GetHealthzResponseObject, error) {
	return oapi.GetHealthz200JSONResponse{Status: oapi.HealthStatusOk}, nil
}

// GetReadyz reports whether the database answers.
func (h *Handler) GetReadyz(ctx context.Context, _ oapi.GetReadyzRequestObject) (oapi.GetReadyzResponseObject, error) {
	if err := h.ready(ctx); err != nil {
		checks := map[string]string{"postgres": "unavailable"}
		return oapi.GetReadyz503JSONResponse{Status: oapi.HealthStatusUnavailable, Checks: &checks}, nil
	}
	checks := map[string]string{"postgres": "ok"}
	return oapi.GetReadyz200JSONResponse{Status: oapi.HealthStatusOk, Checks: &checks}, nil
}

// CreateDispute opens a dispute; the idempotency hash covers the parsed body, so key order in the wire JSON does not matter.
func (h *Handler) CreateDispute(ctx context.Context, req oapi.CreateDisputeRequestObject) (oapi.CreateDisputeResponseObject, error) {
	body, _ := json.Marshal(req.Body)
	var reason string
	if req.Body.Reason != nil {
		reason = string(*req.Body.Reason)
	}
	res, err := h.svc.CreateDispute(ctx, application.CreateDisputeInput{
		TransactionID: req.Body.TransactionId,
		Reason:        reason,
		Actor:         orDefault(req.Body.Actor, "customer"),
		Idempotency:   idempotency(req.Params.IdempotencyKey, body),
	})
	if err != nil {
		p := h.problem(ctx, "/disputes", err, uuid.Nil)
		switch p.Status {
		case http.StatusNotFound:
			return oapi.CreateDispute404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusUnprocessableEntity:
			return oapi.CreateDispute422ApplicationProblemPlusJSONResponse{UnprocessableApplicationProblemPlusJSONResponse: oapi.UnprocessableApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusBadRequest:
			return oapi.CreateDispute400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: oapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return oapi.CreateDispute201JSONResponse(toAPI(res.View)), nil
}

// GetDispute returns state and log.
func (h *Handler) GetDispute(ctx context.Context, req oapi.GetDisputeRequestObject) (oapi.GetDisputeResponseObject, error) {
	view, err := h.svc.GetDispute(ctx, req.DisputeId)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			p := h.problem(ctx, "/disputes/"+req.DisputeId.String(), err, uuid.Nil)
			return oapi.GetDispute404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return oapi.GetDispute200JSONResponse(toAPI(view)), nil
}

// ListEmailTemplates returns the analyst's catalogue for one dispute.
func (h *Handler) ListEmailTemplates(ctx context.Context, req oapi.ListEmailTemplatesRequestObject) (oapi.ListEmailTemplatesResponseObject, error) {
	cat, err := h.svc.ListEmailTemplates(ctx, req.DisputeId)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			p := h.problem(ctx, "/disputes/"+req.DisputeId.String()+"/email-templates", err, uuid.Nil)
			return oapi.ListEmailTemplates404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	out := oapi.EmailTemplates{Templates: make([]oapi.EmailTemplate, 0, len(cat.Templates))}
	out.Facts.Customer, out.Facts.Bank, out.Facts.Amount, out.Facts.Merchant, out.Facts.Dispute = cat.Facts.Customer, cat.Facts.Bank, cat.Facts.Amount, cat.Facts.Merchant, cat.Facts.Dispute
	out.Facts.Today = openapi_types.Date{Time: cat.Facts.Today}
	for _, t := range cat.Templates {
		out.Templates = append(out.Templates, templateOf(t))
	}
	return oapi.ListEmailTemplates200JSONResponse(out), nil
}

// ComposeEmail sends a templated email as the signed-in analyst; tenant keys are refused.
func (h *Handler) ComposeEmail(ctx context.Context, req oapi.ComposeEmailRequestObject) (oapi.ComposeEmailResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, auth.ErrForbidden.WithDetail("emails to customers are written by analysts, not by tenant keys")
	}
	actor := principal.Email
	if actor == "" {
		actor = principal.Subject
	}
	var attachments []uuid.UUID
	if req.Body.Attachments != nil {
		attachments = *req.Body.Attachments
	}
	view, err := h.svc.ComposeEmail(ctx, application.ComposeEmailInput{DisputeID: req.DisputeId, Template: domain.NoticeKind(req.Body.Template),
		Fields: req.Body.Fields, Attachments: attachments, Actor: actor})
	if err != nil {
		p := h.problem(ctx, "/disputes/"+req.DisputeId.String()+"/notices", err, req.DisputeId)
		switch p.Status {
		case http.StatusNotFound:
			return oapi.ComposeEmail404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusUnprocessableEntity:
			return oapi.ComposeEmail422ApplicationProblemPlusJSONResponse{UnprocessableApplicationProblemPlusJSONResponse: oapi.UnprocessableApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusBadRequest:
			return oapi.ComposeEmail400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: oapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return oapi.ComposeEmail201JSONResponse(toAPI(view)), nil
}

// UploadAttachment stores one file from a multipart form as a draft for the dispute.
func (h *Handler) UploadAttachment(ctx context.Context, req oapi.UploadAttachmentRequestObject) (oapi.UploadAttachmentResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, auth.ErrForbidden.WithDetail("attachments are added by analysts, not by tenant keys")
	}
	var in application.UploadInput
	in.DisputeID, in.Actor = req.DisputeId, principal.Email
	for {
		part, err := req.Body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errs.New(errs.Invalid, "malformed-request", "the upload is not a valid multipart form").WithFields(errs.FieldError{Field: "body", Message: err.Error()})
		}
		if part.FormName() != "file" {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(part, application.MaxAttachmentBytes+1))
		if err != nil {
			return nil, err
		}
		in.Filename, in.ContentType, in.Content = part.FileName(), part.Header.Get("Content-Type"), content
	}
	a, err := h.svc.Upload(ctx, in)
	if err != nil {
		p := h.problem(ctx, "/disputes/"+req.DisputeId.String()+"/attachments", err, req.DisputeId)
		switch p.Status {
		case http.StatusNotFound:
			return oapi.UploadAttachment404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusUnprocessableEntity:
			return oapi.UploadAttachment422ApplicationProblemPlusJSONResponse{UnprocessableApplicationProblemPlusJSONResponse: oapi.UnprocessableApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusBadRequest:
			return oapi.UploadAttachment400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: oapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return oapi.UploadAttachment201JSONResponse(oapi.Attachment{Id: a.ID, Filename: a.Filename, ContentType: a.ContentType, Size: a.Size}), nil
}

// fileResponse serves an attachment with its own content type and name rather than the generic octet stream.
type fileResponse struct{ a application.Attachment }

func (r fileResponse) VisitGetAttachmentResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", r.a.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(r.a.Content)))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": r.a.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.a.Content)
	return err
}

// GetAttachment returns a file of the dispute.
func (h *Handler) GetAttachment(ctx context.Context, req oapi.GetAttachmentRequestObject) (oapi.GetAttachmentResponseObject, error) {
	a, err := h.svc.GetAttachment(ctx, req.DisputeId, req.AttachmentId)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			p := h.problem(ctx, fmt.Sprintf("/disputes/%s/attachments/%s", req.DisputeId, req.AttachmentId), err, uuid.Nil)
			return oapi.GetAttachment404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return fileResponse{a: a}, nil
}

// ResendNotice queues an email again as the signed-in analyst.
func (h *Handler) ResendNotice(ctx context.Context, req oapi.ResendNoticeRequestObject) (oapi.ResendNoticeResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, auth.ErrForbidden.WithDetail("emails to customers are sent by analysts, not by tenant keys")
	}
	actor := principal.Email
	if actor == "" {
		actor = principal.Subject
	}
	view, err := h.svc.Resend(ctx, application.ResendInput{DisputeID: req.DisputeId, NoticeID: req.NoticeId, Actor: actor})
	if err != nil {
		p := h.problem(ctx, fmt.Sprintf("/disputes/%s/notices/%d/resend", req.DisputeId, req.NoticeId), err, req.DisputeId)
		switch p.Status {
		case http.StatusNotFound:
			return oapi.ResendNotice404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusConflict:
			return oapi.ResendNotice409ApplicationProblemPlusJSONResponse(p), nil
		}
		return nil, err
	}
	return oapi.ResendNotice201JSONResponse(toAPI(view)), nil
}

// templateOf maps a notice template to the API shape.
func templateOf(t notice.Template) oapi.EmailTemplate {
	fields := make([]oapi.TemplateField, 0, len(t.Fields))
	for _, f := range t.Fields {
		tf := oapi.TemplateField{Id: f.ID, Label: f.Label, Type: oapi.FieldType(f.Type), Required: f.Required, Min: f.Min, Max: f.Max}
		if f.List {
			list := true
			tf.List = &list
		}
		if f.Default != "" {
			d := f.Default
			tf.Default = &d
		}
		if len(f.Options) > 0 {
			opts := make([]oapi.TemplateOption, 0, len(f.Options))
			for _, o := range f.Options {
				opts = append(opts, oapi.TemplateOption{Key: o.Key, Label: o.Label, Text: o.Text})
			}
			tf.Options = &opts
		}
		fields = append(fields, tf)
	}
	return oapi.EmailTemplate{Kind: oapi.NoticeKind(t.Kind), Label: t.Label, Description: t.Description, Letter: t.Letter, Fields: fields, Subject: t.Subject, Paragraphs: t.Paragraphs}
}

func overrideOf(o *notice.Override) *oapi.TemplateOverride {
	if o == nil {
		return nil
	}
	out := oapi.TemplateOverride{}
	if o.Label != "" {
		out.Label = &o.Label
	}
	if o.Description != "" {
		out.Description = &o.Description
	}
	if o.Subject != "" {
		out.Subject = &o.Subject
	}
	if len(o.Paragraphs) > 0 {
		out.Paragraphs = &o.Paragraphs
	}
	if len(o.OptionTexts) > 0 {
		out.OptionTexts = &o.OptionTexts
	}
	return &out
}

func settingOf(s application.TemplateSetting) oapi.TemplateSetting {
	out := oapi.TemplateSetting{Base: templateOf(s.Base), Effective: templateOf(s.Effective), Override: overrideOf(s.Override), UpdatedAt: s.UpdatedAt}
	if s.UpdatedBy != "" {
		by := s.UpdatedBy
		out.UpdatedBy = &by
	}
	return out
}

// ListTenantTemplates is the tenant admin's view of every analyst email kind.
func (h *Handler) ListTenantTemplates(ctx context.Context, _ oapi.ListTenantTemplatesRequestObject) (oapi.ListTenantTemplatesResponseObject, error) {
	if err := auth.RequireRole(ctx, auth.RoleTenantAdmin); err != nil {
		return nil, err
	}
	settings, err := h.svc.ListTemplateSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make(oapi.ListTenantTemplates200JSONResponse, 0, len(settings))
	for _, s := range settings {
		out = append(out, settingOf(s))
	}
	return out, nil
}

// PutTenantTemplate stores the tenant's wording for one kind.
func (h *Handler) PutTenantTemplate(ctx context.Context, req oapi.PutTenantTemplateRequestObject) (oapi.PutTenantTemplateResponseObject, error) {
	if err := auth.RequireRole(ctx, auth.RoleTenantAdmin); err != nil {
		return nil, err
	}
	principal, _ := auth.PrincipalFrom(ctx)
	actor := principal.Email
	if actor == "" {
		actor = principal.Subject
	}
	o := notice.Override{}
	if req.Body.Label != nil {
		o.Label = *req.Body.Label
	}
	if req.Body.Description != nil {
		o.Description = *req.Body.Description
	}
	if req.Body.Subject != nil {
		o.Subject = *req.Body.Subject
	}
	if req.Body.Paragraphs != nil {
		o.Paragraphs = *req.Body.Paragraphs
	}
	if req.Body.OptionTexts != nil {
		o.OptionTexts = *req.Body.OptionTexts
	}
	setting, err := h.svc.PutTemplateSetting(ctx, domain.NoticeKind(req.Kind), o, actor)
	if err != nil {
		p := h.problem(ctx, "/tenant-templates/"+string(req.Kind), err, uuid.Nil)
		switch p.Status {
		case http.StatusUnprocessableEntity:
			return oapi.PutTenantTemplate422ApplicationProblemPlusJSONResponse{UnprocessableApplicationProblemPlusJSONResponse: oapi.UnprocessableApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusBadRequest:
			return oapi.PutTenantTemplate400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: oapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return oapi.PutTenantTemplate200JSONResponse(settingOf(setting)), nil
}

// DeleteTenantTemplate reverts one kind to the base wording.
func (h *Handler) DeleteTenantTemplate(ctx context.Context, req oapi.DeleteTenantTemplateRequestObject) (oapi.DeleteTenantTemplateResponseObject, error) {
	if err := auth.RequireRole(ctx, auth.RoleTenantAdmin); err != nil {
		return nil, err
	}
	if err := h.svc.DeleteTemplateSetting(ctx, domain.NoticeKind(req.Kind)); err != nil {
		if errors.Is(err, application.ErrNotFound) {
			p := h.problem(ctx, "/tenant-templates/"+string(req.Kind), err, uuid.Nil)
			return oapi.DeleteTenantTemplate404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return oapi.DeleteTenantTemplate204Response{}, nil
}

// GetNotice returns one composed communication of a dispute.
func (h *Handler) GetNotice(ctx context.Context, req oapi.GetNoticeRequestObject) (oapi.GetNoticeResponseObject, error) {
	rec, doc, err := h.svc.GetNotice(ctx, req.DisputeId, req.NoticeId)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			p := h.problem(ctx, fmt.Sprintf("/disputes/%s/notices/%d", req.DisputeId, req.NoticeId), err, uuid.Nil)
			return oapi.GetNotice404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	out := oapi.NoticeDocument{Id: rec.ID, Kind: oapi.NoticeKind(rec.Kind), Channel: oapi.Channel(rec.Channel), Recipient: rec.Recipient,
		Bank: doc.Bank, Date: rec.CreatedAt, Subject: doc.Subject, Greeting: doc.Greeting, Paragraphs: doc.Paragraphs, Closing: doc.Closing, SentAt: rec.SentAt}
	if doc.Basis != "" {
		basis := doc.Basis
		out.Basis = &basis
	}
	return oapi.GetNotice200JSONResponse(out), nil
}

// ListDisputes pages the tenant's disputes. The cursor is base64 of "<RFC3339Nano opened_at>|<id>"; opaque to clients.
func (h *Handler) ListDisputes(ctx context.Context, req oapi.ListDisputesRequestObject) (oapi.ListDisputesResponseObject, error) {
	q := application.ListQuery{Limit: 25}
	if req.Params.Limit != nil {
		q.Limit = *req.Params.Limit
	}
	if req.Params.State != nil {
		st := domain.State(*req.Params.State)
		q.State = &st
	}
	if req.Params.Overdue != nil {
		q.Overdue = *req.Params.Overdue
	}
	if req.Params.Cursor != nil && *req.Params.Cursor != "" {
		c, err := decodeCursor(*req.Params.Cursor)
		if err != nil {
			return nil, errs.New(errs.Invalid, "contract-violation", "cursor is not one this API issued").
				WithFields(errs.FieldError{Field: "query.cursor", Message: "invalid cursor"})
		}
		q.After = &c
	}
	page, err := h.svc.ListDisputes(ctx, q)
	if err != nil {
		return nil, err
	}
	out := oapi.DisputePage{Items: make([]oapi.DisputeSummary, 0, len(page.Items))}
	for _, d := range page.Items {
		item := oapi.DisputeSummary{Id: d.ID, Regime: oapi.Regime(d.Regime), Reason: oapi.DisputeReason(d.Reason), State: oapi.DisputeState(d.State),
			TransactionId: d.TransactionID, DisputedAmount: money.Format(d.DisputedAmount, d.Currency), Currency: d.Currency, OpenedAt: d.OpenedAt, UpdatedAt: d.UpdatedAt}
		if d.NextDeadline != nil {
			nd := toDeadline(*d.NextDeadline)
			item.NextDeadline = &nd
		}
		if d.Risk != nil {
			tier, score := oapi.RiskTier(d.Risk.Tier), d.Risk.Score
			item.RiskTier, item.RiskScore = &tier, &score
		}
		out.Items = append(out.Items, item)
	}
	if page.Next != nil {
		c := encodeCursor(*page.Next)
		out.NextCursor = &c
	}
	return oapi.ListDisputes200JSONResponse(out), nil
}

func encodeCursor(c application.Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.OpenedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()))
}

func decodeCursor(raw string) (application.Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return application.Cursor{}, err
	}
	at, id, ok := strings.Cut(string(b), "|")
	if !ok {
		return application.Cursor{}, errors.New("malformed cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return application.Cursor{}, err
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return application.Cursor{}, err
	}
	return application.Cursor{OpenedAt: t, ID: u}, nil
}

// ApplyDisputeEvent runs the state machine.
func (h *Handler) ApplyDisputeEvent(ctx context.Context, req oapi.ApplyDisputeEventRequestObject) (oapi.ApplyDisputeEventResponseObject, error) {
	body, _ := json.Marshal(req.Body)
	var payload json.RawMessage
	if req.Body.Payload != nil {
		payload, _ = json.Marshal(req.Body.Payload)
	}
	id := req.DisputeId
	res, err := h.svc.ApplyEvent(ctx, application.ApplyEventInput{
		DisputeID:   id,
		Event:       domain.Event(req.Body.Event),
		Actor:       orDefault(req.Body.Actor, "system"),
		Payload:     payload,
		Idempotency: idempotency(req.Params.IdempotencyKey, body),
	})
	if err != nil {
		p := h.problem(ctx, "/disputes/"+id.String()+"/events", err, id)
		switch p.Status {
		case http.StatusNotFound:
			return oapi.ApplyDisputeEvent404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: oapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusConflict:
			return oapi.ApplyDisputeEvent409ApplicationProblemPlusJSONResponse(p), nil
		case http.StatusUnprocessableEntity:
			return oapi.ApplyDisputeEvent422ApplicationProblemPlusJSONResponse{UnprocessableApplicationProblemPlusJSONResponse: oapi.UnprocessableApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusBadRequest:
			return oapi.ApplyDisputeEvent400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: oapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	return oapi.ApplyDisputeEvent200JSONResponse(toAPI(res.View)), nil
}

// problem builds the contract's Problem; on invalid-transition it adds what the dispute would accept.
func (h *Handler) problem(ctx context.Context, instance string, err error, disputeID uuid.UUID) oapi.Problem {
	p := httpserver.ProblemFrom(ctx, instance, err)
	out := oapi.Problem{
		Type: p.Type, Title: p.Title, Status: p.Status, Instance: &p.Instance,
		Code: oapi.ErrorCode(p.Code), Retryable: p.Retryable, RequestId: p.RequestID,
	}
	if p.Detail != "" {
		out.Detail = &p.Detail
	}
	if p.RetryAfterSeconds != nil {
		out.RetryAfterSeconds = p.RetryAfterSeconds
	}
	if len(p.Errors) > 0 {
		fe := make([]oapi.FieldError, 0, len(p.Errors))
		for _, f := range p.Errors {
			fe = append(fe, oapi.FieldError{Field: f.Field, Message: f.Message})
		}
		out.Errors = &fe
	}
	if errors.Is(err, domain.ErrInvalidTransition) && disputeID != uuid.Nil {
		if view, verr := h.svc.GetDispute(ctx, disputeID); verr == nil {
			allowed := make([]oapi.DisputeEvent, 0, len(view.AllowedEvents))
			for _, e := range view.AllowedEvents {
				allowed = append(allowed, oapi.DisputeEvent(e))
			}
			out.AllowedEvents = &allowed
		}
	}
	return out
}

// fail answers a non-handler failure (parse, validation, unclassified panic-free error) as a problem.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	httpserver.WriteProblem(w, r, httpserver.ProblemFrom(r.Context(), r.URL.Path, err), err)
}

// detailed keeps the classified sentinel but surfaces the binder's message, which is written for API users.
func detailed(base *errs.Error, cause error) error {
	return errs.Wrap(&errs.Error{Kind: base.Kind, Code: base.Code, Msg: cause.Error()}, "%s", base.Msg)
}

// contractViolation turns the validator's structured error into field errors; the summary stays generic.
func contractViolation(err error) error {
	var fields []errs.FieldError
	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) {
		var schemaErr *openapi3.SchemaError
		if errors.As(reqErr.Err, &schemaErr) {
			for e := schemaErr; e != nil; {
				ptr := "/" + strings.Join(e.JSONPointer(), "/")
				if reqErr.Parameter != nil {
					ptr = reqErr.Parameter.In + "." + reqErr.Parameter.Name
				}
				fields = append(fields, errs.FieldError{Field: ptr, Message: e.Reason})
				var inner *openapi3.SchemaError
				if e.Origin == nil || !errors.As(e.Origin, &inner) || inner == e {
					break
				}
				e = inner
			}
		} else if reqErr.Parameter != nil {
			fields = append(fields, errs.FieldError{Field: reqErr.Parameter.In + "." + reqErr.Parameter.Name, Message: reqErr.Reason})
		}
	}
	if len(fields) == 0 {
		return detailed(errContract, err)
	}
	return errs.Wrap(errContract.WithFields(fields...), "%v", err)
}

func idempotency(key *string, body []byte) application.Idempotency {
	if key == nil || *key == "" {
		return application.Idempotency{}
	}
	return application.Idempotency{Key: *key, RequestBody: body}
}

func orDefault(s *string, d string) string {
	if s == nil || *s == "" {
		return d
	}
	return *s
}

func toAPI(v application.DisputeView) oapi.Dispute {
	events := make([]oapi.LoggedEvent, 0, len(v.Events))
	for _, e := range v.Events {
		var payload map[string]any
		_ = json.Unmarshal(e.Payload, &payload)
		if payload == nil {
			payload = map[string]any{}
		}
		events = append(events, oapi.LoggedEvent{
			Seq: e.Seq, Event: string(e.Event), FromState: string(e.FromState), ToState: oapi.DisputeState(e.ToState),
			Actor: e.Actor, Payload: payload, TraceId: e.TraceID, OccurredAt: e.OccurredAt,
		})
	}
	allowed := make([]oapi.DisputeEvent, 0, len(v.AllowedEvents))
	for _, e := range v.AllowedEvents {
		allowed = append(allowed, oapi.DisputeEvent(e))
	}
	deadlines := make([]oapi.Deadline, 0, len(v.Deadlines))
	for _, d := range v.Deadlines {
		deadlines = append(deadlines, toDeadline(d))
	}
	ledger := make([]oapi.LedgerEntry, 0, len(v.Ledger))
	for _, l := range v.Ledger {
		entry := oapi.LedgerEntry{Seq: l.Seq, Kind: oapi.PostingKind(l.Kind), Debit: oapi.LedgerAccount(l.Debit),
			Credit: oapi.LedgerAccount(l.Credit), Amount: money.Format(l.Amount, l.Currency), Currency: l.Currency, Reference: l.Reference, PostedAt: l.PostedAt}
		if l.Core != nil {
			entry.Core = &oapi.CoreReceipt{Rrn: l.Core.RRN, ResponseCode: l.Core.ResponseCode, LatencyMs: l.Core.LatencyMs}
		}
		ledger = append(ledger, entry)
	}
	out := oapi.Dispute{
		Id: v.ID, Regime: oapi.Regime(v.Regime), Reason: oapi.DisputeReason(v.Reason), State: oapi.DisputeState(v.State), Appeals: v.Appeals,
		Version: v.Version, TransactionId: v.TransactionID, AccountId: v.AccountID,
		DisputedAmount: money.Format(v.DisputedAmount, v.Currency), Currency: v.Currency, OpenedAt: v.OpenedAt, UpdatedAt: v.UpdatedAt,
		AllowedEvents: allowed, Events: events, Deadlines: deadlines, Ledger: ledger,
		Balances: oapi.Balances{Customer: money.Format(v.Balances.Customer, v.Currency), Suspense: money.Format(v.Balances.Suspense, v.Currency),
			Recovery: money.Format(v.Balances.Recovery, v.Currency), Loss: money.Format(v.Balances.Loss, v.Currency)},
	}
	out.Notices = make([]oapi.Notice, 0, len(v.Notices))
	for _, n := range v.Notices {
		nv := oapi.Notice{Id: n.ID, Seq: n.Seq, Kind: oapi.NoticeKind(n.Kind), Channel: oapi.Channel(n.Channel),
			Recipient: n.Recipient, Subject: n.Subject, CreatedAt: n.CreatedAt, SentAt: n.SentAt, Error: n.Error}
		if n.Actor != "" {
			actor := n.Actor
			nv.Actor = &actor
		}
		nv.ResendOf = n.ResendOf
		nv.Attachments = make([]oapi.Attachment, 0, len(n.Attachments))
		for _, a := range n.Attachments {
			nv.Attachments = append(nv.Attachments, oapi.Attachment{Id: a.ID, Filename: a.Filename, ContentType: a.ContentType, Size: a.Size})
		}
		out.Notices = append(out.Notices, nv)
	}
	if r := v.Risk; r != nil {
		signals := make([]oapi.RiskSignal, 0, len(r.Signals))
		for _, sg := range r.Signals {
			signals = append(signals, oapi.RiskSignal{Name: sg.Name, Weight: sg.Weight, Points: sg.Points, Detail: sg.Detail})
		}
		history := make([]struct {
			AssessedAt time.Time     `json:"assessedAt"`
			Score      int           `json:"score"`
			Seq        int           `json:"seq"`
			Tier       oapi.RiskTier `json:"tier"`
		}, 0, len(r.History))
		for _, h := range r.History {
			history = append(history, struct {
				AssessedAt time.Time     `json:"assessedAt"`
				Score      int           `json:"score"`
				Seq        int           `json:"seq"`
				Tier       oapi.RiskTier `json:"tier"`
			}{AssessedAt: h.AssessedAt, Score: h.Score, Seq: h.Seq, Tier: oapi.RiskTier(h.Tier)})
		}
		out.Risk = &oapi.Risk{Score: r.Score, Tier: oapi.RiskTier(r.Tier), Signals: signals, AssessedAt: r.AssessedAt, History: history}
	}
	if q := v.Questionnaire; q != nil {
		questions := make([]oapi.Question, 0, len(q.Questions))
		for _, x := range q.Questions {
			questions = append(questions, oapi.Question{Id: x.ID, Text: x.Text, Type: oapi.AnswerType(x.Type), Required: x.Required})
		}
		qv := oapi.Questionnaire{Reason: oapi.DisputeReason(q.Reason), Questions: questions, Inconsistencies: q.Inconsistencies, SentAt: q.SentAt, ReceivedAt: q.ReceivedAt}
		if q.Answers != nil {
			answers := q.Answers
			qv.Answers = &answers
		}
		out.Questionnaire = &qv
	}
	return out
}

func toDeadline(d application.DeadlineView) oapi.Deadline {
	return oapi.Deadline{Kind: oapi.DeadlineKind(d.Kind), Cycle: d.Cycle, StartedAt: d.StartedAt, DueAt: d.DueAt, MetAt: d.MetAt,
		Status: oapi.DeadlineStatus(d.Status), Basis: d.Basis}
}
