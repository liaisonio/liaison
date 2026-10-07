package webide

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func testArchive(t *testing.T, headers ...tar.Header) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, h := range headers {
		require.NoError(t, tw.WriteHeader(&h))
		if h.Size > 0 {
			_, err := tw.Write(bytes.Repeat([]byte("x"), int(h.Size)))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return b.Bytes()
}
func TestPackageRejectsUnsafeEntries(t *testing.T) {
	for _, tc := range []tar.Header{
		{Name: "../escaped", Typeflag: tar.TypeReg},
		{Name: "/pkg/file", Typeflag: tar.TypeReg},
		{Name: "pkg/link", Typeflag: tar.TypeSymlink, Linkname: "../../escaped"},
		{Name: "pkg/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		{Name: "pkg/device", Typeflag: tar.TypeChar},
		{Name: "pkg/hard", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"},
	} {
		t.Run(tc.Name+tc.Linkname, func(t *testing.T) {
			require.Error(t, extractArchive(t.Context(), t.TempDir(), "pkg", bytes.NewReader(testArchive(t, tc))))
		})
	}
}
func TestPackageRejectsDuplicateAndLinkChain(t *testing.T) {
	require.Error(t, extractArchive(t.Context(), t.TempDir(), "pkg", bytes.NewReader(testArchive(t, tar.Header{Name: "pkg/a", Typeflag: tar.TypeReg}, tar.Header{Name: "pkg/a", Typeflag: tar.TypeReg}))))
	require.Error(t, extractArchive(t.Context(), t.TempDir(), "pkg", bytes.NewReader(testArchive(t, tar.Header{Name: "pkg/a", Typeflag: tar.TypeSymlink, Linkname: "b"}, tar.Header{Name: "pkg/b", Typeflag: tar.TypeSymlink, Linkname: "a"}))))
}
func TestInstallChecksDigestAndNeverOverwrites(t *testing.T) {
	archive := testArchive(t, tar.Header{Name: "pkg/bin/code-server", Mode: 0755, Typeflag: tar.TypeReg, Size: 4})
	digest := sha256.Sum256(archive)
	root := t.TempDir()
	_, err := installArchive(t.Context(), root, "pkg", string(bytes.Repeat([]byte("0"), 64)), bytes.NewReader(archive))
	require.ErrorContains(t, err, "checksum")
	_, err = os.Stat(filepath.Join(root, "pkg"))
	require.True(t, os.IsNotExist(err))
	program, err := installArchive(t.Context(), root, "pkg", hex.EncodeToString(digest[:]), bytes.NewReader(archive))
	require.NoError(t, err)
	data, err := os.ReadFile(program)
	require.NoError(t, err)
	require.Equal(t, "xxxx", string(data))
	_, err = installArchive(t.Context(), root, "pkg", hex.EncodeToString(digest[:]), bytes.NewReader(archive))
	require.ErrorContains(t, err, "already exists")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
func TestExtractionAllowsInternalRelativeSymlink(t *testing.T) {
	dir := t.TempDir()
	archive := testArchive(t, tar.Header{Name: "pkg/lib/main", Mode: 0755, Typeflag: tar.TypeReg, Size: 3}, tar.Header{Name: "pkg/bin/code-server", Typeflag: tar.TypeSymlink, Linkname: "../lib/main"})
	require.NoError(t, extractArchive(t.Context(), dir, "pkg", bytes.NewReader(archive)))
	data, err := os.ReadFile(filepath.Join(dir, "bin/code-server"))
	require.NoError(t, err)
	require.Equal(t, "xxx", string(data))
}
func TestInstallCancelledBeforePublish(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	root := t.TempDir()
	_, err := installArchive(ctx, root, "pkg", string(bytes.Repeat([]byte("0"), 64)), bytes.NewReader(nil))
	require.ErrorIs(t, err, context.Canceled)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries)
}
func TestPinnedDigestsAreSHA256(t *testing.T) {
	for _, digest := range releaseDigests {
		b, err := hex.DecodeString(digest)
		require.NoError(t, err)
		require.Len(t, b, 32)
	}
}
