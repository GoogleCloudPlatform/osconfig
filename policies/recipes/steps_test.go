package recipes

import (
	"archive/tar"
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
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
