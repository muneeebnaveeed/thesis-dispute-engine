package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// problems counts error responses by code so validation and conflict rates are visible by kind, not just status.
var problems = sync.OnceValue(func() metric.Int64Counter {
	c, err := otel.Meter("github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver").
		Int64Counter("http.server.problems", metric.WithDescription("Problem responses by error code"))
	if err != nil {
		panic(err)
	}
	return c
})

// Problem is the RFC 9457 body every error response uses; all of it is user-safe.
type Problem struct {
	Type              string            `json:"type"`
	Title             string            `json:"title"`
	Status            int               `json:"status"`
	Detail            string            `json:"detail,omitempty"`
	Instance          string            `json:"instance,omitempty"`
	Code              string            `json:"code"`
	Retryable         bool              `json:"retryable"`
	RetryAfterSeconds *int              `json:"retryAfterSeconds,omitempty"`
	RequestID         string            `json:"requestId"`
	Errors            []errs.FieldError `json:"errors,omitempty"`
	Extensions        map[string]any    `json:"-"`
}

var statusByKind = map[errs.Kind]int{
	errs.Invalid:       http.StatusBadRequest,
	errs.NotFound:      http.StatusNotFound,
	errs.Conflict:      http.StatusConflict,
	errs.Unprocessable: http.StatusUnprocessableEntity,
	errs.Unauthorized:  http.StatusUnauthorized,
	errs.Forbidden:     http.StatusForbidden,
	errs.Unavailable:   http.StatusServiceUnavailable,
	errs.RateLimited:   http.StatusTooManyRequests,
	errs.Internal:      http.StatusInternalServerError,
}

var titleByKind = map[errs.Kind]string{
	errs.Invalid:       "The request is not valid",
	errs.NotFound:      "Not found",
	errs.Conflict:      "The request conflicts with the current state",
	errs.Unprocessable: "The request cannot be processed",
	errs.Unauthorized:  "Authentication required",
	errs.Forbidden:     "Not allowed",
	errs.Unavailable:   "Temporarily unavailable",
	errs.RateLimited:   "Too many requests",
	errs.Internal:      "Something went wrong on our side",
}

// WithRetryAfter returns a copy carrying the Retry-After hint.
func (p Problem) WithRetryAfter(seconds int) Problem {
	p.RetryAfterSeconds = &seconds
	return p
}

// ProblemFrom classifies err; unclassified errors become a generic 500 whose detail carries only the request ID.
func ProblemFrom(ctx context.Context, instance string, err error) Problem {
	kind := errs.KindOf(err)
	p := Problem{
		Type:      "urn:dispute-engine:error:" + string(errs.CodeOf(err)),
		Title:     titleByKind[kind],
		Status:    statusByKind[kind],
		Detail:    errs.UserMessage(err),
		Instance:  instance,
		Code:      string(errs.CodeOf(err)),
		Retryable: errs.Retryable(err),
		RequestID: RequestIDFrom(ctx),
		Errors:    errs.FieldsOf(err),
	}
	problems().Add(ctx, 1, metric.WithAttributes(attribute.String("code", p.Code), attribute.Int("status", p.Status)))
	switch kind {
	case errs.Internal:
		p.Detail = "Something went wrong on our side. Reference: " + p.RequestID
	case errs.Unavailable:
		after := 5
		p.RetryAfterSeconds = &after
		if p.Detail == "" {
			p.Detail = "A dependency is unavailable. Reference: " + p.RequestID
		}
	}
	return p
}

// WriteProblem encodes p; 5xx are logged with the original error, 4xx are not (the request log already has the status).
func WriteProblem(w http.ResponseWriter, r *http.Request, p Problem, cause error) {
	if p.Status >= 500 && cause != nil {
		slog.Default().ErrorContext(r.Context(), "request failed",
			"request_id", p.RequestID, "code", p.Code, "err", cause)
	}
	if p.RetryAfterSeconds != nil {
		w.Header().Set("Retry-After", itoa(*p.RetryAfterSeconds))
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p.marshal())
}

// marshal merges Extensions into the body without a custom MarshalJSON on the struct.
func (p Problem) marshal() map[string]any {
	raw, _ := json.Marshal(struct {
		Problem
		Extensions map[string]any `json:"-"`
	}{Problem: p})
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	for k, v := range p.Extensions {
		out[k] = v
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
