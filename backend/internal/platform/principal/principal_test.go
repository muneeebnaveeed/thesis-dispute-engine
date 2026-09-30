package principal_test

import (
	"context"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/principal"
)

func TestRoundTrip(t *testing.T) {
	if _, ok := principal.From(context.Background()); ok {
		t.Fatal("empty context carried a principal")
	}
	p := principal.Principal{Kind: principal.Analyst, ID: "sub-1", Display: "a@x", TenantAdmin: true}
	got, ok := principal.From(principal.With(context.Background(), p))
	if !ok || got != p {
		t.Fatalf("got %+v %v", got, ok)
	}
}
