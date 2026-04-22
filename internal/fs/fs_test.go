package fs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
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

func TestFindFileReturnsErrorWhenNoMatchExists(t *testing.T) {
	t.Helper()

	_, _, err := FindFile(t.TempDir(), `\.mmdb$`)
	if err == nil {
		t.Fatal("expected error when no matching file exists")
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

func TestInstallFileAtomicallyReplacesDestinationOnSuccess(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.mmdb")
	destPath := filepath.Join(dir, "GeoLite2-City.mmdb")

	if err := os.WriteFile(sourcePath, []byte("new-valid-db"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(destPath, []byte("old-db"), 0600); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	validated := false
	err := InstallFileAtomically(sourcePath, destPath, func(path string) error {
		validated = true
		if path == destPath {
			t.Fatal("validator should run against the temporary file, not the final destination")
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(content) != "new-valid-db" {
			t.Fatalf("unexpected temp file contents: %q", string(content))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("InstallFileAtomically: %v", err)
	}
	if !validated {
		t.Fatal("expected validator to run")
	}

	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(content) != "new-valid-db" {
		t.Fatalf("expected new contents, got %q", string(content))
	}
}

func TestInstallFileAtomicallyLeavesDestinationUntouchedOnValidationError(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.mmdb")
	destPath := filepath.Join(dir, "GeoLite2-City.mmdb")

	if err := os.WriteFile(sourcePath, []byte("candidate-db"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(destPath, []byte("known-good-db"), 0600); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	expectedErr := errors.New("validator rejected candidate")
	err := InstallFileAtomically(sourcePath, destPath, func(path string) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected validation error, got %v", err)
	}

	content, readErr := os.ReadFile(destPath)
	if readErr != nil {
		t.Fatalf("read dest: %v", readErr)
	}
	if string(content) != "known-good-db" {
		t.Fatalf("destination was modified: %q", string(content))
	}

	matches, globErr := filepath.Glob(filepath.Join(dir, ".GeoLite2-City.mmdb.tmp-*"))
	if globErr != nil {
		t.Fatalf("glob temp files: %v", globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("expected temp file cleanup, found %v", matches)
	}
}
