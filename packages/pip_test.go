package packages

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"reflect"

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
			name: "Snapshot pip list --format=json",
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
			name: "CLI throw an error",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
					stdout: []byte(""),
					stderr: []byte(""),
					err:    fmt.Errorf("unexpected error"),
				},
			},
			expectedError:       fmt.Errorf("error running /usr/bin/pip with args [\"list\" \"--format=json\"]: unexpected error, stdout: \"\", stderr: \"\""),
		},
		{
			name: "Output from CLI is invalid JSON",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
					stdout: utiltest.BytesFromFile(t, "./testdata/linux-pip-list-format-json.stdout")[:100],
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedError:       fmt.Errorf("unexpected end of JSON input"),

		},
		{
			name: "Empty json",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/pip", "list", "--format=json"),
					stdout: []byte("[]"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: nil,
			expectedError:       nil,

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
