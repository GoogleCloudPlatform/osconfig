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

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/golang/protobuf/jsonpb"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

type patchStep string

const (
	prePatch  = "PrePatch"
	patching  = "Patching"
	postPatch = "PostPatch"
)

type patchTask struct {
	client *Client

	lastProgressState map[agentendpointpb.ApplyPatchesTaskProgress_State]time.Time

	TaskID      string
	Task        *applyPatchesTask
	StartedAt   time.Time `json:",omitempty"`
	PatchStep   patchStep `json:",omitempty"`
	RebootCount int
	LogLabels   map[string]string `json:",omitempty"`
	// TODO add Attempts and track number of retries with backoff, jitter, etc.
}

func (r *patchTask) saveState() error {
	return saveState(&taskState{PatchTask: r}, taskStateFile)
}

func (r *patchTask) complete() {
	if err := saveState(nil, taskStateFile); err != nil {
		r.errorf("Error saving state: %v", err)
	}
}

func (r *patchTask) debugf(format string, v ...interface{}) {
	logger.Log(logger.LogEntry{Message: fmt.Sprintf(format, v...), Severity: logger.Debug, Labels: r.LogLabels})
}

func (r *patchTask) infof(format string, v ...interface{}) {
	logger.Log(logger.LogEntry{Message: fmt.Sprintf(format, v...), Severity: logger.Info, Labels: r.LogLabels})
}

func (r *patchTask) errorf(format string, v ...interface{}) {
	logger.Log(logger.LogEntry{Message: fmt.Sprintf(format, v...), Severity: logger.Error, Labels: r.LogLabels})
}

type applyPatchesTask struct {
	*agentendpointpb.ApplyPatchesTask
}

// MarshalJSON marshals a patchConfig using jsonpb.
func (j *applyPatchesTask) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{}
	s, err := m.MarshalToString(j)
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

// UnmarshalJSON unmarshals a patchConfig using jsonpb.
func (j *applyPatchesTask) UnmarshalJSON(b []byte) error {
	return jsonpb.UnmarshalString(string(b), j)
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
	r.errorf(msg)
	return r.reportCompletedState(ctx, msg, &agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput{
		ApplyPatchesTaskOutput: &agentendpointpb.ApplyPatchesTaskOutput{State: agentendpointpb.ApplyPatchesTaskOutput_FAILED},
	})
}

func (r *patchTask) reportCanceled(ctx context.Context) error {
	r.infof("Canceling patch execution")
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
	if err := r.client.ReportTaskComplete(ctx, req); err != nil {
		return fmt.Errorf("error reporting completed state: %v", err)
	}
	return nil
}

func (r *patchTask) reportContinuingState(ctx context.Context, patchState agentendpointpb.ApplyPatchesTaskProgress_State) error {
	st, ok := r.lastProgressState[patchState]
	if ok && st.After(time.Now().Add(-5*time.Second)) {
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
	res, err := r.client.ReportTaskProgress(ctx, req)
	if err != nil {
		return fmt.Errorf("error reporting state %s: %v", patchState, err)
	}
	if res.GetTaskDirective() == agentendpointpb.TaskDirective_STOP {
		return errServerCancel
	}

	r.lastProgressState[patchState] = time.Now()
	return r.saveState()
}

func (r *patchTask) run(ctx context.Context) error {
	r.infof("Beginning patch task")

	for {
		r.debugf("Running PatchStep %q.", r.PatchStep)
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
		case patching:
			if err := r.reportContinuingState(ctx, agentendpointpb.ApplyPatchesTaskProgress_APPLYING_PATCHES); err != nil {
				return r.handleErrorState(ctx, err.Error(), err)
			}
			if err := r.setStep(postPatch); err != nil {
				return r.reportFailed(ctx, fmt.Sprintf("Error saving agent step: %v", err))
			}
		case postPatch:
			if err := r.reportCompletedState(ctx, "", &agentendpointpb.ReportTaskCompleteRequest_ApplyPatchesTaskOutput{
				ApplyPatchesTaskOutput: &agentendpointpb.ApplyPatchesTaskOutput{},
			}); err != nil {
				return err
			}
			r.infof("Successfully completed patch task")
			return nil
		}
	}
}

// RunApplyPatches runs a apply patches task.
func (c *Client) RunApplyPatches(ctx context.Context, task *agentendpointpb.ApplyPatchesTask) error {
	r := &patchTask{
		client:            c,
		lastProgressState: map[agentendpointpb.ApplyPatchesTaskProgress_State]time.Time{},
		Task:              &applyPatchesTask{task},
		LogLabels:         map[string]string{"instance_name": config.Name(), "agent_version": config.Version()},
	}
	r.setStep(prePatch)

	return r.run(ctx)
}
