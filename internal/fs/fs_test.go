package fs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyMD5HashFromFileRejectsMismatch(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "GeoLite2-City.mmdb")
	checksumPath := filepath.Join(dir, "GeoLite2-City.mmdb.md5")

	if err := os.WriteFile(filePath, []byte("actual contents"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(checksumPath, []byte("deadbeef  GeoLite2-City.mmdb\n"), 0600); err != nil {
		t.Fatalf("write checksum: %v", err)
	}

	if err := VerifyMD5HashFromFile(filePath, checksumPath); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestExtractTarGzRejectsPathTraversal(t *testing.T) {
	t.Helper()

	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)

	payload := []byte("malicious")
	header := &tar.Header{
		Name: "../escape.txt",
		Mode: 0600,
		Size: int64(len(payload)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	if err := ExtractTarGz(bytes.NewReader(archive.Bytes()), t.TempDir()); err == nil {
		t.Fatal("expected path traversal error")
	}
}
