package workers_test

import (
	"os"
	"strings"
	"testing"
)

func TestClaimPendingBrregRawInputsRequiresTranslatedPayload(t *testing.T) {
	queryBytes, err := os.ReadFile("../../../database/queries/raw_inputs.sql")
	if err != nil {
		t.Fatalf("read raw input queries: %v", err)
	}
	query := string(queryBytes)

	start := strings.Index(query, "-- name: ClaimPendingBrregRawInputs")
	if start == -1 {
		t.Fatal("ClaimPendingBrregRawInputs query not found")
	}
	end := strings.Index(query[start+1:], "-- name:")
	if end == -1 {
		end = len(query)
	} else {
		end += start + 1
	}
	brregClaim := query[start:end]

	expectedGate := `WHERE (
        processing_status = 'pending'
        OR (processing_status = 'processing' AND processing_lease_until < now())
    )
    AND raw_payload_en IS NOT NULL`
	if !strings.Contains(brregClaim, expectedGate) {
		t.Fatalf("Brreg claim query must gate processing on translated payload with grouped status predicate:\n%s", brregClaim)
	}
}
