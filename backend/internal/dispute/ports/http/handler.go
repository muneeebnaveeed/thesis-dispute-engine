// Package disputehttp serves the dispute API; routes come from docs/api/openapi.yaml via the generated strict server.
package disputehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
	disputepg "github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/infrastructure/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/ports/http/oapi"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/auth"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
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
	res, err := h.svc.CreateDispute(ctx, application.CreateDisputeInput{
		TransactionID: req.Body.TransactionId,
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
	return oapi.Dispute{
		Id: v.ID, Regime: oapi.Regime(v.Regime), State: oapi.DisputeState(v.State), Appeals: v.Appeals,
		Version: v.Version, TransactionId: v.TransactionID, AccountId: v.AccountID,
		DisputedAmount: v.DisputedAmount.StringFixed(4), Currency: v.Currency, OpenedAt: v.OpenedAt, UpdatedAt: v.UpdatedAt,
		AllowedEvents: allowed, Events: events,
	}
}
