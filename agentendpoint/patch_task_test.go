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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
			utiltest.AssertErrorMatch(t, err, tt.wantErr, utiltest.EquateErrorMessage)
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

func withSystemRebootRequired(fn func(context.Context) (bool, error), f func()) {
	orig := systemRebootRequired
	systemRebootRequired = fn
	defer func() { systemRebootRequired = orig }()
	f()
}

// TestHandleErrorStateCanceledContext verifies that handleErrorState does not report FAILED when the agent context is canceled.
func TestHandleErrorStateCanceledContext(t *testing.T) {
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(context.Background(), srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pt := &patchTask{
		client: tc.client,
		TaskID: "test-task",
	}

	err = pt.handleErrorState(ctx, "Failed to apply patches", context.Canceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("handleErrorState error = %v, want wrapping context.Canceled", err)
	}
	if srv.lastReportTaskCompleteRequest != nil {
		t.Errorf("ReportTaskComplete should not be called when ctx is canceled, got %v", srv.lastReportTaskCompleteRequest)
	}
}

// TestRunPostPatchPreservesStateOnCanceledContextAndRetriesOnResume simulates a reboot interrupting
// ReportTaskComplete in the postPatch step and verifies the saved state is preserved and retried on resume.
func TestRunPostPatchPreservesStateOnCanceledContextAndRetriesOnResume(t *testing.T) {
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(context.Background(), srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	stateFile := filepath.Join(t.TempDir(), "testState")
	withSystemRebootRequired(func(context.Context) (bool, error) { return false, nil }, func() {
		_ = withStateFile(stateFile, func() error {
			pt := &patchTask{
				client: tc.client,
				TaskID: "test-task",
				state:  &taskState{},
				Task:   &applyPatchesTask{&agentendpointpb.ApplyPatchesTask{}},
			}
			if err := pt.setStep(postPatch); err != nil {
				t.Fatalf("setStep error: %v", err)
			}

			// Simulate system shutdown canceling the agent context during completion reporting.
			canceledCtx, cancel := context.WithCancel(context.Background())
			cancel()

			if err := pt.run(canceledCtx); err == nil {
				t.Fatal("pt.run(canceledCtx) = nil, want error")
			}
			if srv.patchTaskComplete {
				t.Fatal("patchTaskComplete = true, want false when ctx is canceled")
			}

			// State must remain persisted at postPatch for retry after reboot.
			st, err := loadState(stateFile)
			if err != nil {
				t.Fatalf("loadState error: %v", err)
			}
			if st == nil || st.PatchTask == nil {
				t.Fatal("saved PatchTask state was cleared on canceled completion report, want preserved")
			}
			utiltest.AssertEquals(t, st.PatchTask.TaskID, "test-task")
			utiltest.AssertEquals(t, string(st.PatchTask.PatchStep), postPatch)

			// Simulate next boot resuming from saved state.
			st.PatchTask.client = tc.client
			st.PatchTask.state = st
			if err := st.PatchTask.run(context.Background()); err != nil {
				t.Fatalf("resumed pt.run error: %v", err)
			}
			if !srv.patchTaskComplete {
				t.Fatal("patchTaskComplete = false after resume, want true")
			}
			output := srv.lastReportTaskCompleteRequest.GetApplyPatchesTaskOutput()
			if output == nil || output.GetState() != agentendpointpb.ApplyPatchesTaskOutput_SUCCEEDED {
				t.Errorf("reported output = %v, want SUCCEEDED", output)
			}

			// State must now be cleared after successful completion report.
			st, err = loadState(stateFile)
			if err != nil {
				t.Fatalf("loadState after completion error: %v", err)
			}
			if st != nil && st.PatchTask != nil {
				t.Errorf("saved PatchTask state was not cleared after successful completion: %+v", st.PatchTask)
			}
			return nil
		})
	})
}

// TestRunPatchingPreservesStateOnCanceledContext verifies that a shutdown during the patching step
// neither reports FAILED nor clears the saved task state.
func TestRunPatchingPreservesStateOnCanceledContext(t *testing.T) {
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(context.Background(), srv)
	if err != nil {
		t.Fatalf("newTestClient error: %v", err)
	}
	defer tc.s.Stop()

	stateFile := filepath.Join(t.TempDir(), "testState")
	_ = withStateFile(stateFile, func() error {
		pt := &patchTask{
			client: tc.client,
			TaskID: "test-task",
			state:  &taskState{},
			Task:   &applyPatchesTask{&agentendpointpb.ApplyPatchesTask{}},
		}
		if err := pt.setStep(patching); err != nil {
			t.Fatalf("setStep error: %v", err)
		}

		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := pt.run(canceledCtx); err == nil {
			t.Fatal("pt.run(canceledCtx) = nil, want error")
		}
		if srv.lastReportTaskCompleteRequest != nil {
			t.Errorf("ReportTaskComplete should not be called on canceled ctx, got %v", srv.lastReportTaskCompleteRequest)
		}

		st, err := loadState(stateFile)
		if err != nil {
			t.Fatalf("loadState error: %v", err)
		}
		if st == nil || st.PatchTask == nil {
			t.Fatal("saved PatchTask state was cleared on canceled patching step, want preserved")
		}
		utiltest.AssertEquals(t, string(st.PatchTask.PatchStep), patching)
		return nil
	})
}

// TestRunPostPatchStateClearingByServerError verifies that permanent backend rejections clear
// saved state while transient/cancellation failures preserve it for retry.
func TestRunPostPatchStateClearingByServerError(t *testing.T) {
	tests := []struct {
		name               string
		serverErr          error
		wantErr            bool
		wantStatePreserved bool
	}{
		{
			name:               "success clears state",
			serverErr:          nil,
			wantErr:            false,
			wantStatePreserved: false,
		},
		{
			name:               "Canceled preserves state",
			serverErr:          status.Error(codes.Canceled, "context canceled"),
			wantErr:            true,
			wantStatePreserved: true,
		},
		{
			name:               "InvalidArgument clears state",
			serverErr:          status.Error(codes.InvalidArgument, "Invalid taskId"),
			wantErr:            true,
			wantStatePreserved: false,
		},
		{
			name:               "FailedPrecondition clears state",
			serverErr:          status.Error(codes.FailedPrecondition, "Task is not started yet"),
			wantErr:            true,
			wantStatePreserved: false,
		},
		{
			name:               "NotFound clears state",
			serverErr:          status.Error(codes.NotFound, "not found"),
			wantErr:            true,
			wantStatePreserved: false,
		},
		{
			name:               "PermissionDenied clears state",
			serverErr:          status.Error(codes.PermissionDenied, "permission denied"),
			wantErr:            true,
			wantStatePreserved: false,
		},
		{
			name:               "Unimplemented clears state",
			serverErr:          status.Error(codes.Unimplemented, "unimplemented"),
			wantErr:            true,
			wantStatePreserved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newAgentEndpointServiceTestServer()
			srv.reportTaskCompleteErr = tt.serverErr
			tc, err := newTestClient(context.Background(), srv)
			if err != nil {
				t.Fatalf("newTestClient error: %v", err)
			}
			defer tc.s.Stop()

			stateFile := filepath.Join(t.TempDir(), "testState")
			withSystemRebootRequired(func(context.Context) (bool, error) { return false, nil }, func() {
				_ = withStateFile(stateFile, func() error {
					pt := &patchTask{
						client: tc.client,
						TaskID: "test-task",
						state:  &taskState{},
						Task:   &applyPatchesTask{&agentendpointpb.ApplyPatchesTask{}},
					}
					if err := pt.setStep(postPatch); err != nil {
						t.Fatalf("setStep error: %v", err)
					}

					err := pt.run(context.Background())
					if (err != nil) != tt.wantErr {
						t.Fatalf("pt.run() error = %v, wantErr = %v", err, tt.wantErr)
					}

					st, err := loadState(stateFile)
					if err != nil {
						t.Fatalf("loadState error: %v", err)
					}
					statePreserved := st != nil && st.PatchTask != nil
					if statePreserved != tt.wantStatePreserved {
						t.Errorf("statePreserved = %v, want %v", statePreserved, tt.wantStatePreserved)
					}
					return nil
				})
			})
		})
	}
}
