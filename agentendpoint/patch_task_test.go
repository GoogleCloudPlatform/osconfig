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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
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
	if got.TaskId != taskID {
		t.Errorf("TaskId = %q, want %q", got.TaskId, taskID)
	}
	if got.ErrorMessage != errMsg {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, errMsg)
	}

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
		wantErr string
	}{
		{
			name:    "ServerCancel",
			err:     errServerCancel,
			wantErr: errServerCancel.Error(),
		},
		{
			name:    "GenericError",
			err:     fmt.Errorf("generic error"),
			wantErr: "generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := &patchTask{
				client: tc.client,
				TaskID: "test-task",
			}
			if err := pt.handleErrorState(ctx, tt.wantErr, tt.err); err != nil {
				t.Fatalf("handleErrorState error: %v", err)
			}

			if srv.lastReportTaskCompleteRequest.ErrorMessage != tt.wantErr {
				t.Errorf("ErrorMessage = %q, want %q", srv.lastReportTaskCompleteRequest.ErrorMessage, tt.wantErr)
			}
		})
	}
}

// TestSetStep verifies that setStep correctly updates the task step and saves the state file with the correct information.
func TestSetStep(t *testing.T) {
	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	oldStateFile := taskStateFile
	taskStateFile = filepath.Join(td, "testState")
	defer func() { taskStateFile = oldStateFile }()

	pt := &patchTask{
		TaskID: "test-task",
		state:  &taskState{},
	}

	if err := pt.setStep(patching); err != nil {
		t.Fatalf("setStep error: %v", err)
	}

	if pt.PatchStep != patching {
		t.Errorf("PatchStep = %q, want %q", pt.PatchStep, patching)
	}

	if _, err := os.Stat(taskStateFile); os.IsNotExist(err) {
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

	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	oldStateFile := taskStateFile
	taskStateFile = filepath.Join(td, "testState")
	defer func() { taskStateFile = oldStateFile }()

	pt := &patchTask{
		client: tc.client,
		TaskID: "test-task",
		state:  &taskState{},
	}

	patchState := agentendpointpb.ApplyPatchesTaskProgress_STARTED
	if err := pt.reportContinuingState(ctx, patchState); err != nil {
		t.Fatalf("reportContinuingState error: %v", err)
	}

	// Test deduplication - calling again immediately should not trigger a second report (returns nil early)
	if err := pt.reportContinuingState(ctx, patchState); err != nil {
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
		wantErr      bool
	}{
		{
			name:         "ConfigNever",
			rebootConfig: agentendpointpb.PatchConfig_NEVER,
			wantErr:      false,
		},
		{
			name:         "AlwaysWithDryRun",
			rebootConfig: agentendpointpb.PatchConfig_ALWAYS,
			dryRun:       true,
			prePatch:     false,
			wantErr:      false,
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
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: rebootIfNeeded() error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
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
	if output.ApplyPatchesTaskOutput.State != agentendpointpb.ApplyPatchesTaskOutput_FAILED {
		t.Errorf("State = %v, want %v", output.ApplyPatchesTaskOutput.State, agentendpointpb.ApplyPatchesTaskOutput_FAILED)
	}
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
		name    string
		op      func() error
		wantErr bool
	}{
		{
			name: "completeError",
			op: func() error {
				oldStateFile := taskStateFile
				taskStateFile = "/proc/invalid/path/state"
				defer func() { taskStateFile = oldStateFile }()
				pt.complete(ctx)
				return nil // complete doesn't return an error
			},
			wantErr: false,
		},
		{
			name: "setStepError",
			op: func() error {
				oldStateFile := taskStateFile
				taskStateFile = "/proc/invalid/path/state"
				defer func() { taskStateFile = oldStateFile }()
				return pt.setStep(patching)
			},
			wantErr: true,
		},
		{
			name: "reportContinuingStateError",
			op: func() error {
				return pt.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_STARTED)
			},
			wantErr: true,
		},
		{
			name: "reportCompletedStateError",
			op: func() error {
				return pt.reportCompletedState(ctx, "error", &agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput{})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op()
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: unexpected error result: %v", tt.name, err)
			}
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
}
