//  Copyright 2026 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package policies

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

// TestChecksum verifies that checksum correctly calculates the SHA256 hash of the input reader.
func TestChecksum(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "basic string data, want correct sha256 hash",
			data: []byte("test data"),
		},
		{
			name: "empty data, want hash of empty data",
			data: []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			h := checksum(r)

			expected := sha256.Sum256(tt.data)
			got := h.Sum(nil)

			utiltest.AssertEquals(t, got, expected[:])
		})
	}
}

// TestWriteIfChanged verifies that writeIfChanged only writes to the file if the content has changed.
func TestWriteIfChanged(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		initialContent []byte
		newContent     []byte
		wantErr        error
	}{
		{
			name:       "new content for non-existent file, want nil error",
			newContent: []byte("content 1"),
			wantErr:    nil,
		},
		{
			name:           "same content as existing file, want nil error",
			initialContent: []byte("content 1"),
			newContent:     []byte("content 1"),
			wantErr:        nil,
		},
		{
			name:           "different content for existing file, want nil error",
			initialContent: []byte("content 1"),
			newContent:     []byte("content 2"),
			wantErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := t.TempDir()
			path := filepath.Join(td, "test_file")

			if tt.initialContent != nil {
				if err := os.WriteFile(path, tt.initialContent, 0644); err != nil {
					t.Fatalf("failed to setup initial file: %v", err)
				}
			}

			err := writeIfChanged(ctx, tt.newContent, path)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			utiltest.AssertFileContents(t, path, string(tt.newContent))
		})
	}
}

// TestInstallRecipes verifies that installRecipes correctly iterates over and installs recipes.
func TestInstallRecipes(t *testing.T) {
	uniqueSuffix := fmt.Sprintf("-%d", time.Now().UnixNano())

	tests := []struct {
		name    string
		egp     *agentendpointpb.EffectiveGuestPolicy
		wantErr error
	}{
		{
			name: "policy without recipes",
			egp:  &agentendpointpb.EffectiveGuestPolicy{},
		},
		{
			name: "policy with nil software recipe",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: nil,
					},
				},
			},
		},
		{
			name: "policy with invalid recipe",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name: "failing-recipe" + uniqueSuffix,
							InstallSteps: []*agentendpointpb.SoftwareRecipe_Step{
								{
									Step: &agentendpointpb.SoftwareRecipe_Step_FileCopy{
										FileCopy: &agentendpointpb.SoftwareRecipe_Step_CopyFile{
											ArtifactId:  "non-existent",
											Destination: "/tmp/dest",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := installRecipes(ctx, tt.egp)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// TestRun covers the Run function.
func TestRun(t *testing.T) {
	Run(context.Background())
}

// Test_run covers the internal run function.
func Test_run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run(ctx)
}

type expectedCommand struct {
	cmd    *exec.Cmd
	envs   []string
	stdout []byte
	stderr []byte
	err    error
}

func setExpectations(mockCommandRunner *utilmocks.MockCommandRunner, expectedCommandsChain []expectedCommand) {
	if len(expectedCommandsChain) == 0 {
		return
	}

	var prev *gomock.Call
	for _, expectedCmd := range expectedCommandsChain {
		cmd := expectedCmd.cmd
		if len(expectedCmd.envs) > 0 {
			cmd.Env = append(os.Environ(), expectedCmd.envs...)
		}

		if prev == nil {
			prev = mockCommandRunner.EXPECT().
				Run(gomock.Any(), utilmocks.EqCmd(cmd)).
				Return(expectedCmd.stdout, expectedCmd.stderr, expectedCmd.err).Times(1)
		} else {
			prev = mockCommandRunner.EXPECT().
				Run(gomock.Any(), utilmocks.EqCmd(cmd)).
				After(prev).
				Return(expectedCmd.stdout, expectedCmd.stderr, expectedCmd.err).Times(1)
		}
	}
}
