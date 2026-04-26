package glyphs

import (
	"testing"
)

// TestLookupKnownFonts ensures the three embedded fonts resolve for the
// first Unicode range. Each PBF in that range carries basic-Latin SDF
// glyphs and should be at least a few kilobytes.
func TestLookupKnownFonts(t *testing.T) {
	for _, f := range []string{"Noto Sans Regular", "Noto Sans Bold", "Noto Sans Italic"} {
		data, ok := Lookup(f, "0-255")
		if !ok {
			t.Fatalf("Lookup(%q, 0-255) not found", f)
		}
		if len(data) < 1000 {
			t.Fatalf("Lookup(%q, 0-255) too small: %d bytes", f, len(data))
		}
		// First byte is the protobuf field-tag for glyphs.proto `stacks`
		// (field 1, wire type 2) → 0x0a. Anything else would mean the
		// embed picked up a corrupt/renamed file.
		if data[0] != 0x0a {
			t.Fatalf("Lookup(%q, 0-255) first byte = %#x, want 0x0a", f, data[0])
		}
	}
}

// TestLookupFallback confirms the comma-separated fontstack picks the
// first available name. MapLibre sends stacks like
// "Noto Sans Regular,Arial Unicode MS Regular" — the handler must
// resolve to Noto Sans even if the Arial entry comes first.
func TestLookupFallback(t *testing.T) {
	got, ok := Lookup("Arial Unicode MS Regular,Noto Sans Regular", "0-255")
	if !ok || len(got) == 0 {
		t.Fatalf("fallback did not pick up Noto Sans Regular")
	}
	want, _ := Lookup("Noto Sans Regular", "0-255")
	if len(got) != len(want) {
		t.Fatalf("fallback returned different bytes: got %d want %d", len(got), len(want))
	}
}

// TestLookupRejectsBadInput covers: missing font, malformed range,
// and empty inputs. All should return (nil, false) — no panics, no
// partial reads.
func TestLookupRejectsBadInput(t *testing.T) {
	cases := []struct{ stack, rng string }{
		{"Missing Font", "0-255"},
		{"Noto Sans Regular", "bogus"},
		{"Noto Sans Regular", ""},
		{"", "0-255"},
		{"Noto Sans Regular", "../../etc/passwd"},
	}
	for _, c := range cases {
		if data, ok := Lookup(c.stack, c.rng); ok || data != nil {
			t.Errorf("Lookup(%q, %q) = (%d bytes, %v), want (nil, false)", c.stack, c.rng, len(data), ok)
		}
	}
}

// TestFonts verifies the introspection helper lists all three
// embedded directories in sorted order.
func TestFonts(t *testing.T) {
	got := Fonts()
	want := []string{"Noto Sans Bold", "Noto Sans Italic", "Noto Sans Regular"}
	if len(got) != len(want) {
		t.Fatalf("Fonts() returned %d entries, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Fonts()[%d] = %q, want %q", i, got[i], w)
		}
	}
}
