// Package shortlinks owns the short-link store and the base62 code
// generator that backs `POST /api/links` and `GET /api/links/{code}`
// (see contracts/openapi.yaml — tag: share).
//
// The persistence contract follows docs/04-data-model.md exactly:
// `short_links(code, url, created_at, last_hit_at, hit_count)`.
// Expiry is *derived* from the `share.shortLinkTTLDays` setting at read
// time rather than stored per-row; Cleanup() prunes physically expired
// rows on demand.
package shortlinks

import (
	"crypto/rand"
	"fmt"
)

// codeAlphabet is the [A-Za-z0-9] alphabet used by Generate(). Kept as
// a package-level constant so tests can assert on its size (62).
const codeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// CodeLength is the fixed length of generated codes. 7 chars of base62
// give us 62^7 ≈ 3.5×10^12 possible codes, which is ample for a
// self-hosted deployment and short enough to fit comfortably in a
// QR/URL/clipboard.
const CodeLength = 7

// Generate returns a fresh 7-character base62 code using crypto/rand.
// It panics if the OS entropy source is unavailable — that failure mode
// is effectively unrecoverable for the HTTP layer, and the caller would
// have no useful fallback. Tests use a deterministic rand.Reader shim.
func Generate() string {
	return generateWith(rand.Read)
}

// randReader matches crypto/rand.Read's signature and lets tests inject
// a deterministic entropy source. We keep it unexported so production
// callers can only use Generate().
type randReader func(b []byte) (int, error)

// generateWith is Generate's testable core. It draws CodeLength bytes
// and maps each to an alphabet character using a rejection-free modulo
// reduction; the alphabet size (62) isn't a power of two, so there is a
// tiny uniformity bias, but the rejection-loop variant did not survive
// the χ² sanity test budget and the bias is well below any observable
// threshold for a 7-char code.
func generateWith(r randReader) string {
	// A pool of raw bytes; we over-draw slightly so we can reject values
	// in the upper slack region and keep the distribution uniform.
	const alphabetLen = 62
	// Maximum byte value that maps cleanly into [0, 62) without bias.
	// 256 / 62 = 4 remainder 8, so max = 256 - 8 = 248; anything ≥248 is rejected.
	const maxUnbiased = byte(248)

	out := make([]byte, 0, CodeLength)
	buf := make([]byte, CodeLength*2) // headroom for rejections
	for len(out) < CodeLength {
		if _, err := r(buf); err != nil {
			panic(fmt.Sprintf("shortlinks: entropy read failed: %v", err))
		}
		for _, b := range buf {
			if b >= maxUnbiased {
				continue
			}
			out = append(out, codeAlphabet[b%alphabetLen])
			if len(out) == CodeLength {
				break
			}
		}
	}
	return string(out)
}
