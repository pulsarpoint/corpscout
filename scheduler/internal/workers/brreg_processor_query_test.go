package workers_test

import (
	"os"
	"strings"
	"testing"
)

func TestClaimPendingTranslatedRawInputsRequireTranslatedPayload(t *testing.T) {
	queryBytes, err := os.ReadFile("../../../database/queries/raw_inputs.sql")
	if err != nil {
		t.Fatalf("read raw input queries: %v", err)
	}
	query := string(queryBytes)

	for _, queryName := range []string{
		"ClaimPendingBrregRawInputs",
		"ClaimPendingCVRRawInputs",
		"ClaimPendingAriregisterRawInputs",
	} {
		t.Run(queryName, func(t *testing.T) {
			claim := rawInputQueryByName(t, query, queryName)

			expectedGate := `WHERE (
        processing_status = 'pending'
        OR (processing_status = 'processing' AND processing_lease_until < now())
    )
    AND raw_payload_en IS NOT NULL`
			if !strings.Contains(claim, expectedGate) {
				t.Fatalf("%s query must gate processing on translated payload with grouped status predicate:\n%s", queryName, claim)
			}
		})
	}
}

func rawInputQueryByName(t *testing.T, queries, name string) string {
	t.Helper()

	start := strings.Index(queries, "-- name: "+name)
	if start == -1 {
		t.Fatalf("%s query not found", name)
	}
	end := strings.Index(queries[start+1:], "-- name:")
	if end == -1 {
		end = len(queries)
	} else {
		end += start + 1
	}
	return queries[start:end]
}
