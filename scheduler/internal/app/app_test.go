package app

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	db "github.com/pulsarpoint/corpscout/scheduler/internal/db/gen"
)

func TestSourceScheduleDue_disabledSourceNotDue(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	expr := "1h"

	due, err := sourceScheduleDue(db.DataSource{
		Enabled:            false,
		ScheduleEnabled:    true,
		ScheduleKind:       "interval",
		ScheduleExpression: &expr,
	}, now)

	require.NoError(t, err)
	require.False(t, due)
}

func TestSourceScheduleDue_scheduleDisabledNotDue(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	expr := "1h"

	due, err := sourceScheduleDue(db.DataSource{
		Enabled:            true,
		ScheduleEnabled:    false,
		ScheduleKind:       "interval",
		ScheduleExpression: &expr,
	}, now)

	require.NoError(t, err)
	require.False(t, due)
}

func TestSourceScheduleDue_intervalDueWhenLastStartedOlderThanInterval(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	expr := "1h"

	due, err := sourceScheduleDue(db.DataSource{
		Enabled:            true,
		ScheduleEnabled:    true,
		ScheduleKind:       "interval",
		ScheduleExpression: &expr,
		LastStartedAt: pgtype.Timestamptz{
			Time:  now.Add(-2 * time.Hour),
			Valid: true,
		},
	}, now)

	require.NoError(t, err)
	require.True(t, due)
}

func TestSourceScheduleDue_invalidDurationReturnsErrorAndFalse(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	expr := "daily"

	due, err := sourceScheduleDue(db.DataSource{
		Enabled:            true,
		ScheduleEnabled:    true,
		ScheduleKind:       "interval",
		ScheduleExpression: &expr,
	}, now)

	require.Error(t, err)
	require.False(t, due)
}

func TestSourceScheduleDue_nonPositiveDurationReturnsErrorAndFalse(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)

	for _, expr := range []string{"0s", "-1h"} {
		t.Run(expr, func(t *testing.T) {
			due, err := sourceScheduleDue(db.DataSource{
				Enabled:            true,
				ScheduleEnabled:    true,
				ScheduleKind:       "interval",
				ScheduleExpression: &expr,
			}, now)

			require.Error(t, err)
			require.False(t, due)
		})
	}
}
