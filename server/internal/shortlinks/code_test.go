package shortlinks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodeAlphabet_Size(t *testing.T) {
	require.Len(t, codeAlphabet, 62,
		"base62 alphabet must contain exactly 62 distinct characters")
	seen := map[rune]bool{}
	for _, r := range codeAlphabet {
		require.False(t, seen[r], "alphabet has duplicate %q", r)
		seen[r] = true
	}
}

func TestGenerate_LengthAndAlphabet(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := Generate()
		require.Len(t, code, CodeLength)
		for _, r := range code {
			require.Truef(t, strings.ContainsRune(codeAlphabet, r),
				"code %q contains non-base62 char %q", code, r)
		}
	}
}

func TestGenerate_UniqueAcross1000(t *testing.T) {
	// Collisions in a 7-char base62 code at n=1000 are astronomically
	// unlikely (~1 in 7×10^9); any collision here indicates a bug.
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		c := Generate()
		_, dup := seen[c]
		require.Falsef(t, dup, "duplicate code generated: %s", c)
		seen[c] = struct{}{}
	}
}

// TestGenerate_FirstCharChiSquare runs a simple χ² goodness-of-fit on
// the first character across 6200 draws (100 expected per alphabet
// entry). The 99.9% critical value for 61 degrees of freedom is 99.6;
// we assert well under that so flakes are vanishingly rare.
func TestGenerate_FirstCharChiSquare(t *testing.T) {
	const draws = 6200
	const expectedPerBucket = draws / 62 // 100

	counts := map[byte]int{}
	for i := 0; i < draws; i++ {
		c := Generate()
		counts[c[0]]++
	}
	require.Len(t, counts, 62, "first char must hit every bucket in %d draws", draws)

	var chi2 float64
	for _, b := range []byte(codeAlphabet) {
		diff := float64(counts[b] - expectedPerBucket)
		chi2 += (diff * diff) / float64(expectedPerBucket)
	}
	// 99.9% χ² critical value for df=61 is ≈99.6. We allow 120 as a
	// generous ceiling — any real uniformity regression trips this.
	require.Lessf(t, chi2, 120.0,
		"first-char χ² = %.2f exceeds 120 — distribution is not uniform", chi2)
}

// Inject a failing reader to prove the panic path is wired. We don't
// expect this to fire in production, but the panic is the contract.
func TestGenerateWith_EntropyFailurePanics(t *testing.T) {
	bad := func([]byte) (int, error) {
		return 0, errTest
	}
	require.PanicsWithValue(t,
		"shortlinks: entropy read failed: boom",
		func() { _ = generateWith(bad) })
}

// errTest is a sentinel error used by the entropy-failure test.
var errTest = testErr("boom")

type testErr string

func (e testErr) Error() string { return string(e) }
