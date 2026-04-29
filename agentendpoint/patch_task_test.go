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

package agentendpoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

// TestReportFailed verifies that reportFailed correctly reports a failed state to the server with the expected error message.
func TestReportFailed(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	taskID := "test-task"
	pt := &patchTask{
		client: tc.client,
		TaskID: taskID,
	}

	errMsg := "test error message"
	if err := pt.reportFailed(ctx, errMsg); err != nil {
		t.Fatalf("reportFailed error: %v", err)
	}

	if srv.lastReportTaskCompleteRequest == nil {
		t.Fatal("ReportTaskComplete was not called")
	}

	got := srv.lastReportTaskCompleteRequest
	utiltest.AssertEquals(t, got.TaskId, taskID)
	utiltest.AssertEquals(t, got.ErrorMessage, errMsg)

	output, ok := got.Output.(*agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput)
	if !ok {
		t.Fatal("Output is not ApplyPatchesTaskOutput")
	}
	if output.ApplyPatchesTaskOutput.State != agentendpointpb.ApplyPatchesTaskOutput_FAILED {
		t.Errorf("State = %v, want %v", output.ApplyPatchesTaskOutput.State, agentendpointpb.ApplyPatchesTaskOutput_FAILED)
	}
}

// TestHandleErrorState verifies that handleErrorState correctly dispatches to reportCanceled or reportFailed.
func TestHandleErrorState(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "handleErrorState should report errServerCancel",
			err:     errServerCancel,
			wantErr: errServerCancel,
		},
		{
			name:    "handleErrorState should report generic error",
			err:     fmt.Errorf("generic error"),
			wantErr: errors.New("generic error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := &patchTask{
				client: tc.client,
				TaskID: "test-task",
			}
			err := pt.handleErrorState(ctx, tt.err.Error(), tt.err)
			utiltest.AssertErrorMatch(t, err, nil)
			utiltest.AssertEquals(t, srv.lastReportTaskCompleteRequest.ErrorMessage, tt.wantErr.Error())
		})
	}
}

// TestSetStep verifies that setStep correctly updates the task step and saves the state file with the correct information.
func TestSetStep(t *testing.T) {
	td := t.TempDir()

	pt := &patchTask{
		TaskID: "test-task",
		state:  &taskState{},
	}

	stateFile := filepath.Join(td, "testState")
	if err := withStateFile(stateFile, func() error {
		return pt.setStep(patching)
	}); err != nil {
		t.Fatalf("setStep error: %v", err)
	}

	utiltest.AssertEquals(t, string(pt.PatchStep), patching)

	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file was not created")
	}
}

// TestReportContinuingState verifies that reportContinuingState correctly reports task progress.
func TestReportContinuingState(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	td := t.TempDir()

	pt := &patchTask{
		client: tc.client,
		TaskID: "test-task",
		state:  &taskState{},
	}

	patchState := agentendpointpb.ApplyPatchesTaskProgress_STARTED
	stateFile := filepath.Join(td, "testState")
	if err := withStateFile(stateFile, func() error {
		return pt.reportContinuingState(ctx, patchState)
	}); err != nil {
		t.Fatalf("reportContinuingState error: %v", err)
	}

	// Test deduplication - calling again immediately should not trigger a second report (returns nil early)
	if err := withStateFile(stateFile, func() error {
		return pt.reportContinuingState(ctx, patchState)
	}); err != nil {
		t.Fatalf("reportContinuingState deduplication error: %v", err)
	}
}

// TestRebootIfNeededSafe verifies the reboot logic for configurations that don't trigger actual system calls.
func TestRebootIfNeededSafe(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	tests := []struct {
		name         string
		rebootConfig agentendpointpb.PatchConfig_RebootConfig
		dryRun       bool
		prePatch     bool
		wantErr      error
	}{
		{
			name:         "reboot config NEVER, want nil",
			rebootConfig: agentendpointpb.PatchConfig_NEVER,
			wantErr:      nil,
		},
		{
			name:         "reboot config ALWAYS with dry run, want nil",
			rebootConfig: agentendpointpb.PatchConfig_ALWAYS,
			dryRun:       true,
			prePatch:     false,
			wantErr:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := &patchTask{
				client: tc.client,
				TaskID: "test-task",
				Task: &applyPatchesTask{
					&agentendpointpb.ApplyPatchesTask{
						PatchConfig: &agentendpointpb.PatchConfig{RebootConfig: tt.rebootConfig},
						DryRun:      tt.dryRun,
					},
				},
				state: &taskState{},
			}

			err := pt.rebootIfNeeded(ctx, tt.prePatch)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// TestRunPanicRecovery triggers a panic inside the run loop and checks if it's caught and reported as a failure.
func TestRunPanicRecovery(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	pt := &patchTask{
		client:    tc.client,
		TaskID:    "panic-task",
		state:     &taskState{},
		PatchStep: prePatch,
	}

	// run() returns the error from the panic recovery
	err = pt.run(ctx)
	if err == nil {
		t.Fatal("run() expected error from panic recovery, got nil")
	}

	if srv.lastReportTaskCompleteRequest == nil {
		t.Fatal("ReportTaskComplete was not called after panic")
	}

	output, ok := srv.lastReportTaskCompleteRequest.Output.(*agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput)
	if !ok {
		t.Fatal("Output is not ApplyPatchesTaskOutput")
	}
	utiltest.AssertEquals(t, output.ApplyPatchesTaskOutput.State, agentendpointpb.ApplyPatchesTaskOutput_FAILED)
}

// TestPatchTaskErrorPaths verifies the error handling logic in various patchTask methods using a table-driven approach.
func TestPatchTaskErrorPaths(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	// Close immediately to trigger errors on API calls.
	tc.close()

	pt := &patchTask{
		state:  &taskState{},
		client: tc.client,
		TaskID: "test-task",
	}

	tests := []struct {
		name      string
		operation func() error
		wantErr   error
	}{
		{
			name: "invalid state file during complete, want success (nil error)",
			operation: func() error {
				return withInvalidStateFile(func() error {
					pt.complete(ctx)
					return nil
				})
			},
			wantErr: nil,
		},
		{
			name: "invalid state file during setStep, want error saving step",
			operation: func() error {
				return withInvalidStateFile(func() error {
					return pt.setStep(patching)
				})
			},
			wantErr: errors.New("error saving state: mkdir /proc/invalid: no such file or directory"),
		},
		{
			name: "continuing state report on closed client, want connection error",
			operation: func() error {
				return pt.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_STARTED)
			},
			wantErr: errors.New("error reporting state STARTED: error calling ReportTaskProgress: code: \"Canceled\", message: \"grpc: the client connection is closing\", details: []"),
		},
		{
			name: "completed state report on closed client, want connection error",
			operation: func() error {
				return pt.reportCompletedState(ctx, "error", &agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput{})
			},
			wantErr: errors.New("error reporting completed state: error calling ReportTaskComplete: code: \"Canceled\", message: \"grpc: the client connection is closing\", details: []"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// TestReportContinuingStateStop verifies that reportContinuingState returns errServerCancel when STOP directive is received.
func TestReportContinuingStateStop(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	pt := &patchTask{
		client: tc.client,
		TaskID: "test-task",
	}

	// Set the directive to STOP.
	srv.taskDirective = agentendpointpb.TaskDirective_STOP

	err = pt.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_STARTED)
	utiltest.AssertErrorMatch(t, err, errServerCancel)
}

func withStateFile(path string, f func() error) error {
	oldStateFile := taskStateFile
	taskStateFile = path
	defer func() { taskStateFile = oldStateFile }()
	return f()
}

func withInvalidStateFile(f func() error) error {
	return withStateFile("/proc/invalid/path/state", f)
}
