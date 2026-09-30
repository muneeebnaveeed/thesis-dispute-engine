package disputehttp

import (
	"strings"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/ports/http/oapi"
)

func TestEveryOperationIsClassified(t *testing.T) {
	spec, err := oapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkCoverage(spec); err != nil {
		t.Fatal(err)
	}
}

func TestUnclassifiedOperationFails(t *testing.T) {
	spec, err := oapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	spec.Paths.Find("/healthz").Get.OperationID = "brandNew"
	if err := checkCoverage(spec); err == nil || !strings.Contains(err.Error(), "brandNew") {
		t.Fatalf("err = %v", err)
	}
}
