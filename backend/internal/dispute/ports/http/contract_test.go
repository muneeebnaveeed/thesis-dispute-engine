package disputehttp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
)

// The spec enum and the codes the Go side can emit are maintained by hand in two places; this keeps them equal.
func TestErrorCodesMatchContract(t *testing.T) {
	a := newAPI(t, func(context.Context) error { return nil })
	rec, _ := a.do(http.MethodGet, "/openapi.json", nil, nil)
	var spec struct {
		Components struct {
			Schemas struct {
				ErrorCode struct {
					Enum []string `json:"enum"`
				}
			}
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	inSpec := spec.Components.Schemas.ErrorCode.Enum
	slices.Sort(inSpec)

	// Importing this package pulls in domain, application and the HTTP port, so every sentinel has been constructed.
	registered := errs.Codes()
	inGo := make([]string, 0, len(registered))
	for c := range registered {
		inGo = append(inGo, string(c))
	}
	slices.Sort(inGo)

	if !slices.Equal(inSpec, inGo) {
		t.Fatalf("ErrorCode enum and emitted codes differ\nspec: %v\ngo:   %v", inSpec, inGo)
	}
	for code, kinds := range registered {
		if len(slices.Compact(slices.Sorted(slices.Values(kinds)))) != 1 {
			t.Errorf("code %q constructed with more than one kind: %v", code, kinds)
		}
	}
}

// Retryable is a promise in the Problem schema's description; pin which codes may make it.
func TestRetryableCodes(t *testing.T) {
	want := []string{"concurrent-update", "unavailable"}
	var got []string
	for code, kinds := range errs.Codes() {
		p := httpserver.ProblemFrom(context.Background(), "/x", errs.New(kinds[0], code, "test"))
		if p.Status == 0 || p.Title == "" {
			t.Errorf("code %q (kind %v) has no status or title", code, kinds[0])
		}
		if p.Retryable {
			got = append(got, string(code))
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("retryable codes = %v, want %v", got, want)
	}
}
