package tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestIDFrom(t *testing.T) {
	if _, ok := IDFrom(context.Background()); ok {
		t.Fatal("empty context should have no tenant")
	}
	if _, ok := IDFrom(WithID(context.Background(), uuid.Nil)); ok {
		t.Fatal("nil uuid is not a tenant")
	}
	want := uuid.New()
	if got, ok := IDFrom(WithID(context.Background(), want)); !ok || got != want {
		t.Fatalf("got %v %v", got, ok)
	}
}

func TestStatic(t *testing.T) {
	want := uuid.New()
	var got uuid.UUID
	h := Static(want)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got, _ = IDFrom(r.Context()) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if got != want {
		t.Fatalf("got %v", got)
	}
}
