package application_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application/apptest"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/domain"
)

// Idempotency keys are stored per caller, but the replay metric must stay labelled by operation: a label per
// principal would put user identifiers in the metrics and grow a series per analyst.
func TestReplayMetricIsLabelledByOperationOnly(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	svc, store := newService(t)
	txn := store.AddTransaction(domain.RailCard, "EUR", "EUR", "40.00")
	idem := application.Idempotency{Key: "k", RequestBody: []byte(`{}`)}
	for range 2 {
		if _, err := svc.CreateDispute(apptest.Ctx(), application.CreateDisputeInput{TransactionID: txn, Idempotency: idem}); err != nil {
			t.Fatal(err)
		}
	}
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "dispute.idempotent_replays" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				if v, _ := dp.Attributes.Value("scope"); v.AsString() != "dispute.create" {
					t.Fatalf("replay metric scope = %q", v.AsString())
				}
			}
			return
		}
	}
	t.Fatal("no replay was recorded")
}
