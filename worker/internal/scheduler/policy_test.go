package scheduler

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return ts.UTC()
}

func TestComputeNextUpdate_Never(t *testing.T) {
	got, err := ComputeNextUpdate(mustUTC(t, "2026-01-15T12:00:00Z"), "never")
	require.NoError(t, err)
	require.True(t, got.IsZero(), "never should return zero time")
}

func TestComputeNextUpdate_EmptyIsError(t *testing.T) {
	_, err := ComputeNextUpdate(time.Now(), "")
	require.ErrorIs(t, err, ErrEmptySchedule)
}

func TestComputeNextUpdate_InvalidIsError(t *testing.T) {
	_, err := ComputeNextUpdate(time.Now(), "garbage")
	require.ErrorIs(t, err, ErrInvalidSchedule)

	// Wrong field count (4 fields only).
	_, err = ComputeNextUpdate(time.Now(), "0 3 * *")
	require.ErrorIs(t, err, ErrInvalidSchedule)

	// 5 fields but invalid hour.
	_, err = ComputeNextUpdate(time.Now(), "0 99 * * *")
	require.ErrorIs(t, err, ErrInvalidSchedule)
}

func TestComputeNextUpdate_DailyBeforeThreeAM(t *testing.T) {
	now := mustUTC(t, "2026-04-24T02:30:00Z")
	got, err := ComputeNextUpdate(now, ScheduleDaily)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-04-24T03:00:00Z"), got)
}

func TestComputeNextUpdate_DailyAfterThreeAM(t *testing.T) {
	now := mustUTC(t, "2026-04-24T03:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleDaily)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-04-25T03:00:00Z"), got)
}

func TestComputeNextUpdate_DailyAcrossMonthBoundary(t *testing.T) {
	now := mustUTC(t, "2026-01-31T23:59:00Z")
	got, err := ComputeNextUpdate(now, ScheduleDaily)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-02-01T03:00:00Z"), got)
}

func TestComputeNextUpdate_WeeklyFromMidweek(t *testing.T) {
	// Friday 2026-04-24. Next Sunday = 2026-04-26.
	now := mustUTC(t, "2026-04-24T12:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleWeekly)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-04-26T03:00:00Z"), got)
	require.Equal(t, time.Sunday, got.Weekday())
}

func TestComputeNextUpdate_WeeklyOnSundayBeforeThreeAM(t *testing.T) {
	// Sunday 2026-04-26 02:00 → lands same day at 03:00.
	now := mustUTC(t, "2026-04-26T02:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleWeekly)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-04-26T03:00:00Z"), got)
}

func TestComputeNextUpdate_WeeklyOnSundayAfterThreeAM(t *testing.T) {
	// Sunday 2026-04-26 04:00 → skip to next Sunday.
	now := mustUTC(t, "2026-04-26T04:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleWeekly)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-05-03T03:00:00Z"), got)
}

func TestComputeNextUpdate_MonthlyMidMonth(t *testing.T) {
	now := mustUTC(t, "2026-04-15T10:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleMonthly)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-05-01T03:00:00Z"), got)
}

func TestComputeNextUpdate_MonthlyYearRollover(t *testing.T) {
	now := mustUTC(t, "2026-12-20T00:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleMonthly)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2027-01-01T03:00:00Z"), got)
}

func TestComputeNextUpdate_MonthlyOnFirstBeforeThreeAM(t *testing.T) {
	// On the 1st before 03:00 — monthly preset still advances to next
	// month's first to keep cadence unambiguous.
	now := mustUTC(t, "2026-05-01T01:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleMonthly)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-06-01T03:00:00Z"), got)
}

func TestComputeNextUpdate_MonthlyFeb29(t *testing.T) {
	// Leap year: Feb 29 2028 exists. Starting at Jan 31 → Feb 1, then
	// Feb 1 → Mar 1.
	now := mustUTC(t, "2028-01-31T00:00:00Z")
	got, err := ComputeNextUpdate(now, ScheduleMonthly)
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2028-02-01T03:00:00Z"), got)
}

func TestComputeNextUpdate_CronEveryHour(t *testing.T) {
	now := mustUTC(t, "2026-04-24T12:30:00Z")
	got, err := ComputeNextUpdate(now, "0 * * * *")
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-04-24T13:00:00Z"), got)
}

func TestComputeNextUpdate_CronNightly(t *testing.T) {
	now := mustUTC(t, "2026-04-24T12:00:00Z")
	got, err := ComputeNextUpdate(now, "0 3 * * *")
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-04-25T03:00:00Z"), got)
}

func TestComputeNextUpdate_CronMonthlyFirstAtMidnight(t *testing.T) {
	now := mustUTC(t, "2026-04-24T12:00:00Z")
	got, err := ComputeNextUpdate(now, "0 0 1 * *")
	require.NoError(t, err)
	require.Equal(t, mustUTC(t, "2026-05-01T00:00:00Z"), got)
}

func TestComputeNextUpdate_CronAcrossDST(t *testing.T) {
	// UTC has no DST; a cron parsed in UTC always advances by 24h for
	// a daily job. Confirm the two clock noon-to-noons are exactly 24h
	// apart at the DST boundary (Europe was 2026-03-29).
	before := mustUTC(t, "2026-03-28T12:00:00Z")
	after := mustUTC(t, "2026-03-29T12:00:00Z")
	nextBefore, err := ComputeNextUpdate(before, "0 12 * * *")
	require.NoError(t, err)
	nextAfter, err := ComputeNextUpdate(after, "0 12 * * *")
	require.NoError(t, err)
	require.Equal(t, 24*time.Hour, nextBefore.Sub(before))
	require.Equal(t, 24*time.Hour, nextAfter.Sub(after))
}

func TestComputeNextUpdate_ReturnsUTC(t *testing.T) {
	local, _ := time.LoadLocation("America/Los_Angeles")
	if local == nil {
		t.Skip("tz db not available")
	}
	now := time.Date(2026, 4, 24, 2, 30, 0, 0, local)
	got, err := ComputeNextUpdate(now, ScheduleDaily)
	require.NoError(t, err)
	require.Equal(t, time.UTC, got.Location())
}

func TestValidateSchedule(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{ScheduleNever, false},
		{ScheduleDaily, false},
		{ScheduleWeekly, false},
		{ScheduleMonthly, false},
		{"0 3 * * *", false},
		{"", true},
		{"garbage", true},
	}
	for _, c := range cases {
		err := ValidateSchedule(c.in)
		if c.wantErr {
			require.Error(t, err, "expected error for %q", c.in)
		} else {
			require.NoError(t, err, "unexpected error for %q", c.in)
		}
	}
	// ErrEmpty is a specific sentinel.
	require.True(t, errors.Is(ValidateSchedule(""), ErrEmptySchedule))
}
