// Package pmtiles wraps github.com/protomaps/go-pmtiles for the
// minimum subset the tile-router needs: open one local file, parse
// its header (bbox + zoom range), keep its root + leaf directories
// in memory, and serve individual tiles via pread.
//
// We don't reuse the upstream `pmtiles serve` HTTP wrapper because
// it expects a multi-source bucket layout (every file is identified
// by a name in a URL like /<name>/{z}/{x}/{y}). The tile-router has
// already picked which file via the bbox heuristic by the time we
// get here, so serving is just "open file, find offset, ReadAt".
//
// Concurrency notes:
//   - The underlying *os.File is safe for concurrent ReadAt per the
//     io.ReaderAt contract — the kernel handles the pread serialization.
//   - The root + leaf directory caches are populated once at Open()
//     and are immutable afterward, so no lock is needed.
package pmtiles

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	pm "github.com/protomaps/go-pmtiles/pmtiles"
)

// Reader serves tiles from one .pmtiles file on local disk.
type Reader struct {
	Path     string
	Header   pm.HeaderV3
	file     *os.File
	rootDir  []pm.EntryV3
	leafsMu  sync.Mutex // guards leafCache; root is immutable post-Open
	leafs    map[uint64][]pm.EntryV3 // offset → entries; small N, lazy
}

// Open reads + parses the file's header and root directory. The file
// stays open for the lifetime of the Reader and must be Close()d. The
// caller is expected to hold one Reader per loaded region.
func Open(path string) (*Reader, error) {
	f, err := os.Open(path) //nolint:gosec // path is from regions DB, not user input
	if err != nil {
		return nil, fmt.Errorf("open pmtiles %s: %w", path, err)
	}
	hdrBuf := make([]byte, 127)
	if _, err := f.ReadAt(hdrBuf, 0); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("read header: %w", err)
	}
	hdr, err := pm.DeserializeHeader(hdrBuf)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("parse header: %w", err)
	}
	rootBuf := make([]byte, hdr.RootLength)
	if _, err := f.ReadAt(rootBuf, int64(hdr.RootOffset)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("read root dir: %w", err)
	}
	rootEntries, err := decodeDirectory(rootBuf, hdr.InternalCompression)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("decode root dir: %w", err)
	}
	return &Reader{
		Path:    path,
		Header:  hdr,
		file:    f,
		rootDir: rootEntries,
		leafs:   make(map[uint64][]pm.EntryV3),
	}, nil
}

// Close releases the underlying file descriptor.
func (r *Reader) Close() error {
	return r.file.Close()
}

// BBox returns the file's WGS84 bounding box [minLon, minLat, maxLon,
// maxLat]. The pmtiles header stores these as int32 scaled by 1e7
// (E7); we decode to degrees here so callers don't have to know the
// representation.
func (r *Reader) BBox() (minLon, minLat, maxLon, maxLat float64) {
	return float64(r.Header.MinLonE7) / 1e7,
		float64(r.Header.MinLatE7) / 1e7,
		float64(r.Header.MaxLonE7) / 1e7,
		float64(r.Header.MaxLatE7) / 1e7
}

// ZoomRange returns the file's min and max zoom inclusive.
func (r *Reader) ZoomRange() (uint8, uint8) {
	return r.Header.MinZoom, r.Header.MaxZoom
}

// TileType returns the file's tile format (vector mvt, raster png/jpeg/webp/avif).
// We forward it as the Content-Type header without inspecting the bytes.
func (r *Reader) TileType() pm.TileType {
	return r.Header.TileType
}

// TileCompression returns the per-tile compression (typically gzip for
// vector mvt). We forward as Content-Encoding so the browser decompresses
// inline rather than us paying the CPU to decompress + recompress.
func (r *Reader) TileCompression() pm.Compression {
	return r.Header.TileCompression
}

// ReadTile returns the raw (still-compressed) bytes of a single tile,
// or (nil, nil) when no tile exists at that coordinate. Errors are
// reserved for I/O or corruption.
//
// The lookup walks the root directory; on a leaf-pointer hit it loads
// the leaf (small, ~kb each) and caches it for the lifetime of the
// Reader. Country-scale pmtiles typically have a small root + a few
// dozen leaves so memory stays well under the 10 MB-per-file rule of
// thumb.
func (r *Reader) ReadTile(z uint8, x, y uint32) ([]byte, error) {
	if z < r.Header.MinZoom || z > r.Header.MaxZoom {
		return nil, nil // out-of-range zoom is a normal 404
	}
	tileID := pm.ZxyToID(z, x, y)
	entries := r.rootDir
	for {
		e, ok := pm.FindTile(entries, tileID)
		if !ok {
			return nil, nil
		}
		if e.RunLength == 0 {
			// Pointer to a leaf directory rather than a tile. Resolve
			// against the leaf area and recurse into that smaller dir.
			leaf, err := r.leafEntries(uint64(e.Offset), uint64(e.Length))
			if err != nil {
				return nil, err
			}
			entries = leaf
			continue
		}
		// Real tile entry: read from TileDataOffset + e.Offset.
		buf := make([]byte, e.Length)
		if _, err := r.file.ReadAt(buf, int64(r.Header.TileDataOffset)+int64(e.Offset)); err != nil {
			return nil, fmt.Errorf("read tile %d/%d/%d: %w", z, x, y, err)
		}
		return buf, nil
	}
}

// leafEntries reads + decodes a leaf directory at the given offset/length
// (relative to LeafDirectoryOffset). Cached by offset so repeated tile
// reads in the same leaf don't re-decode.
func (r *Reader) leafEntries(offset, length uint64) ([]pm.EntryV3, error) {
	r.leafsMu.Lock()
	if cached, ok := r.leafs[offset]; ok {
		r.leafsMu.Unlock()
		return cached, nil
	}
	r.leafsMu.Unlock()

	buf := make([]byte, length)
	if _, err := r.file.ReadAt(buf, int64(r.Header.LeafDirectoryOffset)+int64(offset)); err != nil {
		return nil, fmt.Errorf("read leaf: %w", err)
	}
	entries, err := decodeDirectory(buf, r.Header.InternalCompression)
	if err != nil {
		return nil, fmt.Errorf("decode leaf: %w", err)
	}
	r.leafsMu.Lock()
	r.leafs[offset] = entries
	r.leafsMu.Unlock()
	return entries, nil
}

// decodeDirectory wraps the raw bytes for go-pmtiles' DeserializeEntries
// which handles compression internally via its compression argument.
func decodeDirectory(buf []byte, compression pm.Compression) ([]pm.EntryV3, error) {
	entries := pm.DeserializeEntries(bytes.NewBuffer(buf), compression)
	return entries, nil
}
