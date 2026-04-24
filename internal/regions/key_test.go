package regions

import (
	"errors"
	"testing"
)

func TestNormaliseKey_RoundTrips(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Continent-only.
		{"europe", "europe"},
		{"africa", "africa"},
		// Canonical passthrough.
		{"europe-romania", "europe-romania"},
		// Slash form.
		{"europe/romania", "europe-romania"},
		// Nested subregion preserves internal hyphens.
		{"europe/germany/baden-wuerttemberg", "europe-germany-baden-wuerttemberg"},
		// Australia-oceania slug (contains hyphen natively).
		{"australia-oceania", "australia-oceania"},
		// Subregion of a hyphen-native continent.
		{"north-america/us/new-york", "north-america-us-new-york"},
		// Leading/trailing whitespace.
		{"  europe/romania  ", "europe-romania"},
		// Digit-containing region (Geofabrik has "us/puerto-rico" but
		// also numeric IDs in rare cases).
		{"europe/russia-federal-district-1", "europe-russia-federal-district-1"},
	}
	for _, tc := range cases {
		got, err := NormaliseKey(tc.in)
		if err != nil {
			t.Fatalf("NormaliseKey(%q): unexpected error %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("NormaliseKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormaliseKey_Rejects(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", ErrEmptyKey},
		{"whitespace-only", "   ", ErrEmptyKey},
		{"absolute", "/europe/romania", ErrAbsoluteKey},
		{"parent-traversal", "europe/../etc", ErrPathTraversal},
		{"bare-traversal", "..", ErrPathTraversal},
		{"double-slash", "europe//romania", ErrInvalidChars},
		{"trailing-slash", "europe/romania/", ErrInvalidChars},
		{"uppercase", "Europe/Romania", ErrInvalidChars},
		{"spaces", "europe/ro mania", ErrInvalidChars},
		{"underscore", "europe/ro_mania", ErrInvalidChars},
		{"backslash", `europe\romania`, ErrInvalidChars},
		{"null-byte", "europe\x00romania", ErrInvalidChars},
		{"unicode", "europe/românia", ErrInvalidChars},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormaliseKey(tc.in)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestIsCanonical(t *testing.T) {
	yes := []string{"europe", "europe-romania", "north-america-us-new-york",
		"europe-germany-baden-wuerttemberg"}
	for _, s := range yes {
		if !IsCanonical(s) {
			t.Errorf("IsCanonical(%q) = false, want true", s)
		}
	}
	no := []string{"", "Europe", "europe/romania", "europe_romania",
		"europe romania", "europe*", "europe-germany!"}
	for _, s := range no {
		if IsCanonical(s) {
			t.Errorf("IsCanonical(%q) = true, want false", s)
		}
	}
}

func TestGeofabrikPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Bare continent.
		{"europe", "europe"},
		{"africa", "africa"},
		// Hyphen-native continents.
		{"australia-oceania", "australia-oceania"},
		{"north-america", "north-america"},
		{"south-america", "south-america"},
		{"central-america", "central-america"},
		// Continent / country.
		{"europe-romania", "europe/romania"},
		{"europe-germany", "europe/germany"},
		// Hyphen-native country beneath a plain continent.
		{"europe-czech-republic", "europe/czech-republic"},
		// Country beneath a hyphen-native continent.
		{"north-america-us", "north-america/us"},
		// Subregion gets best-effort continent/rest.
		{"europe-germany-baden-wuerttemberg", "europe/germany-baden-wuerttemberg"},
	}
	for _, tc := range cases {
		got, err := GeofabrikPath(tc.in)
		if err != nil {
			t.Fatalf("GeofabrikPath(%q): unexpected error %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("GeofabrikPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGeofabrikPath_Rejects(t *testing.T) {
	bad := []string{"", "   ", "Europe-Romania", "europe/romania",
		"unknown-continent-foo"}
	for _, s := range bad {
		if _, err := GeofabrikPath(s); err == nil {
			t.Errorf("GeofabrikPath(%q) expected error, got nil", s)
		}
	}
}

func TestNormaliseThenGeofabrikPath_RoundTripContinentCountry(t *testing.T) {
	// The main round-trip guarantee: continent and continent/country
	// inputs survive Normalise -> GeofabrikPath unchanged.
	inputs := []string{"europe", "europe/romania", "europe/germany",
		"north-america", "north-america/us", "europe/czech-republic"}
	for _, in := range inputs {
		canon, err := NormaliseKey(in)
		if err != nil {
			t.Fatalf("Normalise(%q): %v", in, err)
		}
		back, err := GeofabrikPath(canon)
		if err != nil {
			t.Fatalf("GeofabrikPath(%q): %v", canon, err)
		}
		if back != in {
			t.Errorf("round-trip %q -> %q -> %q", in, canon, back)
		}
	}
}
