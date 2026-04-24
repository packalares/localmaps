// policy.go — schedule -> next_update_at computation.
//
// Preset schedules map to fixed daily/weekly/monthly slots at 03:00 UTC
// (matching regions.updateCheckCron default in docs/07-config-schema.md).
// Arbitrary 5-field cron strings are parsed via robfig/cron/v3.

package scheduler

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Preset schedule tokens accepted by ComputeNextUpdate. They MUST stay
// in sync with server/internal/regions/model.go and contracts/openapi.yaml
// RegionSchedule enum.
const (
	ScheduleNever   = "never"
	ScheduleDaily   = "daily"
	ScheduleWeekly  = "weekly"
	ScheduleMonthly = "monthly"
)

// ErrEmptySchedule is returned when the caller hands in an empty
// schedule string. Callers treat this as a programming error.
var ErrEmptySchedule = errors.New("scheduler: empty schedule string")

// ErrInvalidSchedule wraps parse failures for custom cron expressions
// and unknown preset tokens.
var ErrInvalidSchedule = errors.New("scheduler: invalid schedule")

// cronParser accepts the standard 5-field "min hour dom mon dow" form.
// No descriptors (e.g. "@every 5m"), no seconds field — see R17; when in
// doubt be conservative.
var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// ComputeNextUpdate returns the next scheduled firing strictly after
// now, in UTC. The returned time is always in the UTC location so
// callers can format it with time.RFC3339Nano for the regions row.
//
// Behaviour by input:
//
//   - "never"                      -> (time.Time{}, nil)   caller skips.
//   - "daily"                      -> next 03:00 UTC after now.
//   - "weekly"                     -> next Sunday 03:00 UTC after now.
//   - "monthly"                    -> first of next month 03:00 UTC
//     (or this month if now is strictly before that instant and day==1,
//     but monthly preset always lands on the *next* month per R17 to
//     keep semantics unambiguous).
//   - 5-field cron ("m h dom mon dow") -> next firing after now.
//   - "" / unknown preset          -> (zero, ErrInvalidSchedule).
//
// Because we always return a time strictly after now, calling tick->
// persist->recompute yields monotonically increasing next_update_at.
func ComputeNextUpdate(now time.Time, schedule string) (time.Time, error) {
	s := strings.TrimSpace(schedule)
	if s == "" {
		return time.Time{}, ErrEmptySchedule
	}
	nowUTC := now.UTC()
	switch s {
	case ScheduleNever:
		return time.Time{}, nil
	case ScheduleDaily:
		return nextDaily(nowUTC), nil
	case ScheduleWeekly:
		return nextWeekly(nowUTC), nil
	case ScheduleMonthly:
		return nextMonthly(nowUTC), nil
	}
	// Custom cron expression.
	if strings.Count(s, " ") != 4 {
		return time.Time{}, fmt.Errorf("%w: %q", ErrInvalidSchedule, schedule)
	}
	sched, err := cronParser.Parse(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %q: %v", ErrInvalidSchedule, schedule, err)
	}
	return sched.Next(nowUTC).UTC(), nil
}

// nextDaily returns the next 03:00 UTC occurrence strictly after now.
func nextDaily(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
	if today.After(now) {
		return today
	}
	return today.Add(24 * time.Hour)
}

// nextWeekly returns the next Sunday 03:00 UTC strictly after now.
// If "today is Sunday before 03:00" we land at 03:00 today; otherwise
// we skip to the Sunday of the following week.
func nextWeekly(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
	daysUntilSunday := (int(time.Sunday) - int(today.Weekday()) + 7) % 7
	candidate := today.AddDate(0, 0, daysUntilSunday)
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate
}

// nextMonthly returns the first of next month at 03:00 UTC. By always
// advancing a month we sidestep the edge case where now is before
// 03:00 on the 1st (which would otherwise allow two firings in the
// same month if invoked before and after the 03:00 instant).
func nextMonthly(now time.Time) time.Time {
	firstNext := time.Date(now.Year(), now.Month()+1, 1, 3, 0, 0, 0, time.UTC)
	return firstNext
}

// ValidateSchedule reports whether the given schedule string would be
// accepted by ComputeNextUpdate. Callers (e.g. the scheduler tick) use
// it to skip malformed rows without constructing a zero time.Time that
// looks like "never".
func ValidateSchedule(s string) error {
	_, err := ComputeNextUpdate(time.Now().UTC(), s)
	return err
}
