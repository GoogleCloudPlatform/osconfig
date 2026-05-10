package recipes

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

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

// TestParsePermissions tests the parsePermissions function
func TestParsePermissions(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    os.FileMode
		wantErr error
	}{
		{
			name:  "empty string, want 0755",
			input: "",
			want:  0755,
		},
		{
			name:  "valid octal 0644, want 0644",
			input: "0644",
			want:  0644,
		},
		{
			name:  "valid octal 755, want 0755",
			input: "755",
			want:  0755,
		},
		{
			name:    "invalid octal, want parse error",
			input:   "888",
			wantErr: &strconv.NumError{Func: "ParseUint", Num: "888", Err: strconv.ErrSyntax},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePermissions(tt.input)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			utiltest.AssertEquals(t, got, tt.want)
		})
	}
}

// TestEnsureSymlinkBelongsToDir tests the ensureSymlinkBelongsToDir function
func TestEnsureSymlinkBelongsToDir(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "dir")
	otherDir := filepath.Join(tmpDir, "other")
	os.Mkdir(dir, 0755)
	os.Mkdir(otherDir, 0755)

	file := filepath.Join(dir, "file")
	os.WriteFile(file, []byte("test"), 0644)

	otherFile := filepath.Join(otherDir, "other_file")
	os.WriteFile(otherFile, []byte("test"), 0644)

	linkInside := filepath.Join(dir, "link_inside")
	os.Symlink(file, linkInside)

	linkOutside := filepath.Join(dir, "link_outside")
	os.Symlink(otherFile, linkOutside)

	tests := []struct {
		name    string
		dir     string
		link    string
		wantErr error
	}{
		{
			name: "link target inside dir, want nil",
			dir:  dir,
			link: linkInside,
		},
		{
			name:    "link target outside dir, want error",
			dir:     dir,
			link:    linkOutside,
			wantErr: fmt.Errorf("symlink %s, does not belongs to dir %s, rel ../other/other_file", linkOutside, dir),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureSymlinkBelongsToDir(tt.dir, tt.link)
			if tt.wantErr == nil {
				utiltest.AssertErrorMatch(t, err, nil)
				return
			}

			if err == nil {
				t.Errorf("got error nil, want %v", tt.wantErr)
				return
			}

			utiltest.AssertFormatMatch(t, err.Error(), regexp.QuoteMeta(tt.wantErr.Error()))
		})
	}
}

// TestStepCopyFile tests the stepCopyFile function
func TestStepCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "artifact")
	content := "artifact content"
	if err := os.WriteFile(artifactPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(tmpDir, "dest")

	tests := []struct {
		name         string
		step         *agentendpointpb.SoftwareRecipe_Step_CopyFile
		artifacts    map[string]string
		setupFunc    func()
		wantErr      error
		expectedPerm os.FileMode
	}{
		{
			name: "successful copy, want 0644",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Permissions: "0644",
			},
			artifacts:    map[string]string{"art1": artifactPath},
			expectedPerm: 0644,
		},
		{
			name: "file already exists, overwrite false, want error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Overwrite:   false,
			},
			artifacts: map[string]string{"art1": artifactPath},
			setupFunc: func() { os.WriteFile(destPath, []byte("old content"), 0644) },
			wantErr:   fmt.Errorf("file already exists at path %q and Overwrite = false", destPath),
		},
		{
			name: "file already exists, overwrite true, want success",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Overwrite:   true,
				Permissions: "0755",
			},
			artifacts:    map[string]string{"art1": artifactPath},
			setupFunc:    func() { os.WriteFile(destPath, []byte("old content"), 0644) },
			expectedPerm: 0755,
		},
		{
			name: "invalid permissions, want error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art1",
				Destination: destPath,
				Permissions: "888",
			},
			artifacts: map[string]string{"art1": artifactPath},
			wantErr:   &strconv.NumError{Func: "ParseUint", Num: "888", Err: strconv.ErrSyntax},
		},
		{
			name: "artifact not found, want error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "unknown",
				Destination: destPath,
			},
			artifacts: map[string]string{"art1": artifactPath},
			wantErr:   fmt.Errorf("could not find location for artifact \"unknown\""),
		},
		{
			name: "artifact file missing, want error",
			step: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
				ArtifactId:  "art2",
				Destination: destPath,
			},
			artifacts: map[string]string{"art2": filepath.Join(tmpDir, "missing")},
			wantErr:   &os.PathError{Op: "open", Path: filepath.Join(tmpDir, "missing"), Err: fmt.Errorf("no such file or directory")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Remove(destPath) // Ensure clean state
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := stepCopyFile(tt.step, tt.artifacts, nil, "")
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			if tt.wantErr == nil {
				utiltest.AssertFileContents(t, destPath, content)
				info, err := os.Stat(destPath)
				if err != nil {
					t.Fatal(err)
				}
				utiltest.AssertEquals(t, info.Mode().Perm(), tt.expectedPerm)
			}
		})
	}
}
