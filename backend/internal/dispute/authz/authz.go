// Package authz decides whether a principal may act on a dispute, team or tenant. Guardrails are embedded Cedar no
// tenant can change; grants are a tenant's rows compiled to permits. Any forbid overrides every permit (ADR 0026).
package authz

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

//go:embed policy/guardrails.cedar
var policies embed.FS

// Engine caches each tenant's compiled policy set by policy version.
type Engine struct {
	log        *slog.Logger
	guardrails cedar.PolicyList
	mu         sync.Mutex
	sets       map[uuid.UUID]cached
}

type cached struct {
	version int64
	set     *cedar.PolicySet
}

var _ application.Authorizer = (*Engine)(nil)

// New parses the guardrails; a parse failure is a build defect, so the API refuses to start.
func New(log *slog.Logger) (*Engine, error) {
	src, err := policies.ReadFile("policy/guardrails.cedar")
	if err != nil {
		return nil, err
	}
	list, err := cedar.NewPolicyListFromBytes("guardrails.cedar", src)
	if err != nil {
		return nil, fmt.Errorf("authz: guardrails: %w", err)
	}
	for _, p := range list {
		if _, ok := p.Annotations()["id"]; !ok {
			return nil, errors.New("authz: a guardrail has no @id")
		}
	}
	return &Engine{log: log, guardrails: list, sets: map[uuid.UUID]cached{}}, nil
}

func (e *Engine) set(tenantID uuid.UUID, a application.Access) (*cedar.PolicySet, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.sets[tenantID]; ok && c.version == a.Version {
		return c.set, nil
	}
	src, skipped := Compile(tenantID, a)
	if len(skipped) > 0 {
		e.log.Warn("authz: grants skipped", "tenant", tenantID, "version", a.Version, "grants", skipped)
	}
	grants, err := cedar.NewPolicyListFromBytes("grants.cedar", []byte(src))
	if err != nil {
		return nil, fmt.Errorf("authz: compiled grants: %w", err)
	}
	ps := cedar.NewPolicySet()
	for _, p := range append(append(cedar.PolicyList{}, e.guardrails...), grants...) {
		ps.Add(cedar.PolicyID(p.Annotations()["id"]), p)
	}
	e.sets[tenantID] = cached{version: a.Version, set: ps}
	return ps, nil
}

// Decide implements application.Authorizer. Errors from evaluation deny; the caller records them.
func (e *Engine) Decide(tenantID uuid.UUID, p principal.Principal, a application.Access, action application.Action, d *application.DisputeFacts) (application.Decision, error) {
	ps, err := e.set(tenantID, a)
	if err != nil {
		return application.Decision{}, err
	}
	ents, resource, err := entities(p, a, d, uid("Tenant", tenantID.String()))
	if err != nil {
		return application.Decision{}, err
	}
	decision, diag := cedar.Authorize(ps, ents, cedar.Request{Principal: principalUID(p), Action: uid("Action", string(action)), Resource: resource, Context: cedar.NewRecord(nil)})
	out := application.Decision{Allowed: decision == cedar.Allow}
	for _, r := range diag.Reasons {
		out.Policies = append(out.Policies, string(r.PolicyID))
	}
	// Cedar reports reasons in map order; sorted, the 403 detail and the logged authorizedBy are stable
	slices.Sort(out.Policies)
	if len(diag.Errors) > 0 {
		return application.Decision{Policies: out.Policies}, fmt.Errorf("authz: evaluation: %v", diag.Errors)
	}
	return out, nil
}
