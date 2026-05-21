package workers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemporalWorkflowForSource(t *testing.T) {
	cases := []struct {
		source    string
		workflow  string
		country   string
		firstMode string
		nextMode  string
		bulkFirst bool
	}{
		{"companies_house", "PullCompaniesHouse", "GB", "", "", false},
		{"brreg", "PullBrreg", "NO", "bulk", "incremental", true},
		{"gleif", "PullGLEIF", "", "bulk", "delta", true},
		{"cvr", "PullCVR", "DK", "bulk", "incremental", true},
		{"ariregister", "PullAriregister", "EE", "bulk", "refresh", true},
	}

	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			cfg, ok := TemporalWorkflowForSource(tc.source)
			require.True(t, ok)
			require.Equal(t, tc.workflow, cfg.WorkflowType)
			require.Equal(t, tc.country, cfg.Country)
			require.Equal(t, tc.firstMode, cfg.FirstMode)
			require.Equal(t, tc.nextMode, cfg.NextMode)
			require.Equal(t, tc.bulkFirst, cfg.BulkFirst)
		})
	}
}
