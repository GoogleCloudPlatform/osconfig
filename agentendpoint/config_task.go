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
	"github.com/GoogleCloudPlatform/osconfig/config"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

const (
	numExecutionSteps              = 4
	validationStepIndex            = 0
	checkDesiredStateStepIndex     = 1
	enforcementStepIndex           = 2
	postCheckDesiredStateStepIndex = 3
)

var newResource = func(r *agentendpointpb.ApplyConfigTask_OSPolicy_Resource) resourceIface {
	return resourceIface(&config.OSPolicyResource{ApplyConfigTask_OSPolicy_Resource: r})
}

type configTask struct {
	client *Client

	lastProgressState map[agentendpointpb.ApplyConfigTaskProgress_State]time.Time

	TaskID    string
	Task      *applyConfigTask
	StartedAt time.Time `json:",omitempty"`

	// ApplyConfigTaskOutput result
	results           []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult
	postCheckRequired bool
	policies          map[string]*policy
}

type applyConfigTask struct {
	*agentendpointpb.ApplyConfigTask
}

type policy struct {
	resources map[string]resourceIface
	hasError  bool
}

type resourceIface interface {
	Validate(context.Context) error
	CheckState(context.Context) error
	EnforceState(context.Context) error
	Cleanup(context.Context) error
	InDesiredState() bool
	ManagedResources() *config.ManagedResources
}

func (c *configTask) reportCompletedState(ctx context.Context, errMsg string, state agentendpointpb.ApplyConfigTaskOutput_State) error {
	req := &agentendpointpb.ReportTaskCompleteRequest{
		TaskId:       c.TaskID,
		TaskType:     agentendpointpb.TaskType_APPLY_CONFIG_TASK,
		ErrorMessage: errMsg,
		Output: &agentendpointpb.ReportTaskCompleteRequest_ApplyConfigTaskOutput{
			ApplyConfigTaskOutput: &agentendpointpb.ApplyConfigTaskOutput{State: state, OsPolicyResults: c.results},
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
	st, ok := c.lastProgressState[configState]
	if ok && st.After(time.Now().Add(sameStateTimeWindow)) {
		// Don't resend the same state more than once every 5s.
		return nil
	}

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
	if c.lastProgressState == nil {
		c.lastProgressState = make(map[agentendpointpb.ApplyConfigTaskProgress_State]time.Time)
	}
	c.lastProgressState[configState] = time.Now()
	return nil
}

// detectPolicyConflicts checks for managed resource conflicts between a proposed
// OSPolicyResource and all other OSPolcyResources up to this point, adding to the
// current set of ManagedResources.
func detectPolicyConflicts(proposed, current *config.ManagedResources) error {
	// TODO: implement
	return nil
}

func validateConfigResource(ctx context.Context, plcy *policy, policyMR *config.ManagedResources, rResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_OSPolicy_Resource) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	plcy.resources[configResource.GetId()] = newResource(configResource)
	resource := plcy.resources[configResource.GetId()]

	outcome := agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_Validation_OK
	errMsg := ""
	if err := resource.Validate(ctx); err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_Validation_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error validating resource: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	// Detect any resource conflicts within this policy.
	if err := detectPolicyConflicts(resource.ManagedResources(), policyMR); err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_Validation_CONFLICT
		plcy.hasError = true
		errMsg = fmt.Sprintf("Resource conflict in policy: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	rResult.GetExecutionSteps()[validationStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_Validation{
			Validation: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_Validation{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
}

func (c *configTask) validation(ctx context.Context) {
	// This is all the managed resources by policy.
	globalManagedResources := map[string]*config.ManagedResources{}

	// Validate each resouce and populate results and internal assignment state.
	c.policies = map[string]*policy{}
	for i, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": osPolicy.GetSourceOsPolicyAssignment(), "policy_id": osPolicy.GetId()})
		pResult := c.results[i]
		plcy := &policy{resources: map[string]resourceIface{}}
		c.policies[osPolicy.GetId()] = plcy
		var policyMR *config.ManagedResources

		for i, configResource := range osPolicy.GetResources() {
			rResult := pResult.GetResourceResults()[i]
			validateConfigResource(ctx, plcy, policyMR, rResult, configResource)
		}

		// We only care about conflict detection across policies that are in enforcement mode.
		if osPolicy.GetMode() == agentendpointpb.ApplyConfigTask_OSPolicy_ENFORCEMENT {
			globalManagedResources[osPolicy.GetId()] = policyMR
		}
	}

	// TODO: check for global resource conflicts.

}

func checkConfigResourceState(ctx context.Context, plcy *policy, rResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_OSPolicy_Resource) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	res := plcy.resources[configResource.GetId()]

	outcome := agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck_IN_DESIRED_STATE
	errMsg := ""
	err := res.CheckState(ctx)
	if err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error running desired state check: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	if !res.InDesiredState() {
		outcome = agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck_NOT_IN_DESIRED_STATE
	}

	rResult.GetExecutionSteps()[checkDesiredStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_DesiredStateCheck{
			DesiredStateCheck: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
}

func (c *configTask) checkState(ctx context.Context) {
	// First populate check state results (for policies that do not have validation errors).
	for i, osPolicy := range c.Task.GetOsPolicies() {
		plcy := c.policies[osPolicy.GetId()]
		// Skip state check if this policy already has an error from a previous step.
		if plcy.hasError {
			continue
		}
		pResult := c.results[i]
		for i := range osPolicy.GetResources() {
			result := pResult.GetResourceResults()[i]
			result.GetExecutionSteps()[checkDesiredStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
				Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_DesiredStateCheck{
					DesiredStateCheck: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck{
						Outcome: agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck_SKIPPED,
					},
				},
			}
		}
	}

	// Actually run check state.
	for i, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": osPolicy.GetSourceOsPolicyAssignment(), "policy_id": osPolicy.GetId()})
		plcy := c.policies[osPolicy.GetId()]

		// Skip state check if this policy already has an error from a previous step.
		if plcy.hasError {
			clog.Debugf(ctx, "Policy has error, skipping state check.")
			continue
		}

		pResult := c.results[i]
		for i, configResource := range osPolicy.GetResources() {
			rResult := pResult.GetResourceResults()[i]
			checkConfigResourceState(ctx, plcy, rResult, configResource)

			// Stop state check of this policy if we encounter an error.
			if plcy.hasError {
				clog.Debugf(ctx, "Policy has error, stopping state check.")
				break
			}
		}
	}
}

func enforceConfigResourceState(ctx context.Context, plcy *policy, rResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_OSPolicy_Resource) bool {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	res := plcy.resources[configResource.GetId()]
	// Only enforce resources that need it.
	if res.InDesiredState() {
		clog.Debugf(ctx, "No enforcement required.")
		return false
	}

	outcome := agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateEnforcement_SUCCESS
	errMsg := ""
	err := res.EnforceState(ctx)
	if err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateEnforcement_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error running enforcement: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	rResult.GetExecutionSteps()[enforcementStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_DesiredStateEnforcement{
			DesiredStateEnforcement: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateEnforcement{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
	return true
}

func (c *configTask) enforceState(ctx context.Context) {
	// Run enforcement (for resources that require it).
	for i, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": osPolicy.GetSourceOsPolicyAssignment(), "policy_id": osPolicy.GetId()})
		plcy := c.policies[osPolicy.GetId()]

		// Skip state check if this policy already has an error from a previous step.
		if plcy.hasError {
			clog.Debugf(ctx, "Policy has error, skipping enforcement.")
			continue
		}

		pResult := c.results[i]
		for i, configResource := range osPolicy.GetResources() {
			rResult := pResult.GetResourceResults()[i]
			if enforceConfigResourceState(ctx, plcy, rResult, configResource) {
				// On aany change we trigger post check.
				c.postCheckRequired = true
			}

			// Stop enforcement of this policy if we encounter an error.
			if plcy.hasError {
				clog.Debugf(ctx, "Policy has error, stopping enforcement.")
				break
			}
		}
	}
}

func postCheckConfigResourceState(ctx context.Context, plcy *policy, rResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_OSPolicy_Resource) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	res := plcy.resources[configResource.GetId()]

	outcome := agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheckPostEnforcement_IN_DESIRED_STATE
	errMsg := ""
	err := res.CheckState(ctx)
	if err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheckPostEnforcement_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error running desired state check: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	if !res.InDesiredState() {
		outcome = agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheckPostEnforcement_NOT_IN_DESIRED_STATE
	}

	rResult.GetExecutionSteps()[postCheckDesiredStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_DesiredStateCheckPostEnforcement{
			DesiredStateCheckPostEnforcement: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheckPostEnforcement{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
}

func (c *configTask) postCheckState(ctx context.Context) {
	// Actually run post check state (for policies that do not have a previous error).
	// No prepopulate run for post check as we will always check every resource.
	for i, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": osPolicy.GetSourceOsPolicyAssignment(), "policy_id": osPolicy.GetId()})
		plcy := c.policies[osPolicy.GetId()]

		// Skip state check if this policy already has an error from a previous step.
		if plcy.hasError {
			clog.Debugf(ctx, "Policy has error, skipping post check.")
			continue
		}

		pResult := c.results[i]
		for i, configResource := range osPolicy.GetResources() {
			rResult := pResult.GetResourceResults()[i]
			postCheckConfigResourceState(ctx, plcy, rResult, configResource)
		}
	}
}

func (c *configTask) generateBaseResults() {
	c.results = make([]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult, len(c.Task.GetOsPolicies()))
	for i, p := range c.Task.GetOsPolicies() {
		pResult := &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
			Id:                       p.GetId(),
			SourceOsPolicyAssignment: p.GetSourceOsPolicyAssignment(),
			ResourceResults:          make([]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult, len(p.GetResources())),
		}
		c.results[i] = pResult
		for i, r := range p.GetResources() {
			pResult.GetResourceResults()[i] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult{
				Id:             r.GetId(),
				ExecutionSteps: make([]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep, numExecutionSteps),
			}
		}
	}
}

func (c *configTask) cleanup(ctx context.Context) {
	for _, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": osPolicy.GetSourceOsPolicyAssignment(), "policy_id": osPolicy.GetId()})
		plcy := c.policies[osPolicy.GetId()]
		for _, configResource := range osPolicy.GetResources() {
			ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
			res := plcy.resources[configResource.GetId()]
			if err := res.Cleanup(ctx); err != nil {
				clog.Warningf(ctx, "Error running resource cleanup:%v", err)
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

	if len(c.Task.GetOsPolicies()) == 0 {
		clog.Infof(ctx, "No OSPolicies to apply.")
		return c.reportCompletedState(ctx, "", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED)
	}

	// We need to generate base results first thing, each execution step
	// just adds on.
	c.generateBaseResults()
	c.validation(ctx)
	defer c.cleanup(ctx)
	c.checkState(ctx)

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_APPLYING_CONFIG); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}
	c.enforceState(ctx)

	if c.postCheckRequired {
		c.postCheckState(ctx)
	}

	return c.reportCompletedState(ctx, "", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED)
}

// RunApplyConfig runs an apply config task.
func (c *Client) RunApplyConfig(ctx context.Context, task *agentendpointpb.Task) error {
	ctx = clog.WithLabels(ctx, task.GetServiceLabels())
	e := &configTask{
		TaskID: task.GetTaskId(),
		client: c,
		Task:   &applyConfigTask{task.GetApplyConfigTask()},
	}

	return e.run(ctx)
}
