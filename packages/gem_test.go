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

func TestInstalledGemPackages(t *testing.T) {
	tests := []struct {
		name                  string
		expectedCommandsChain []expectedCommand
		expectedResultsFile   string
		expectedResults       []*PkgInfo
		expectedError         error
	}{
		{
			name: "`gem list` mapped output matches snapshot",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/gem", "list", "--local"),
					stdout: utiltest.BytesFromFile(t, "./testdata/linux-gem-list-local.stdout"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResultsFile: "./testdata/linux-gem-list-local.expected",
			expectedError:       nil,
		},
		{
			name: "`gem list` non-zero exit code propagates as error",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/gem", "list", "--local"),
					stdout: []byte(""),
					stderr: []byte(""),
					err:    fmt.Errorf("unexpected error"),
				},
			},
			expectedError: fmt.Errorf("error running /usr/bin/gem with args [\"list\" \"--local\"]: unexpected error, stdout: \"\", stderr: \"\""),
		},
		{
			name: "`gem list` empty file output maps to nil",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/gem", "list", "--local"),
					stdout: []byte(""),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: nil,
			expectedError:   nil,
		},
		{
			name: "`gem list` skip invalid entry without an error",
			expectedCommandsChain: []expectedCommand{
				{
					cmd:    exec.Command("/usr/bin/gem", "list", "--local"),
					stdout: []byte("\n*** LOCAL GEMS ***\nuri \nwebrick (default: 1.6.0)\nxmlrpc (0.3.0)"),
					stderr: []byte(""),
					err:    nil,
				},
			},
			expectedResults: []*PkgInfo{
				{Name: "webrick", Arch: noarch, Version: "1.6.0", Type: "gem"},
				{Name: "xmlrpc", Arch: noarch, Version: "0.3.0", Type: "gem"},
			},
			expectedError: nil,
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

			pkgs, err := InstalledGemPackages(ctx)
			if formatError(tt.expectedError) != formatError(err) {
				t.Errorf("InstalledGemPackages: unexpected error, expect %q, got %q", formatError(tt.expectedError), formatError(err))
			}

			if tt.expectedResultsFile != "" {
				utiltest.MatchSnapshot(t, pkgs, tt.expectedResultsFile)
			} else if !reflect.DeepEqual(pkgs, tt.expectedResults) {
				t.Errorf("InstalledGemPackages: unexpected result, expect %v, got %v", tt.expectedResults, pkgs)
			}
		})

	}

}
