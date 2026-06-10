package recipes

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

type fileEntry struct {
	name    string
	content []byte
}

func Test_extractTar(t *testing.T) {
	chownActual := chownFunc
	chown = func(string, int, int) error {
		return nil
	}

	defer func() { chown = chownActual }()

	tests := []struct {
		name          string
		entries       []fileEntry
		wantErrRegexp *regexp.Regexp
	}{
		{
			name: "base case scenario",
			entries: []fileEntry{
				{
					name: "test1", content: []byte("test1"),
				},
				{
					name: "test2", content: []byte("test2"),
				},
			},
			wantErrRegexp: nil,
		},
		{
			name: "tar with vulnerable path, fail with expected error",
			entries: []fileEntry{
				{
					name: "../test1", content: []byte("test1"),
				},
				{
					name: "test2", content: []byte("test2"),
				},
			},
			wantErrRegexp: regexp.MustCompile("^unable to extract tar archive /tmp/[0-9]+/extractTar.tar: path /tmp/test1, does not belongs to dir /tmp/[0-9]+, rel ../test1$"),
		},
		{
			name: "tar with advance vulnerable path, fail with expected error",
			entries: []fileEntry{
				{
					name: "....//test1", content: []byte("test1"),
				},
				{
					name: "test2", content: []byte("test2"),
				},
			},
			wantErrRegexp: regexp.MustCompile("^unable to extract tar archive /tmp/[0-9]+/extractTar.tar: path /tmp/[0-9]+/..../test1, does not belongs to dir /tmp/[0-9]+, rel ..../test1$"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tmpDir, tmpFile, err := getTempDirAndFile(t, "extractTar.tar")
			if err != nil {
				t.Errorf("unable to create tmp file: %s", err)
			}

			ensureTar(t, tmpFile.Name(), tt.entries)

			ctx := context.Background()
			err = extractTar(ctx, tmpFile.Name(), tmpDir, agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR)
			if tt.wantErrRegexp == nil && err == nil {
				return
			}

			msg := fmt.Sprintf("%s", err)
			if !tt.wantErrRegexp.MatchString(msg) {
				t.Errorf("Unexpecte error, expect message to match regexp %s, got %s", tt.wantErrRegexp, err)
			}
		})

	}
}
func Test_extractZip(t *testing.T) {
	tests := []struct {
		name          string
		entries       []fileEntry
		wantErrRegexp *regexp.Regexp
	}{
		{
			name: "base case scenario",
			entries: []fileEntry{
				{
					name: "test1", content: []byte("test1"),
				},
				{
					name: "test2", content: []byte("test2"),
				},
			},
			wantErrRegexp: nil,
		},
		{
			name: "zip with vulnerable path, fail with expected error",
			entries: []fileEntry{
				{
					name: "../test1", content: []byte("test1"),
				},
				{
					name: "test2", content: []byte("test2"),
				},
			},
			wantErrRegexp: regexp.MustCompile("^unable to extract zip archive /tmp/[0-9]+/extractZip.zip: path /tmp/test1, does not belongs to dir /tmp/[0-9]+, rel ../test1$"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tmpDir, tmpFile, err := getTempDirAndFile(t, "extractZip.zip")
			if err != nil {
				t.Errorf("unable to create tmp file: %s", err)
			}

			ensureZip(t, tmpFile.Name(), tt.entries)

			err = extractZip(tmpFile.Name(), tmpDir)
			if tt.wantErrRegexp == nil && err == nil {
				return
			}

			msg := fmt.Sprintf("%s", err)
			if !tt.wantErrRegexp.MatchString(msg) {
				t.Errorf("Unexpecte error, expect message to match regexp %s, got %s", tt.wantErrRegexp, err)
			}
		})

	}
}

func getTempDirAndFile(t *testing.T, fileName string) (dir string, file *os.File, err error) {
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		t.Errorf("unable to create tmp dir: %s", err)
		return "", nil, err
	}

	tmpFile, err := os.OpenFile(filepath.Join(tmpDir, fileName), os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		t.Errorf("unable to create tmp file: %s", err)
		return "", nil, err
	}

	return tmpDir, tmpFile, nil
}

func ensureZip(t *testing.T, dst string, entries []fileEntry) {
	fd, err := os.OpenFile(dst, os.O_RDWR, os.ModePerm)
	if err != nil {
		t.Errorf("unable to open file: %s", err)
	}
	w := zip.NewWriter(fd)

	for _, entry := range entries {
		f, err := w.Create(entry.name)
		if err != nil {
			t.Errorf("unable to create file: %s", err)
		}

		if _, err = f.Write(entry.content); err != nil {
			t.Errorf("unable to write content to file: %s", err)
		}
	}

	if err := w.Close(); err != err {
		t.Errorf("unable to close file: %s", err)
	}
}

func ensureTar(t *testing.T, dst string, entries []fileEntry) {
	fd, err := os.OpenFile(dst, os.O_RDWR, os.ModePerm)
	if err != nil {
		t.Errorf("unable to open file: %s", err)
	}
	w := tar.NewWriter(fd)

	for _, entry := range entries {
		hdr := &tar.Header{
			Name: entry.name,
			Mode: 0600,
			Size: int64(len(entry.content)),
		}

		if err := w.WriteHeader(hdr); err != nil {
			t.Errorf("unable to create file: %s", err)
		}

		if _, err = w.Write(entry.content); err != nil {
			t.Errorf("unable to write content to file: %s", err)
		}
	}

	if err := w.Close(); err != err {
		t.Errorf("unable to close file: %s", err)
	}
}

// Test_checkForConflicts verifies that archive extraction does not overwrite existing files.
func Test_checkForConflicts(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := "exists.txt"
	os.WriteFile(filepath.Join(tmpDir, existingFile), []byte("content"), 0644)

	tests := []struct {
		name    string
		files   []string
		wantErr error
	}{
		{
			name:  "no conflicts, want nil error",
			files: []string{"new.txt", "another.txt"},
		},
		{
			name:    "conflict with existing file, want file exists error",
			files:   []string{"exists.txt"},
			wantErr: fmt.Errorf("file exists: %s", filepath.Join(tmpDir, existingFile)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := createTarArchive(t, tt.files)
			gotTar := tar.NewReader(buf)

			gotErr := checkForConflicts(gotTar, tmpDir)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

func setupTestDecompress(t *testing.T, content []byte) (gzipData, xzData, lzmaData, bzip2Data []byte) {
	// GZIP
	var gzipBuf bytes.Buffer
	gw := gzip.NewWriter(&gzipBuf)
	if _, err := gw.Write(content); err != nil {
		t.Fatal(err)
	}
	gw.Close()

	// XZ
	var xzBuf bytes.Buffer
	xw, err := xz.NewWriter(&xzBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xw.Write(content); err != nil {
		t.Fatal(err)
	}
	xw.Close()

	// LZMA
	var lzmaBuf bytes.Buffer
	lw, err := lzma.NewWriter2(&lzmaBuf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lw.Write(content); err != nil {
		t.Fatal(err)
	}
	lw.Close()

	// BZIP2 (pre-generated hex for "dummy content")
	bzip2Data = []byte{
		0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0x7f, 0x40, 0x3b, 0xb8, 0x00, 0x00,
		0x01, 0x11, 0x80, 0x40, 0x00, 0x0e, 0x03, 0x86, 0x20, 0x20, 0x00, 0x22, 0x00, 0x69, 0xea, 0x10,
		0x03, 0x02, 0x39, 0x84, 0x84, 0x31, 0x9e, 0x2e, 0xe4, 0x8a, 0x70, 0xa1, 0x20, 0xfe, 0x80, 0x77,
		0x70,
	}

	return gzipBuf.Bytes(), xzBuf.Bytes(), lzmaBuf.Bytes(), bzip2Data
}

// Test_decompress verifies that the decompress function correctly identifies different archive formats.
func Test_decompress(t *testing.T) {
	content := []byte("dummy content")
	gzipData, xzData, lzmaData, bzip2Data := setupTestDecompress(t, content)

	tests := []struct {
		name        string
		archiveType agentendpointpb.SoftwareRecipe_Step_ExtractArchive_ArchiveType
		data        []byte
		wantErr     error
	}{
		{
			name:        "archive type TAR, want nil",
			archiveType: agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR,
			data:        content,
			wantErr:     nil,
		},
		{
			name:        "archive type TAR_GZIP, want nil",
			archiveType: agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_GZIP,
			data:        gzipData,
			wantErr:     nil,
		},
		{
			name:        "archive type TAR_BZIP, want nil",
			archiveType: agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_BZIP,
			data:        bzip2Data,
			wantErr:     nil,
		},
		{
			name:        "archive type TAR_LZMA, want nil",
			archiveType: agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_LZMA,
			data:        lzmaData,
			wantErr:     nil,
		},
		{
			name:        "archive type TAR_XZ, want nil",
			archiveType: agentendpointpb.SoftwareRecipe_Step_ExtractArchive_TAR_XZ,
			data:        xzData,
			wantErr:     nil,
		},
		{
			name:        "unrecognized archive type, want error",
			archiveType: 999,
			data:        content,
			wantErr:     fmt.Errorf("Unrecognized archive type \"999\" when trying to decompress tar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.data)
			gotReader, gotErr := decompress(reader, tt.archiveType)

			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			gotContent, err := io.ReadAll(gotReader)
			if err != nil {
				t.Fatalf("failed to read decompressed content: %v", err)
			}
			utiltest.AssertEquals(t, gotContent, content)
		})
	}
}

func createTarArchive(t *testing.T, files []string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{Name: f}); err != nil {
			t.Fatalf("failed to write tar header for %q: %v", f, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	return &buf
}
