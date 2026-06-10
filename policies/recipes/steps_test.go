package recipes

import (
	"archive/tar"
	"archive/zip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
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

// Test_stepInstallDpkg verifies that stepInstallDpkg correctly handles system state and artifact mapping.
func Test_stepInstallDpkg(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	ctx := context.Background()
	artifactID := "test-artifact"
	artifactPath := "/path/to/artifact.deb"
	step := &agentendpointpb.SoftwareRecipe_Step_InstallDpkg{ArtifactId: artifactID}

	tests := []struct {
		name         string
		dpkgExists   bool
		artifacts    map[string]string
		expectedCommands []utiltest.ExpectedCommand
		wantErr      error
	}{
		{
			name:       "dpkg missing, want dpkg error",
			dpkgExists: false,
			wantErr:    fmt.Errorf("dpkg does not exist on system"),
		},
		{
			name:       "artifact missing, want not found error",
			dpkgExists: true,
			artifacts:  map[string]string{"other": "path"},
			wantErr:    fmt.Errorf("%q not found in artifact map", artifactID),
		},
		{
			name:       "successful install, want nil",
			dpkgExists: true,
			artifacts:  map[string]string{artifactID: artifactPath},
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/usr/bin/dpkg", "--install", artifactPath)},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.OverrideVariable(t, &packages.DpkgExists, tt.dpkgExists)
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			gotErr := stepInstallDpkg(ctx, step, tt.artifacts)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

// Test_stepInstallRpm verifies that stepInstallRpm correctly handles system state and artifact mapping.
func Test_stepInstallRpm(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	packages.SetCommandRunner(mockCommandRunner)

	ctx := context.Background()
	artifactID := "test-artifact"
	artifactPath := "/path/to/artifact.rpm"
	step := &agentendpointpb.SoftwareRecipe_Step_InstallRpm{ArtifactId: artifactID}

	tests := []struct {
		name         string
		rpmExists    bool
		artifacts    map[string]string
		expectedCommands []utiltest.ExpectedCommand
		wantErr      error
	}{
		{
			name:      "rpm missing, want rpm error",
			rpmExists: false,
			wantErr:   fmt.Errorf("rpm does not exist on system"),
		},
		{
			name:      "artifact missing, want not found error",
			rpmExists: true,
			artifacts: map[string]string{"other": "path"},
			wantErr:   fmt.Errorf("%q not found in artifact map", artifactID),
		},
		{
			name:      "successful install, want nil",
      rpmExists: true,
			artifacts: map[string]string{artifactID: artifactPath},
			expectedCommands: []utiltest.ExpectedCommand{
				{Cmd: exec.Command("/bin/rpm", "--upgrade", "--replacepkgs", "-v", artifactPath)},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.OverrideVariable(t, &packages.RPMExists, tt.rpmExists)
			utiltest.SetExpectedCommands(ctx, mockCommandRunner, tt.expectedCommands)

			gotErr := stepInstallRpm(ctx, step, tt.artifacts)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}

// Test_executeCommand verifies the command execution logic, including allowed exit codes.
func Test_executeCommand(t *testing.T) {
	ctx := context.Background()

	// Pre-generate expected errors to match types and messages exactly.
	exit1Err := exec.Command("sh", "-c", "exit 1").Run()
	noSuchCmdErr := exec.Command("non-existent-command").Run()

	tests := []struct {
		name             string
		cmd              string
		args             []string
		allowedExitCodes []int32
		wantErr          error
	}{
		{
			name:    "exit 0, want nil",
			cmd:     "sh",
			args:    []string{"-c", "exit 0"},
			wantErr: nil,
		},
		{
			name:             "allowed exit code 1, want nil error",
			cmd:              "sh",
			args:             []string{"-c", "exit 1"},
			allowedExitCodes: []int32{1},
			wantErr:          nil,
		},
		{
			name:    "unallowed exit code 1, want exit 1 error",
			cmd:     "sh",
			args:    []string{"-c", "exit 1"},
			wantErr: exit1Err,
		},
		{
			name:    "command not found, want no command error",
			cmd:     "non-existent-command",
			wantErr: noSuchCmdErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := executeCommand(ctx, tt.cmd, tt.args, "", nil, tt.allowedExitCodes)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
		})
	}
}
