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
	"strconv"
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

// Test_parsePermissions verifies that octal permission strings are correctly parsed into os.FileMode.
func Test_parsePermissions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPerm os.FileMode
		wantErr  error
	}{
		{
			name:     "empty string, want 0755",
			input:    "",
			wantPerm: 0755,
			wantErr:  nil,
		},
		{
			name:     "valid octal 0644, want 0644",
			input:    "0644",
			wantPerm: 0644,
			wantErr:  nil,
		},
		{
			name:     "valid octal 755, want 0755",
			input:    "755",
			wantPerm: 0755,
			wantErr:  nil,
		},
		{
			name:    "invalid octal, want parse error",
			input:   "888",
			wantErr: &strconv.NumError{Func: "ParseUint", Num: "888", Err: strconv.ErrSyntax},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPerm, gotErr := parsePermissions(tt.input)
			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotPerm, tt.wantPerm)
		})
	}
}

func setupSymlinkTest(t *testing.T) (dir, linkInside, linkOutside string) {
	tmpDir := t.TempDir()
	dir = filepath.Join(tmpDir, "dir")
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(otherDir, 0755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(otherDir, "other_file")
	if err := os.WriteFile(otherFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	linkInside = filepath.Join(dir, "link_inside")
	if err := os.Symlink(file, linkInside); err != nil {
		t.Fatal(err)
	}
	linkOutside = filepath.Join(dir, "link_outside")
	if err := os.Symlink(otherFile, linkOutside); err != nil {
		t.Fatal(err)
	}

	return dir, linkInside, linkOutside
}

// Test_ensureSymlinkBelongsToDir ensures that symlinks do not point to locations outside the designated directory.
func Test_ensureSymlinkBelongsToDir(t *testing.T) {
	dir, linkInside, linkOutside := setupSymlinkTest(t)

	tests := []struct {
		name    string
		link    string
		wantErr error
	}{
		{
			name:    "link target inside dir, want nil error",
			link:    linkInside,
			wantErr: nil,
		},
		{
			name:    "link target outside dir, want outside link error",
			link:    linkOutside,
			wantErr: fmt.Errorf("symlink %s, does not belongs to dir %s, rel ../other/other_file", linkOutside, dir),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureSymlinkBelongsToDir(dir, tt.link)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// Test_stepCopyFile verifies the file copying logic, including artifact resolution, overwrite behavior, and permission setting.
func Test_stepCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "artifact")
	content := "artifact content"
	if err := os.WriteFile(artifactPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(tmpDir, "dest")

	tests := []struct {
		name        string
		step        *agentendpointpb.SoftwareRecipe_Step_CopyFile
		artifacts   map[string]string
		setupFunc   func()
		wantErr     error
		wantContent string
		wantPerm    os.FileMode
	}{
		{
			name: "successful copy, want nil error and 0644 permission",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Permissions: "0644",
			},
			artifacts:   map[string]string{"art1": artifactPath},
			setupFunc:   func() {},
			wantContent: content,
			wantPerm:    0644,
		},
		{
			name: "file already exists and overwrite false, want file exists error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Overwrite:   false,
			},
			artifacts:   map[string]string{"art1": artifactPath},
			setupFunc:   func() { os.WriteFile(destPath, []byte("old content"), 0644) },
			wantErr:     fmt.Errorf("file already exists at path %q and Overwrite = false", destPath),
			wantContent: "old content",
			wantPerm:    0644,
		},
		{
			name: "file already exists and overwrite true, want nil error and 0755",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Overwrite:   true,
				Permissions: "0755",
			},
			artifacts:   map[string]string{"art1": artifactPath},
			setupFunc:   func() { os.WriteFile(destPath, []byte("old content"), 0644) },
			wantContent: content,
			wantPerm:    0755,
		},
		{
			name: "invalid permissions, want parse error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Permissions: "888",
			},
			artifacts:   map[string]string{"art1": artifactPath},
			setupFunc:   func() {},
			wantErr:     &strconv.NumError{Func: "ParseUint", Num: "888", Err: strconv.ErrSyntax},
			wantContent: "",
			wantPerm:    0,
		},
		{
			name: "artifact not found, want find error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "unknown",
				Destination: destPath,
			},
			artifacts:   map[string]string{"art1": artifactPath},
			setupFunc:   func() {},
			wantErr:     fmt.Errorf("could not find location for artifact \"unknown\""),
			wantContent: "",
			wantPerm:    0,
		},
		{
			name: "artifact file missing, want no file error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art2",
				Destination: destPath,
			},
			artifacts:   map[string]string{"art2": filepath.Join(tmpDir, "missing")},
			setupFunc:   func() {},
			wantErr:     &os.PathError{Op: "open", Path: filepath.Join(tmpDir, "missing"), Err: fmt.Errorf("no such file or directory")},
			wantContent: "",
			wantPerm:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				os.Remove(destPath)
			})
			tt.setupFunc()

			gotErr := stepCopyFile(tt.step, tt.artifacts, nil, "")
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertFileExistsAndContents(t, destPath, tt.wantContent)
			info, err := os.Stat(destPath)
			if err != nil {
				t.Fatal(err)
			}
			utiltest.AssertEquals(t, info.Mode().Perm(), tt.wantPerm)
		})
	}
}

// Test_checkForConflicts verifies that archive extraction does not overwrite existing files.
func Test_checkForConflicts(t *testing.T) {
	existingFilePath := utiltest.WriteToTempFileMust(t, "exists.txt", []byte("content"))
	tmpDir := filepath.Dir(existingFilePath)

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
			wantErr: fmt.Errorf("file exists: %s", existingFilePath),
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
