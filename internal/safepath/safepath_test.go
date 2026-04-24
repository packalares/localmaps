package safepath_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/internal/safepath"
)

func TestJoin_Legitimate(t *testing.T) {
	got, err := safepath.Join("/data", "regions", "europe-romania", "map.pmtiles")
	require.NoError(t, err)
	require.Equal(t, filepath.Clean("/data/regions/europe-romania/map.pmtiles"), got)
}

func TestJoin_RejectsTraversal(t *testing.T) {
	_, err := safepath.Join("/data", "..", "etc", "passwd")
	require.ErrorIs(t, err, safepath.ErrEscapesRoot)

	_, err = safepath.Join("/data", "regions", "..", "..", "etc")
	require.ErrorIs(t, err, safepath.ErrEscapesRoot)
}

func TestJoin_RejectsAbsoluteComponent(t *testing.T) {
	_, err := safepath.Join("/data", "/etc/passwd")
	require.ErrorIs(t, err, safepath.ErrAbsolutePart)
}

func TestJoin_RejectsNullByte(t *testing.T) {
	_, err := safepath.Join("/data", "regions", "bad\x00name")
	require.ErrorIs(t, err, safepath.ErrNullByte)
}

func TestJoin_RequiresAbsoluteRoot(t *testing.T) {
	_, err := safepath.Join("", "x")
	require.ErrorIs(t, err, safepath.ErrEmptyRoot)
	_, err = safepath.Join("relative/root", "x")
	require.ErrorIs(t, err, safepath.ErrEmptyRoot)
}

func TestJoin_RejectsEmptyPart(t *testing.T) {
	_, err := safepath.Join("/data", "")
	require.ErrorIs(t, err, safepath.ErrEmptyPart)
}

func TestJoin_InnerDotDotStayingWithinRootIsFine(t *testing.T) {
	// "regions/europe-romania/../europe-bulgaria" resolves to
	// "/data/regions/europe-bulgaria" — still inside /data.
	got, err := safepath.Join("/data", "regions", "europe-romania", "..", "europe-bulgaria")
	require.NoError(t, err)
	require.Equal(t, filepath.Clean("/data/regions/europe-bulgaria"), got)
}
