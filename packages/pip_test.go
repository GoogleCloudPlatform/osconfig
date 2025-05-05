package packages

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"testing"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	utiltest "github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestInstalledPipPackages(t *testing.T) {
	tests := []struct {
		name                  string
		expectedCommandsChain []expectedCommand
		expectedResultsFile   string
		expectedResults       []*PkgInfo
		expectedError         error
	}{
		{
			name: "`pip list` mapped output matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
					stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-format-json.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/linux-pip-list-format-json.expected",
			expectedError:       nil,
		},
		{
			name: "`pip list` non-zero exit code propagates as error",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
					stdout: []byte(""),
					stderr: []byte(""),
					err:    fmt.Errorf("unexpected error"),
				},
			},
			expectedError: fmt.Errorf("error running /usr/bin/pip with args [\"list\" \"--format=json\"]: unexpected error, stdout: \"\", stderr: \"\""),
		},
		{
			name: "`pip list` invalid json output propagates as error",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
					stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-format-json.stdout")[:100],
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedError: fmt.Errorf("unexpected end of JSON input"),
		},
		{
			name: "`pip list` empty json output maps to nil",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
					stdout: []byte("[]"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: nil,
			expectedError:   nil,
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := InstalledPipPackages(ctx)
			if formatError(tt.expectedError) != formatError(err) {
				t.Errorf("InstalledPipPackages: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}

			if tt.expectedResultsFile != "" {
				utiltest.MatchSnapshot(t, pkgs, tt.expectedResultsFile)
			} else if !reflect.DeepEqual(pkgs, tt.expectedResults) {
				t.Errorf("InstalledPipPackages: unexpected result, expect %v, got %v", tt.expectedResults, pkgs)
			}
		})

	}
}

func TestPipUpdates(t *testing.T) {
	tests := []struct {
		name                  string
		expectedCommandsChain []expectedCommand
		expectedResultsFile   string
		expectedResults       []*PkgInfo
		expectedError         error
	}{
		{
			name: "`pip list outdated` mapped output matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json", "--outdated"),
					stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-outdated-format-json.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/linux-pip-list-outdated-format-json.expected",
			expectedError:       nil,
		},
		{
			name: "`pip list outdated` non-zero exit code propagates as error",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json", "--outdated"),
					stdout: []byte(""),
					stderr: []byte(""),
					err:    fmt.Errorf("unexpected error"),
				},
			},
			expectedError: fmt.Errorf("error running /usr/bin/pip with args [\"list\" \"--format=json\" \"--outdated\"]: unexpected error, stdout: \"\", stderr: \"\""),
		},
		{
			name: "`pip list outdated` invalid json output propagates as error",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json", "--outdated"),
					stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-outdated-format-json.stdout")[:100],
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedError: fmt.Errorf("unexpected end of JSON input"),
		},
		{
			name: "`pip list outdated` empty json output maps to nil",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json", "--outdated"),
					stdout: []byte("[]"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: nil,
			expectedError:   nil,
		},
	}

	for _, tt := range tests {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
		runner = mockCommandRunner

		ctx := context.Background()
		t.Run(tt.name, func(t *testing.T) {
			setExpectations(mockCommandRunner, tt.expectedCommandsChain)

			pkgs, err := PipUpdates(ctx)
			if formatError(tt.expectedError) != formatError(err) {
				t.Errorf("PipUpdates: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}

			if tt.expectedResultsFile != "" {
				utiltest.MatchSnapshot(t, pkgs, tt.expectedResultsFile)
			} else if !reflect.DeepEqual(pkgs, tt.expectedResults) {
				t.Errorf("PipUpdates: unexpected result, expect %v, got %v", tt.expectedResults, pkgs)
			}
		})

	}
}
