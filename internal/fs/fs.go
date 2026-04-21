package fs

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"rest-geoip/internal/hash"
	"strings"
	"sync"
)

// FindFile returns a path to a file matching regex under root
// Returns
//
//	string: Full path
//	string: File name
//	error : Error
func FindFile(root, r string) (string, string, error) {
	regex, err := regexp.Compile(r)
	if err != nil {
		return "", "", err
	}

	var foundPath string
	var foundName string

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if regex.MatchString(info.Name()) {
			foundPath = path
			foundName = info.Name()
		}
		return nil
	})

	if err != nil {
		return "", "", err
	}
	return foundPath, foundName, nil
}

// VerifyMD5HashFromFile hashes a file and verifies it against a sum
// contained within a .md5 file
func VerifyMD5HashFromFile(file, md5sumFile string) error {
	actual, err := hash.MD5Hash(file)
	if err != nil {
		return err
	}

	cleanMD5SumFile := filepath.Clean(md5sumFile)

	// We know exactly where this file and path is
	// #nosec G304
	expected, err := os.ReadFile(cleanMD5SumFile)
	if err != nil {
		return err
	}

	fields := strings.Fields(string(expected))
	if len(fields) == 0 {
		return fmt.Errorf("md5 file %q did not contain a checksum", md5sumFile)
	}

	actualChecksum := fmt.Sprintf("%x", actual)
	if actualChecksum != fields[0] {
		return fmt.Errorf("md5 checksum mismatch: expected %s, got %s", fields[0], actualChecksum)
	}

	return nil
}

// ExtractTarGz extracts a gzipped stream to dest
func ExtractTarGz(r io.Reader, dest string) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("Stream requires gzip-compressed body: %v", err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	cleanDest := filepath.Clean(dest)
	if err := os.MkdirAll(cleanDest, 0750); err != nil {
		return fmt.Errorf("ExtractTarGz: MkdirAll() failed: %v", err)
	}

	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Tar error: %v", err)
		}

		targetPath, err := safeArchivePath(cleanDest, f.Name)
		if err != nil {
			return err
		}

		switch f.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return fmt.Errorf("ExtractTarGz: MkdirAll() failed: %v", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
				return fmt.Errorf("ExtractTarGz: MkdirAll() failed: %v", err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("ExtractTarGz: Create() failed: %v", err)
			}
			// For our purposes, we don't expect any files larger than 100MiB
			limited := &io.LimitedReader{R: tr, N: 100 << 20}
			if _, err := io.Copy(outFile, limited); err != nil {
				return fmt.Errorf("ExtractTarGz: Copy() failed: %v", err)
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf(
				"ExtractTarGz: %s has unknown type: %v",
				f.Name,
				f.Typeflag)
		}
	}

	return nil
}

// MoveFile moves a file
func MoveFile(source, dest string) error {
	// #nosec G304
	input, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	err = os.WriteFile(dest, input, 0600)
	if err != nil {
		return err
	}

	return nil
}

// Download a file
func Download(url, dest string, wg *sync.WaitGroup, errChannel chan<- error) {
	defer wg.Done()

	// We know exactly how this url is constructed
	// #nosec G107
	resp, err := http.Get(url)
	if err != nil {
		errChannel <- err
		return
	}
	if resp.StatusCode%200 > 99 {
		errChannel <- fmt.Errorf("Download error: %s", resp.Status)
		return
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(dest)
	if err != nil {
		errChannel <- err
		return
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		errChannel <- err
		return
	}

	if err = out.Close(); err != nil {
		errChannel <- err
	}
}

func safeArchivePath(root, name string) (string, error) {
	targetPath := filepath.Join(root, filepath.Clean(name))
	relPath, err := filepath.Rel(root, targetPath)
	if err != nil {
		return "", fmt.Errorf("ExtractTarGz: Rel() failed: %v", err)
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("ExtractTarGz: illegal archive path %q", name)
	}
	return targetPath, nil
}
