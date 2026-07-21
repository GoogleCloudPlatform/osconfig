package util

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "Basic file name",
			input:          "test.yaml",
			expectedOutput: "test.yaml",
		},
		{
			name:           "Basic full path",
			input:          "/x/test.yaml",
			expectedOutput: "/x/test.yaml",
		},
		{
			name:           "Relative path",
			input:          "x/test.yaml",
			expectedOutput: "x/test.yaml",
		},
		{
			name:           "Relative path with traversal segment",
			input:          "../x/test.yaml",
			expectedOutput: "x/test.yaml",
		},
		{
			name:           "Relative path with traversal segment",
			input:          "/../x/test.yaml",
			expectedOutput: "/x/test.yaml",
		},
	}

	for _, tt := range tests {
		if result := SanitizePath(tt.input); result != tt.expectedOutput {
			t.Errorf("Test %q failed, expectedOutput %q, got %q", tt.name, tt.expectedOutput, result)
		}
	}
}

func TestExists(t *testing.T) {
	tmpPath := utiltest.WriteToTempFileMust(t, "exists-test", []byte(""))

	tests := []struct {
		name       string
		input      string
		wantExists bool
	}{
		{
			name:       "valid file path, expect true",
			input:      tmpPath,
			wantExists: true,
		},
		{
			name:       "non-existent file path, expect false",
			input:      tmpPath + "-does-not-exist",
			wantExists: false,
		},
		{
			name:       "empty string, expect false",
			input:      "",
			wantExists: false,
		},
		{
			name:       "whitespace string, expect false",
			input:      "   ",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExists := Exists(tt.input)
			utiltest.AssertEquals(t, gotExists, tt.wantExists)
		})
	}
}

func testSuccessCmd() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "echo success msg")
	}
	return exec.Command("echo", "success msg")
}

func testFailCmd() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/c", "echo error msg 1>&2 & exit 1")
	}
	return exec.Command("sh", "-c", "echo 'error msg' >&2; exit 1")
}

func TestDefaultRunnerRun(t *testing.T) {
	runner := &DefaultRunner{}
	ctx := t.Context()

	tests := []struct {
		name       string
		cmd        *exec.Cmd
		wantStdout string
		wantStderr string
		wantErr    error
	}{
		{
			name:       "successful command execution, expect stdout output",
			cmd:        testSuccessCmd(),
			wantStdout: "success msg",
			wantStderr: "",
			wantErr:    nil,
		},
		{
			name:       "failing command execution, expect stderr output and error",
			cmd:        testFailCmd(),
			wantStdout: "",
			wantStderr: "error msg",
			wantErr:    errors.New("exit status 1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStdout, gotStderr, gotErr := runner.Run(ctx, tt.cmd)

			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr, utiltest.EquateErrorMessage)
			utiltest.AssertEquals(t, strings.TrimSpace(string(gotStdout)), tt.wantStdout)
			utiltest.AssertEquals(t, strings.TrimSpace(string(gotStderr)), tt.wantStderr)
		})
	}
}

func TestAtomicWriteFileStream(t *testing.T) {
	tmpDir := t.TempDir()
	existingStreamPath := utiltest.WriteToTempFileMust(t, "test-stream-existing.txt", []byte("old content"))

	content := "test content"
	hasher := sha256.New()
	hasher.Write([]byte(content))
	validChecksum := hex.EncodeToString(hasher.Sum(nil))

	tests := []struct {
		name         string
		input        string
		checksum     string
		content      string
		mode         os.FileMode
		wantChecksum string
		wantErr      error
	}{
		{
			name:         "valid file path, expect checksum output",
			input:        filepath.Join(tmpDir, "test-stream-1.txt"),
			checksum:     validChecksum,
			content:      content,
			mode:         0644,
			wantChecksum: validChecksum,
			wantErr:      nil,
		},
		{
			name:         "existing file path, expect overwrite and checksum output",
			input:        existingStreamPath,
			checksum:     validChecksum,
			content:      content,
			mode:         0644,
			wantChecksum: validChecksum,
			wantErr:      nil,
		},
		{
			name:         "invalid checksum string, expect error",
			input:        filepath.Join(tmpDir, "test-stream-2.txt"),
			checksum:     "bad-checksum",
			content:      content,
			mode:         0644,
			wantChecksum: "",
			wantErr:      fmt.Errorf("got %q for checksum, expected %q", validChecksum, "bad-checksum"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			gotChecksum, gotErr := AtomicWriteFileStream(reader, tt.checksum, tt.input, tt.mode)

			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotChecksum, tt.wantChecksum)
			utiltest.AssertFileContents(t, tt.input, tt.content)
		})
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	existingFilePath := utiltest.WriteToTempFileMust(t, "test-write-existing.txt", []byte("old content"))

	tests := []struct {
		name    string
		input   string
		content []byte
		mode    os.FileMode
	}{
		{
			name:    "valid file path, expect success",
			input:   filepath.Join(tmpDir, "test-write-1.txt"),
			content: []byte("test content"),
			mode:    0644,
		},
		{
			name:    "existing file path, expect overwrite",
			input:   existingFilePath,
			content: []byte("new overwritten content"),
			mode:    0644,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := AtomicWrite(tt.input, tt.content, tt.mode)
			utiltest.AssertErrorMatch(t, gotErr, nil)
			utiltest.AssertFileContents(t, tt.input, string(tt.content))
		})
	}
}
