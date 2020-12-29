//  Copyright 2018 Google Inc. All Rights Reserved.
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
	"time"

	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/ospatch"
	"google.golang.org/protobuf/encoding/protojson"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

func systemRebootRequired(ctx context.Context) (bool, error) {
	return ospatch.SystemRebootRequired(ctx)
}

type patchStep string

const (
	prePatch  = "PrePatch"
	patching  = "Patching"
	postPatch = "PostPatch"
)

type patchTask struct {
	client *Client

	lastProgressState map[agentendpointpb.ApplyPatchesTaskProgress_State]time.Time
	state             *taskState

	TaskID      string
	Task        *applyPatchesTask
	StartedAt   time.Time `json:",omitempty"`
	PatchStep   patchStep `json:",omitempty"`
	RebootCount int

	// TODO: add Attempts and track number of retries with backoff, jitter, etc.
}

func (r *patchTask) saveState() error {
	r.state.PatchTask = r
	return r.state.save(taskStateFile)
}

func (r *patchTask) complete(ctx context.Context) {
	if err := (&taskState{}).save(taskStateFile); err != nil {
		clog.Errorf(ctx, "Error saving state: %v", err)
	}
}

type applyPatchesTask struct {
	*agentendpointpb.ApplyPatchesTask
}

// MarshalJSON marshals a patchConfig using protojson.
func (a *applyPatchesTask) MarshalJSON() ([]byte, error) {
	m := &protojson.MarshalOptions{AllowPartial: true, EmitUnpopulated: false}
	return m.Marshal(a)
}

// UnmarshalJSON unmarshals a patchConfig using protojson.
func (a *applyPatchesTask) UnmarshalJSON(b []byte) error {
	a.ApplyPatchesTask = &agentendpointpb.ApplyPatchesTask{}
	un := &protojson.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}
	return un.Unmarshal(b, a.ApplyPatchesTask)
}

func (r *patchTask) setStep(step patchStep) error {
	r.PatchStep = step
	if err := r.saveState(); err != nil {
		return fmt.Errorf("error saving state: %v", err)
	}
	return nil
}

func (r *patchTask) handleErrorState(ctx context.Context, msg string, err error) error {
	if err == errServerCancel {
		return r.reportCanceled(ctx)
	}
	return r.reportFailed(ctx, msg)
}

func (r *patchTask) reportFailed(ctx context.Context, msg string) error {
	clog.Errorf(ctx, msg)
	return r.reportCompletedState(ctx, msg, &agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput{
		ApplyPatchesTaskOutput: &agentendpointpb.ApplyPatchesTaskOutput{State: agentendpointpb.ApplyPatchesTaskOutput_FAILED},
	})
}

func (r *patchTask) reportCanceled(ctx context.Context) error {
	clog.Infof(ctx, "Canceling patch execution")
	return r.reportCompletedState(ctx, errServerCancel.Error(), &agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput{
		// Is this right? Maybe there should be a canceled state instead.
		ApplyPatchesTaskOutput: &agentendpointpb.ApplyPatchesTaskOutput{State: agentendpointpb.ApplyPatchesTaskOutput_FAILED},
	})
}

func (r *patchTask) reportCompletedState(ctx context.Context, errMsg string, output *agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput) error {
	req := &agentendpointpb.ReportTaskCompleteRequest{
		TaskId:       r.TaskID,
		TaskType:     agentendpointpb.TaskType_APPLY_PATCHES,
		ErrorMessage: errMsg,
		Output:       output,
	}
	if err := r.client.reportTaskComplete(ctx, req); err != nil {
		return fmt.Errorf("error reporting completed state: %v", err)
	}
	return nil
}

func (r *patchTask) reportContinuingState(ctx context.Context, patchState agentendpointpb.ApplyPatchesTaskProgress_State) error {
	st, ok := r.lastProgressState[patchState]
	if ok && st.After(time.Now().Add(sameStateTimeWindow)) {
		// Don't resend the same state more than once every 5s.
		return nil
	}

	req := &agentendpointpb.ReportTaskProgressRequest{
		TaskId:   r.TaskID,
		TaskType: agentendpointpb.TaskType_APPLY_PATCHES,
		Progress: &agentendpointpb.ReportTaskProgressRequest_ApplyPatchesTaskProgress{
			ApplyPatchesTaskProgress: &agentendpointpb.ApplyPatchesTaskProgress{State: patchState},
		},
	}
	res, err := r.client.reportTaskProgress(ctx, req)
	if err != nil {
		return fmt.Errorf("error reporting state %s: %v", patchState, err)
	}
	if res.GetTaskDirective() == agentendpointpb.TaskDirective_STOP {
		return errServerCancel
	}

	if r.lastProgressState == nil {
		r.lastProgressState = make(map[agentendpointpb.ApplyPatchesTaskProgress_State]time.Time)
	}
	r.lastProgressState[patchState] = time.Now()
	return r.saveState()
}

// TODO: Add MaxRebootCount so we don't loop endlessly.

func (r *patchTask) prePatchReboot(ctx context.Context) error {
	return r.rebootIfNeeded(ctx, true)
}

func (r *patchTask) postPatchReboot(ctx context.Context) error {
	return r.rebootIfNeeded(ctx, false)
}

func (r *patchTask) rebootIfNeeded(ctx context.Context, prePatch bool) error {
	var reboot bool
	var err error
	if r.Task.GetPatchConfig().GetRebootConfig() == agentendpointpb.PatchConfig_ALWAYS && !prePatch && r.RebootCount == 0 {
		reboot = true
		clog.Infof(ctx, "PatchConfig RebootConfig set to %s.", agentendpointpb.PatchConfig_ALWAYS)
	} else {
		reboot, err = systemRebootRequired(ctx)
		if err != nil {
			return fmt.Errorf("error checking if a system reboot is required: %v", err)
		}
		if reboot {
			clog.Infof(ctx, "System indicates a reboot is required.")
		} else {
			clog.Infof(ctx, "System indicates a reboot is not required.")
		}
	}

	if !reboot {
		return nil
	}

	if r.Task.GetPatchConfig().GetRebootConfig() == agentendpointpb.PatchConfig_NEVER {
		clog.Infof(ctx, "Skipping reboot because of PatchConfig RebootConfig set to %s.", agentendpointpb.PatchConfig_NEVER)
		return nil
	}

	if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_REBOOTING); err != nil {
		return err
	}

	if r.Task.GetDryRun() {
		clog.Infof(ctx, "Dry run - not rebooting for patch task")
		return nil
	}

	r.RebootCount++
	if err := r.saveState(); err != nil {
		return fmt.Errorf("error saving state: %v", err)
	}
	if err := rebootSystem(); err != nil {
		return fmt.Errorf("failed to reboot system: %v", err)
	}

	// Reboot can take a bit, pause here so other activities don't start.
	for {
		clog.Debugf(ctx, "Waiting for system reboot.")
		time.Sleep(1 * time.Minute)
	}
}

func (r *patchTask) run(ctx context.Context) (err error) {
	ctx = clog.WithLabels(ctx, r.state.Labels)
	clog.Infof(ctx, "Beginning patch task")
	defer func() {
		// This should not happen but the WUA libraries are complicated and
		// recovering with an error is better than crashing.
		if rec := recover(); rec != nil {
			err = fmt.Errorf("Recovered from panic: %v", rec)
			r.reportFailed(ctx, err.Error())
			return
		}
		r.complete(ctx)
		if agentconfig.OSInventoryEnabled() {
			go r.client.ReportInventory(ctx)
		}
	}()

	for {
		clog.Debugf(ctx, "Running PatchStep %q.", r.PatchStep)
		switch r.PatchStep {
		default:
			return r.reportFailed(ctx, fmt.Sprintf("unknown step: %q", r.PatchStep))
		case prePatch:
			r.StartedAt = time.Now()
			if err := r.setStep(patching); err != nil {
				return r.reportFailed(ctx, fmt.Sprintf("Error saving agent step: %v", err))
			}

			if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_STARTED); err != nil {
				return r.handleErrorState(ctx, err.Error(), err)
			}
			if err := r.prePatchReboot(ctx); err != nil {
				return r.handleErrorState(ctx, fmt.Sprintf("Error running prePatchReboot: %v", err), err)
			}
		case patching:
			if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_APPLYING_PATCHES); err != nil {
				return r.handleErrorState(ctx, err.Error(), err)
			}
			if err := r.runUpdates(ctx); err != nil {
				return r.handleErrorState(ctx, fmt.Sprintf("Failed to apply patches: %v", err), err)
			}
			if err := r.postPatchReboot(ctx); err != nil {
				return r.handleErrorState(ctx, fmt.Sprintf("Error running postPatchReboot: %v", err), err)
			}
			// We have not rebooted so patching is complete.
			if err := r.setStep(postPatch); err != nil {
				return r.reportFailed(ctx, fmt.Sprintf("Error saving agent step: %v", err))
			}
		case postPatch:
			isRebootRequired, err := systemRebootRequired(ctx)
			if err != nil {
				return r.reportFailed(ctx, fmt.Sprintf("Error checking if system reboot is required: %v", err))
			}

			finalState := agentendpointpb.ApplyPatchesTaskOutput_SUCCEEDED
			if isRebootRequired {
				finalState = agentendpointpb.ApplyPatchesTaskOutput_SUCCEEDED_REBOOT_REQUIRED
			}

			if err := r.reportCompletedState(ctx, "", &agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput{
				ApplyPatchesTaskOutput: &agentendpointpb.ApplyPatchesTaskOutput{State: finalState},
			}); err != nil {
				return fmt.Errorf("failed to report state %s: %v", finalState, err)
			}
			clog.Infof(ctx, "Successfully completed patch task")
			return nil
		}
	}
}

// RunApplyPatches runs a apply patches task.
func (c *Client) RunApplyPatches(ctx context.Context, task *agentendpointpb.Task) error {
	r := &patchTask{
		state:  &taskState{Labels: task.GetServiceLabels()},
		TaskID: task.GetTaskId(),
		client: c,
		Task:   &applyPatchesTask{task.GetApplyPatchesTask()},
	}
	r.setStep(prePatch)

	return r.run(ctx)
}
