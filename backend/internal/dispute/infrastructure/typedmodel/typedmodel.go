// Package typedmodel adapts a hosted typed-decision model to the application's Decisions port. The model
// answers constrained questions in one pass and returns a probability with each answer, so nothing it says
// has to be parsed out of prose; what it proposes is still only ever a proposal (ADR 0024).
package typedmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
)

// DefaultEndpoint is the hosted decisions API; DefaultModel pins a released version rather than a moving
// alias, so a proposal recorded beside an analyst's answer names the exact model that made it.
const (
	DefaultEndpoint = "https://openrouter.ai/api/alpha/decisions"
	DefaultModel    = "typesafe/jev-1.13"
)

// Client calls the hosted model. The zero value is not usable; see New.
type Client struct {
	endpoint string
	model    string
	key      string
	http     *http.Client
}

// New returns a client, or nil when no key is configured. A nil Decisions is the supported case: every
// feature that asks for a proposal degrades to an empty field.
func New(endpoint, model, key string, timeout time.Duration) *Client {
	if key == "" {
		return nil
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if model == "" {
		model = DefaultModel
	}
	return &Client{endpoint: endpoint, model: model, key: key, http: &http.Client{Timeout: timeout}}
}

// Model is the version this client asks for, recorded with any proposal it makes.
func (c *Client) Model() string { return c.model }

type wireQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

type wireAnswer struct {
	Type       string  `json:"type"`
	Choice     string  `json:"choice"`
	Noul       float64 `json:"noul"`
	Confidence float64 `json:"confidence"`
}

// Decide implements application.Decisions.
func (c *Client) Decide(ctx context.Context, state map[string]string, questions map[string]application.Question) (map[string]application.Answer, error) {
	ctx, span := otel.Tracer("typedmodel").Start(ctx, "decisions.decide", trace.WithAttributes(
		attribute.String("model", c.model), attribute.Int("questions", len(questions)),
	))
	defer span.End()

	asked := make(map[string]wireQuestion, len(questions))
	for id, q := range questions {
		switch q.Kind {
		case application.KindChoice:
			asked[id] = wireQuestion{Type: "choice", Instructions: q.Ask, Criteria: q.Options}
		case application.KindYesNo:
			asked[id] = wireQuestion{Type: "noul", Instructions: q.Ask}
		default:
			return nil, fmt.Errorf("typedmodel: unknown question kind %q", q.Kind)
		}
	}
	body, err := json.Marshal(map[string]any{"model": c.model, "state": state, "questions": asked})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, errs.Wrap(application.ErrDecisionsUnavailable, "decisions: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 200))
		span.SetAttributes(attribute.Int("http.status", response.StatusCode))
		return nil, errs.Wrap(application.ErrDecisionsUnavailable, "decisions: status %d: %s", response.StatusCode, snippet)
	}
	var decoded struct {
		Answers map[string]wireAnswer `json:"answers"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, errs.Wrap(application.ErrDecisionsUnavailable, "decisions: %v", err)
	}

	out := make(map[string]application.Answer, len(decoded.Answers))
	for id, answer := range decoded.Answers {
		switch answer.Type {
		case "choice":
			out[id] = application.Answer{Value: answer.Choice, Probability: answer.Confidence}
		case "noul":
			// a yes/no answer arrives as the probability of yes, so the confidence is its distance from the middle
			if answer.Noul >= 0.5 {
				out[id] = application.Answer{Value: "yes", Probability: answer.Noul}
			} else {
				out[id] = application.Answer{Value: "no", Probability: 1 - answer.Noul}
			}
		}
	}
	span.SetAttributes(attribute.Int("answers", len(out)))
	return out, nil
}
