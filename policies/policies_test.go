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
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
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
			reader := bytes.NewReader(tt.data)
			hasher := checksum(reader)

			want := sha256.Sum256(tt.data)
			got := hasher.Sum(nil)

			utiltest.AssertEquals(t, got, want[:])
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
		pathPredefined string
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
		{
			name:           "path is a directory, want error",
			newContent:     []byte("content 1"),
			pathPredefined: "/tmp",
			wantErr:        &os.PathError{Op: "open", Path: "/tmp", Err: syscall.EISDIR},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.pathPredefined
			if path == "" {
				path = utiltest.WriteToTempFileMust(t, "test_file", tt.initialContent)
			}

			err := writeIfChanged(ctx, tt.newContent, path)
			utiltest.AssertErrorMatchAndSkip(t, err, tt.wantErr)
			utiltest.AssertFileContents(t, path, string(tt.newContent))
		})
	}
}

// TestInstallRecipesHandlesInputs verifies that installRecipes correctly handles different EffectiveGuestPolicy inputs.
func TestInstallRecipesHandlesInputs(t *testing.T) {
	ctx := context.Background()
	uniqueSuffix := fmt.Sprintf("-%d", time.Now().UnixNano())

	tests := []struct {
		name    string
		egp     *agentendpointpb.EffectiveGuestPolicy
		wantErr error
	}{
		{
			name: "policy without recipes, want nil error",
			egp:  &agentendpointpb.EffectiveGuestPolicy{},
		},
		{
			name: "policy with nil software recipe, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: nil,
					},
				},
			},
		},
		{
			name: "policy with invalid recipe, want installing error",
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
			wantErr: installRecipesError{
				errors: []error{
					fmt.Errorf("Error installing recipe: error running step 0 (CopyFile): could not find location for artifact \"non-existent\""),
				},
			},
		},
		{
			name: "valid policy with one recipe, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name:    "success-recipe" + uniqueSuffix,
							Version: "1.0.0",
						},
					},
				},
			},
		},
		{
			name: "valid policy with multiple recipes, want nil error",
			egp: &agentendpointpb.EffectiveGuestPolicy{
				SoftwareRecipes: []*agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe{
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name:    "recipe-1" + uniqueSuffix,
							Version: "1.0.0",
						},
					},
					{
						SoftwareRecipe: &agentendpointpb.SoftwareRecipe{
							Name:    "recipe-2" + uniqueSuffix,
							Version: "2.1.0",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := installRecipes(ctx, tt.egp)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// TestRunMetadataFailure verifies that run logs an error when metadata server is unreachable.
func TestRunMetadataFailure(t *testing.T) {
	var buf bytes.Buffer
	_ = logger.Init(context.Background(), logger.LogOpts{LoggerName: "test", Debug: true, Writers: []io.Writer{&buf}})

	done := make(chan struct{})
	Run(context.Background())
	// Wait for previous Run task has completed.
	tasker.Enqueue(context.Background(), "signal", func() {
		close(done)
	})
	<-done

	utiltest.AssertFormatMatch(t, buf.String(), `(?s).*Creating new agentendpoint beta client.*`)
	utiltest.AssertFormatMatch(t, buf.String(), `(?s).*Error running LookupEffectiveGuestPolicies: error getting token from metadata.*`)
}
