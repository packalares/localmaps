package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestDiscardProgress ensures the no-op reporter is safe to call and
// never errors — it's the default when the caller passes nil.
func TestDiscardProgress(t *testing.T) {
	var p ProgressReporter = DiscardProgress{}
	require.NoError(t, p.Report(context.Background(), "any", 0.5, "msg"))
}

type capturedEvent struct {
	eventType string
	channel   string
	data      any
}

type fakeWs struct{ events []capturedEvent }

func (f *fakeWs) Publish(t, ch string, d any) {
	f.events = append(f.events, capturedEvent{t, ch, d})
}

// TestAsynqProgressThrottle asserts the MinInterval throttle holds
// between the first and second reports, then flushes on the third.
func TestAsynqProgressThrottle(t *testing.T) {
	fake := &fakeWs{}
	var vnow time.Time
	p := NewAsynqProgress("job-1", nil, nil, fake, zerolog.Nop())
	p.MinInterval = 100 * time.Millisecond
	p.Now = func() time.Time { return vnow }

	ctx := context.Background()

	vnow = time.Unix(1000, 0)
	require.NoError(t, p.Report(ctx, "s1", 0.1, "one"))
	// Second call inside throttle window — coalesced, no publish.
	vnow = vnow.Add(10 * time.Millisecond)
	require.NoError(t, p.Report(ctx, "s1", 0.2, "two"))
	require.Len(t, fake.events, 1, "first report should flush once")

	// Past the window — flush resumes.
	vnow = vnow.Add(200 * time.Millisecond)
	require.NoError(t, p.Report(ctx, "s1", 0.3, "three"))
	require.Len(t, fake.events, 2)

	// Terminal 1.0 always flushes regardless of throttle.
	vnow = vnow.Add(1 * time.Millisecond)
	require.NoError(t, p.Report(ctx, "s1", 1.0, "done"))
	require.Len(t, fake.events, 3)
}

// TestAsynqProgressMonotonic ensures a backwards step is dropped.
func TestAsynqProgressMonotonic(t *testing.T) {
	fake := &fakeWs{}
	p := NewAsynqProgress("j", nil, nil, fake, zerolog.Nop())
	p.MinInterval = 1 // effectively no throttle
	ctx := context.Background()
	require.NoError(t, p.Report(ctx, "s", 0.5, "half"))
	require.NoError(t, p.Report(ctx, "s", 0.4, "backwards")) // dropped
	require.Len(t, fake.events, 1)
	// Forward progress resumes normally.
	require.NoError(t, p.Report(ctx, "s", 0.6, "forward"))
	require.Len(t, fake.events, 2)
}

// TestAsynqProgressTerminalStateSucceeded verifies the payload state
// flips from "running" to "succeeded" at fraction 1.0.
func TestAsynqProgressTerminalStateSucceeded(t *testing.T) {
	fake := &fakeWs{}
	p := NewAsynqProgress("j", nil, nil, fake, zerolog.Nop())
	p.MinInterval = 1
	ctx := context.Background()
	require.NoError(t, p.Report(ctx, "s", 0.5, "mid"))
	require.NoError(t, p.Report(ctx, "s", 1.0, "end"))
	mid := fake.events[0].data.(progressPayload)
	end := fake.events[1].data.(progressPayload)
	require.Equal(t, "running", mid.State)
	require.Equal(t, "succeeded", end.State)
}

// TestProgressParser covers the stage-weight math.
func TestProgressParser(t *testing.T) {
	p := newProgressParser()
	cases := []struct {
		in           string
		wantStage    string
		wantOverall  float64
		wantMatched  bool
	}{
		{"12:00 INFO  [osm_pass1] -   0% [0s] starting", "planetiler.osm_pass1", 0.0, true},
		{"12:01 INFO  [osm_pass1] -  50%", "planetiler.osm_pass1", 0.10, true},
		{"12:02 INFO  [osm_pass1] - 100%", "planetiler.osm_pass1", 0.20, true},
		{"12:03 INFO  [osm_pass2] -  50%", "planetiler.osm_pass2", 0.20 + 0.15, true},
		{"12:04 not a progress line", "", 0, false},
	}
	for _, c := range cases {
		stage, overall, ok := p.Parse(c.in)
		require.Equal(t, c.wantMatched, ok, c.in)
		if !c.wantMatched {
			continue
		}
		require.Equal(t, c.wantStage, stage, c.in)
		require.InDelta(t, c.wantOverall, overall, 1e-6, c.in)
	}
}
