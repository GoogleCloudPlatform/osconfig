//  Copyright 2019 Google Inc. All Rights Reserved.
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

	"github.com/GoogleCloudPlatform/osconfig/clog"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

const (
	numExecutionSteps       = 4
	validationStepIndex     = 0
	checkStateStepIndex     = 1
	enforcementStepIndex    = 2
	postCheckStateStepIndex = 3
)

type configTask struct {
	client *Client

	TaskID    string
	Task      *applyConfigTask
	StartedAt time.Time `json:",omitempty"`

	// ApplyConfigTaskOutput result
	results           *agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults
	postCheckRequired bool
	assignments       map[string]*assignment
}

type applyConfigTask struct {
	*agentendpointpb.ApplyConfigTask
}

type assignment struct {
	policies map[string]*policy
}

type policy struct {
	resources map[string]*resource
	hasError  bool
}

type resource struct {
	enforcementNeeded bool

	packageResource        *agentendpointpb.PackageResource
	repositoryResource     *agentendpointpb.RepositoryResource
	execResource           *agentendpointpb.ExecResource
	fileResource           *agentendpointpb.FileResource
	extractArchiveResource *agentendpointpb.ExtractArchiveResource
	serviceResource        *agentendpointpb.ServiceResource
}

func (r *resource) unmarshal(ctx context.Context, res *agentendpointpb.ApplyConfigTask_Config_Resource) error {
	// TODO: implement
	return nil
}

func (r *resource) checkState(ctx context.Context) error {
	// TODO: implement
	r.enforcementNeeded = false
	return nil
}

func (r *resource) runEnforcement(ctx context.Context) error {
	// TODO: implement
	return nil
}

func (c *configTask) reportCompletedState(ctx context.Context, errMsg string, state agentendpointpb.ApplyConfigTaskOutput_State) error {
	req := &agentendpointpb.ReportTaskCompleteRequest{
		TaskId:       c.TaskID,
		TaskType:     agentendpointpb.TaskType_APPLY_CONFIG_TASK,
		ErrorMessage: errMsg,
		Output: &agentendpointpb.ReportTaskCompleteRequest_ApplyConfigTaskOutput{
			ApplyConfigTaskOutput: &agentendpointpb.ApplyConfigTaskOutput{State: state, ConfigAssignmentResults: c.results},
		},
	}
	if err := c.client.reportTaskComplete(ctx, req); err != nil {
		return fmt.Errorf("error reporting completed state: %v", err)
	}
	return nil
}

func (c *configTask) handleErrorState(ctx context.Context, msg string, err error) error {
	if err == errServerCancel {
		clog.Infof(ctx, "Cancelling config run: %v", errServerCancel)
		return c.reportCompletedState(ctx, errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED)
	}
	msg = fmt.Sprintf("%s: %v", msg, err)
	clog.Errorf(ctx, msg)
	return c.reportCompletedState(ctx, msg, agentendpointpb.ApplyConfigTaskOutput_FAILED)
}

func (c *configTask) reportContinuingState(ctx context.Context, configState agentendpointpb.ApplyConfigTaskProgress_State) error {
	req := &agentendpointpb.ReportTaskProgressRequest{
		TaskId:   c.TaskID,
		TaskType: agentendpointpb.TaskType_APPLY_CONFIG_TASK,
		Progress: &agentendpointpb.ReportTaskProgressRequest_ApplyConfigTaskProgress{
			ApplyConfigTaskProgress: &agentendpointpb.ApplyConfigTaskProgress{State: configState},
		},
	}
	res, err := c.client.reportTaskProgress(ctx, req)
	if err != nil {
		return fmt.Errorf("error reporting task progress %s: %v", configState, err)
	}
	if res.GetTaskDirective() == agentendpointpb.TaskDirective_STOP {
		return errServerCancel
	}
	return nil
}

func (c *configTask) validation(ctx context.Context) {
	// First populate validation results and internal assignment state.
	c.assignments = map[string]*assignment{}
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		asgnmnt := &assignment{policies: map[string]*policy{}}
		c.assignments[a.GetConfigAssignment()] = asgnmnt
		aResult := c.results.GetResults()[i]
		for i, p := range a.GetPolicies() {
			plcy := &policy{resources: map[string]*resource{}}
			asgnmnt.policies[p.GetId()] = plcy
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			for i, r := range p.GetResources() {
				plcy.resources[r.GetId()] = &resource{}
				result := pResult.GetResourceResults().GetResults()[i]
				result.GetExecutionSteps()[validationStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
					Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_Validation{
						Validation: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation{
							Outcome: agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation_SKIPPED,
						},
					},
				}
			}
		}
	}

	// Unmarshal all resources.
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": a.GetConfigAssignment()})
		asgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, p := range a.GetPolicies() {
			plcy := asgnmnt.policies[p.GetId()]
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			for i, r := range p.GetResources() {
				res := plcy.resources[r.GetId()]

				outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation_OK
				errMsg := ""
				err := res.unmarshal(ctx, r)
				if err != nil {
					outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation_RESOURCE_PAYLOAD_CONVERSION_ERROR
					plcy.hasError = true
					errMsg = fmt.Sprintf("Error unmarshalling resource: %v", err)
					clog.Errorf(ctx, errMsg)
				}

				result := pResult.GetResourceResults().GetResults()[i]
				result.GetExecutionSteps()[validationStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
					Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_Validation{
						Validation: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation{
							Outcome: outcome,
						},
					},
					ErrMsg: errMsg,
				}

				// Continue to unmarshal all resources even if one has an error.
			}
		}
	}

	// TODO: check for resouce conflicts
}

func (c *configTask) checkState(ctx context.Context) {
	// First populate check state results (for policies that do not have validation errors).
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		asgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, p := range a.GetPolicies() {
			plcy := asgnmnt.policies[p.GetId()]
			// Skip state check if this policy already has an error from a previous step.
			if plcy.hasError {
				continue
			}
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			for i := range p.GetResources() {
				result := pResult.GetResourceResults().GetResults()[i]
				result.GetExecutionSteps()[checkStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
					Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_CheckDesiredState{
						CheckDesiredState: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState{
							Outcome: agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_SKIPPED,
						},
					},
				}
			}
		}
	}

	// Actually run check state.
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": a.GetConfigAssignment()})
		asgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, p := range a.GetPolicies() {
			ctx = clog.WithLabels(ctx, map[string]string{"policy_id": p.GetId()})
			plcy := asgnmnt.policies[p.GetId()]
			// Skip state check if this policy already has an error from a previous step.
			if plcy.hasError {
				clog.Debugf(ctx, "Policy has error, skipping state check.")
				continue
			}
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			for i, r := range p.GetResources() {
				ctx = clog.WithLabels(ctx, map[string]string{"resource_id": r.GetId()})
				res := plcy.resources[r.GetId()]

				outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_IN_DESIRED_STATE
				errMsg := ""
				err := res.checkState(ctx)
				if err != nil {
					outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_ERROR
					plcy.hasError = true
					errMsg = fmt.Sprintf("Error running desired state check: %v", err)
					clog.Errorf(ctx, errMsg)
				}

				if res.enforcementNeeded {
					outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_NOT_IN_DESIRED_STATE
				}

				result := pResult.GetResourceResults().GetResults()[i]
				result.GetExecutionSteps()[checkStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
					Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_CheckDesiredState{
						CheckDesiredState: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState{
							Outcome: outcome,
						},
					},
					ErrMsg: errMsg,
				}

				// Stop state check of this policy if we encounter an error.
				if plcy.hasError {
					break
				}
			}
		}
	}
}

func (c *configTask) enforceState(ctx context.Context) {
	// Run enforcement (for resources that require it).
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": a.GetConfigAssignment()})
		asgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, p := range a.GetPolicies() {
			ctx = clog.WithLabels(ctx, map[string]string{"policy_id": p.GetId()})
			plcy := asgnmnt.policies[p.GetId()]
			// Skip enforcement if this policy already has an error from a previous step.
			if plcy.hasError {
				clog.Debugf(ctx, "Policy has error, skipping enforcement.")
				continue
			}
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			for i, r := range p.GetResources() {
				ctx = clog.WithLabels(ctx, map[string]string{"resource_id": r.GetId()})
				res := plcy.resources[r.GetId()]
				// Only enforce resources that need it.
				if !res.enforcementNeeded {
					clog.Debugf(ctx, "No enforcement required.")
					continue
				}
				c.postCheckRequired = true
				res.enforcementNeeded = false

				result := pResult.GetResourceResults().GetResults()[i]
				outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState_SUCCESS
				errMsg := ""
				err := res.runEnforcement(ctx)
				if err != nil {
					outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState_ERROR
					plcy.hasError = true
					errMsg = fmt.Sprintf("Error running enforcement: %v", err)
					clog.Errorf(ctx, errMsg)
				}

				result.GetExecutionSteps()[enforcementStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
					Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_EnforceDesiredState{
						EnforceDesiredState: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState{
							Outcome: outcome,
						},
					},
					ErrMsg: errMsg,
				}

				// Stop enforcement of this policy if we encounter an error.
				if plcy.hasError {
					break
				}
			}
		}
	}
}

func (c *configTask) postCheckState(ctx context.Context) {
	// Actually run post check state (for policies that do not have a previous error).
	// No prepopulate run for post check as we will always check every resource.
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": a.GetConfigAssignment()})
		asgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, p := range a.GetPolicies() {
			ctx = clog.WithLabels(ctx, map[string]string{"policy_id": p.GetId()})
			plcy := asgnmnt.policies[p.GetId()]
			// Skip post check if this policy already has an error from a previous step.
			if plcy.hasError {
				clog.Debugf(ctx, "Policy has error, skipping post check.")
				continue
			}
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			for i, r := range p.GetResources() {
				ctx = clog.WithLabels(ctx, map[string]string{"resource_id": r.GetId()})
				res := plcy.resources[r.GetId()]

				outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement_IN_DESIRED_STATE
				errMsg := ""
				err := res.checkState(ctx)
				if err != nil {
					outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement_ERROR
					plcy.hasError = true
					errMsg = fmt.Sprintf("Error running desired state check: %v", err)
					clog.Errorf(ctx, errMsg)
				}

				if res.enforcementNeeded {
					outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement_NOT_IN_DESIRED_STATE
				}

				result := pResult.GetResourceResults().GetResults()[i]
				result.GetExecutionSteps()[checkStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
					Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_CheckDesiredStatePostEnforcement{
						CheckDesiredStatePostEnforcement: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement{
							Outcome: outcome,
						},
					},
					ErrMsg: errMsg,
				}
			}
		}
	}
}

func (c *configTask) generateBaseResults() {
	c.results.Results = make([]*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult, len(c.Task.GetConfig().GetConfigAssignments()))
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		aResult := &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
			ConfigAssignment: a.GetConfigAssignment(),
			Result: &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult_OsPolicyResults{
				OsPolicyResults: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResults{
					Results: make([]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult, len(a.GetPolicies())),
				},
			},
		}
		c.results.GetResults()[i] = aResult
		for i, p := range a.GetPolicies() {
			pResult := &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
				Id: p.GetId(),
				Result: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResults{
					ResourceResults: &agentendpointpb.ApplyConfigTaskOutput_ResourceResults{
						Results: make([]*agentendpointpb.ApplyConfigTaskOutput_ResourceResult, len(p.GetResources())),
					},
				},
			}
			aResult.GetOsPolicyResults().GetResults()[i] = pResult
			for i, r := range p.GetResources() {
				pResult.GetResourceResults().GetResults()[i] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult{
					Id:             r.GetId(),
					ExecutionSteps: make([]*agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep, numExecutionSteps),
				}
			}
		}
	}
}

func (c *configTask) run(ctx context.Context) error {
	clog.Infof(ctx, "Beginning apply config task.")
	c.StartedAt = time.Now()
	rcsErrMsg := "Error reporting continuing state"
	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_STARTED); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}

	if len(c.Task.GetConfig().GetConfigAssignments()) == 0 {
		clog.Infof(ctx, "No config assignments to apply.")
		return c.reportCompletedState(ctx, "", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED)
	}

	// We need to generate base results first thing, each execution step
	// just adds on.
	c.generateBaseResults()

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_VALIDATING); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}
	c.validation(ctx)

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_CHECKING_DESIRED_STATE); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}
	c.checkState(ctx)

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_ENFORCING_DESIRED_STATE); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}
	c.enforceState(ctx)

	if c.postCheckRequired {
		if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_CHECKING_DESIRED_STATE_POST_ENFORCEMENT); err != nil {
			return c.handleErrorState(ctx, rcsErrMsg, err)
		}
		c.postCheckState(ctx)
	}

	return c.reportCompletedState(ctx, "", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED)
}

// RunApplyConfig runs an apply config task.
func (c *Client) RunApplyConfig(ctx context.Context, task *agentendpointpb.Task) error {
	ctx = clog.WithLabels(ctx, task.GetServiceLabels())
	e := &configTask{
		TaskID:  task.GetTaskId(),
		client:  c,
		Task:    &applyConfigTask{task.GetApplyConfigTask()},
		results: &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{},
	}

	return e.run(ctx)
}
