package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// TestHelperProcess is a helper to mock command execution.
// It runs as a subprocess and exits with 1 if GO_HELPER_FAIL is set.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if os.Getenv("GO_HELPER_FAIL") == "1" {
		fmt.Fprint(os.Stderr, "error msg")
		os.Exit(1)
	}
	fmt.Fprint(os.Stdout, "success msg")
	os.Exit(0)
}

func TestDefaultRunnerRun(t *testing.T) {
	runner := &DefaultRunner{}
	ctx := t.Context()

	tests := []struct {
		name       string
		env        []string
		wantStdout string
		wantStderr string
	}{
		{
			name:       "successful command execution, expect stdout output",
			env:        []string{"GO_WANT_HELPER_PROCESS=1"},
			wantStdout: "success msg",
			wantStderr: "",
		},
		{
			name:       "failing command execution, expect stderr output and error",
			env:        []string{"GO_WANT_HELPER_PROCESS=1", "GO_HELPER_FAIL=1"},
			wantStdout: "",
			wantStderr: "error msg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
			cmd.Env = append(os.Environ(), tt.env...)
			stdout, stderr, _ := runner.Run(ctx, cmd)

			utiltest.AssertEquals(t, string(stdout), tt.wantStdout)
			utiltest.AssertEquals(t, string(stderr), tt.wantStderr)
		})
	}
}

func TestAtomicWriteFileStream_Success(t *testing.T) {
	tmpDir := t.TempDir()

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
	}{
		{
			name:         "valid file path, expect checksum output",
			input:        filepath.Join(tmpDir, "test-stream-1.txt"),
			checksum:     validChecksum,
			content:      content,
			mode:         0644,
			wantChecksum: validChecksum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.content)
			gotChecksum, err := AtomicWriteFileStream(r, tt.checksum, tt.input, tt.mode)

			utiltest.AssertErrorMatch(t, err, nil)
			utiltest.AssertEquals(t, gotChecksum, tt.wantChecksum)
			utiltest.AssertFileContents(t, tt.input, tt.content)
		})
	}
}

func TestAtomicWriteFileStream_Error(t *testing.T) {
	tmpDir := t.TempDir()

	content := "test content"
	hasher := sha256.New()
	hasher.Write([]byte(content))
	validChecksum := hex.EncodeToString(hasher.Sum(nil))

	tests := []struct {
		name            string
		input           string
		checksum        string
		content         string
		mode            os.FileMode
		wantErrorFormat string
	}{
		{
			name:            "invalid checksum string, expect error",
			input:           filepath.Join(tmpDir, "test-stream-2.txt"),
			checksum:        "bad-checksum",
			content:         content,
			mode:            0644,
			wantErrorFormat: fmt.Sprintf("^got %q for checksum, expected %q", validChecksum, "bad-checksum"),
		},
		{
			name:            "invalid directory path, expect error",
			input:           filepath.Join(tmpDir, "does-not-exist", "test-stream.txt"),
			checksum:        "",
			content:         content,
			mode:            0644,
			wantErrorFormat: "^unable to create temp file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.content)
			gotChecksum, gotErr := AtomicWriteFileStream(r, tt.checksum, tt.input, tt.mode)

			utiltest.AssertFormatMatch(t, fmt.Sprint(gotErr), tt.wantErrorFormat)
			utiltest.AssertEquals(t, gotChecksum, "")
		})
	}
}

func TestAtomicWrite_Success(t *testing.T) {
	tmpDir := t.TempDir()

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AtomicWrite(tt.input, tt.content, tt.mode)
			utiltest.AssertErrorMatch(t, err, nil)
			utiltest.AssertFileContents(t, tt.input, string(tt.content))
		})
	}
}

func TestAtomicWrite_Error(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		input           string
		content         []byte
		mode            os.FileMode
		wantErrorFormat string
	}{
		{
			name:            "invalid directory path, expect error",
			input:           filepath.Join(tmpDir, "does-not-exist", "test-write.txt"),
			content:         []byte("test content"),
			mode:            0644,
			wantErrorFormat: "^unable to create temp file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AtomicWrite(tt.input, tt.content, tt.mode)
			utiltest.AssertFormatMatch(t, fmt.Sprint(err), tt.wantErrorFormat)
		})
	}
}
