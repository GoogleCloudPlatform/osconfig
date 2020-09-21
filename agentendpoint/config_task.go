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

var newResource = func(r *agentendpointpb.ApplyConfigTask_Config_Resource) resourceIface {
	return resourceIface(&config.OSPolicyResource{ApplyConfigTask_Config_Resource: r})
}

type configTask struct {
	client *Client

	lastProgressState map[agentendpointpb.ApplyConfigTaskProgress_State]time.Time

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
	resources map[string]resourceIface
	hasError  bool
}

type resourceIface interface {
	Validate(context.Context) error
	CheckState(context.Context) error
	EnforceState(context.Context) error
	InDesiredState() bool
	ManagedResources() *config.ManagedResources
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

func validateConfigResource(ctx context.Context, plcy *policy, policyMR *config.ManagedResources, rResult *agentendpointpb.ApplyConfigTaskOutput_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_Config_Resource) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	plcy.resources[configResource.GetId()] = newResource(configResource)
	resource := plcy.resources[configResource.GetId()]

	outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation_OK
	errMsg := ""
	if err := resource.Validate(ctx); err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation_RESOURCE_PAYLOAD_CONVERSION_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error validating resource: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	// Detect any resource conflicts within this policy.
	if err := detectPolicyConflicts(resource.ManagedResources(), policyMR); err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation_CONFLICT
		plcy.hasError = true
		errMsg = fmt.Sprintf("Resource conflict in policy: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	rResult.GetExecutionSteps()[validationStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_Validation{
			Validation: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
}

func validateOSPolicy(ctx context.Context, assgnmnt *assignment, osPolicy *agentendpointpb.ApplyConfigTask_Config_OSPolicy, pResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult) (policyMR *config.ManagedResources) {
	ctx = clog.WithLabels(ctx, map[string]string{"policy_id": osPolicy.GetId()})
	plcy := &policy{resources: map[string]resourceIface{}}
	assgnmnt.policies[osPolicy.GetId()] = plcy

	for i, configResource := range osPolicy.GetResources() {
		rResult := pResult.GetResourceResults().GetResults()[i]
		validateConfigResource(ctx, plcy, policyMR, rResult, configResource)
	}
	return
}

func (c *configTask) validation(ctx context.Context) {
	// This is all the managed resources by assignment and policy.
	globalManagedResources := map[string]map[string]*config.ManagedResources{}

	// Validate each resouce and populate results and internal assignment state.
	c.assignments = map[string]*assignment{}
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": a.GetConfigAssignment()})
		assgnmnt := &assignment{policies: map[string]*policy{}}
		c.assignments[a.GetConfigAssignment()] = assgnmnt
		aResult := c.results.GetResults()[i]
		for i, osPolicy := range a.GetPolicies() {
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			policyMR := validateOSPolicy(ctx, assgnmnt, osPolicy, pResult)

			// We only care about conflict detection across policies that are in enforcement mode.
			if osPolicy.GetMode() == agentendpointpb.ApplyConfigTask_Config_OSPolicy_ENFORCEMENT {
				if _, ok := globalManagedResources[a.GetConfigAssignment()]; !ok {
					globalManagedResources[a.GetConfigAssignment()] = map[string]*config.ManagedResources{osPolicy.GetId(): policyMR}
				} else {
					globalManagedResources[a.GetConfigAssignment()][osPolicy.GetId()] = policyMR
				}
			}
		}
	}

	// TODO: check for global resource conflicts.

}

func checkConfigResourceState(ctx context.Context, plcy *policy, rResult *agentendpointpb.ApplyConfigTaskOutput_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_Config_Resource) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	res := plcy.resources[configResource.GetId()]

	outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_IN_DESIRED_STATE
	errMsg := ""
	err := res.CheckState(ctx)
	if err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error running desired state check: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	if !res.InDesiredState() {
		outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_NOT_IN_DESIRED_STATE
	}

	rResult.GetExecutionSteps()[checkDesiredStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_CheckDesiredState{
			CheckDesiredState: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
}

func checkOSPolicyState(ctx context.Context, assgnmnt *assignment, osPolicy *agentendpointpb.ApplyConfigTask_Config_OSPolicy, pResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult) {
	ctx = clog.WithLabels(ctx, map[string]string{"policy_id": osPolicy.GetId()})
	plcy := assgnmnt.policies[osPolicy.GetId()]
	// Skip state check if this policy already has an error from a previous step.
	if plcy.hasError {
		clog.Debugf(ctx, "Policy has error, skipping state check.")
		return
	}
	for i, configResource := range osPolicy.GetResources() {
		rResult := pResult.GetResourceResults().GetResults()[i]
		checkConfigResourceState(ctx, plcy, rResult, configResource)

		// Stop state check of this policy if we encounter an error.
		if plcy.hasError {
			break
		}
	}
}

func (c *configTask) checkState(ctx context.Context) {
	// First populate check state results (for policies that do not have validation errors).
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		assgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, p := range a.GetPolicies() {
			plcy := assgnmnt.policies[p.GetId()]
			// Skip state check if this policy already has an error from a previous step.
			if plcy.hasError {
				continue
			}
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			for i := range p.GetResources() {
				result := pResult.GetResourceResults().GetResults()[i]
				result.GetExecutionSteps()[checkDesiredStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
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
		assgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, osPolicy := range a.GetPolicies() {
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			checkOSPolicyState(ctx, assgnmnt, osPolicy, pResult)
		}
	}
}

func enforceConfigResourceState(ctx context.Context, plcy *policy, rResult *agentendpointpb.ApplyConfigTaskOutput_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_Config_Resource) bool {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	res := plcy.resources[configResource.GetId()]
	// Only enforce resources that need it.
	if res.InDesiredState() {
		clog.Debugf(ctx, "No enforcement required.")
		return false
	}

	outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState_SUCCESS
	errMsg := ""
	err := res.EnforceState(ctx)
	if err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error running enforcement: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	rResult.GetExecutionSteps()[enforcementStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_EnforceDesiredState{
			EnforceDesiredState: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
	return true
}

func enforceOSPolicyState(ctx context.Context, assgnmnt *assignment, osPolicy *agentendpointpb.ApplyConfigTask_Config_OSPolicy, pResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult) (actionTaken bool) {
	ctx = clog.WithLabels(ctx, map[string]string{"policy_id": osPolicy.GetId()})
	plcy := assgnmnt.policies[osPolicy.GetId()]
	// Skip enforcement if this policy already has an error from a previous step.
	if plcy.hasError {
		clog.Debugf(ctx, "Policy has error, skipping enforcement.")
		return
	}

	for i, configResource := range osPolicy.GetResources() {
		rResult := pResult.GetResourceResults().GetResults()[i]
		if enforceConfigResourceState(ctx, plcy, rResult, configResource) {
			actionTaken = true
		}

		// Stop enforcement of this policy if we encounter an error.
		if plcy.hasError {
			break
		}
	}
	return
}

func (c *configTask) enforceState(ctx context.Context) {
	// Run enforcement (for resources that require it).
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": a.GetConfigAssignment()})
		assgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, osPolicy := range a.GetPolicies() {
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			actionTaken := enforceOSPolicyState(ctx, assgnmnt, osPolicy, pResult)
			if actionTaken {
				c.postCheckRequired = true
			}
		}
	}
}

func postCheckConfigResourceState(ctx context.Context, plcy *policy, rResult *agentendpointpb.ApplyConfigTaskOutput_ResourceResult, configResource *agentendpointpb.ApplyConfigTask_Config_Resource) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	res := plcy.resources[configResource.GetId()]

	outcome := agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement_IN_DESIRED_STATE
	errMsg := ""
	err := res.CheckState(ctx)
	if err != nil {
		outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement_ERROR
		plcy.hasError = true
		errMsg = fmt.Sprintf("Error running desired state check: %v", err)
		clog.Errorf(ctx, errMsg)
	}

	if !res.InDesiredState() {
		outcome = agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement_NOT_IN_DESIRED_STATE
	}

	rResult.GetExecutionSteps()[postCheckDesiredStateStepIndex] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
		Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_CheckDesiredStatePostEnforcement{
			CheckDesiredStatePostEnforcement: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement{
				Outcome: outcome,
			},
		},
		ErrMsg: errMsg,
	}
}

func postCheckOSPolicyState(ctx context.Context, assgnmnt *assignment, osPolicy *agentendpointpb.ApplyConfigTask_Config_OSPolicy, pResult *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult) {
	ctx = clog.WithLabels(ctx, map[string]string{"policy_id": osPolicy.GetId()})
	plcy := assgnmnt.policies[osPolicy.GetId()]
	// Skip post check if this policy already has an error from a previous step.
	if plcy.hasError {
		clog.Debugf(ctx, "Policy has error, skipping post check.")
		return
	}

	for i, configResource := range osPolicy.GetResources() {
		rResult := pResult.GetResourceResults().GetResults()[i]
		postCheckConfigResourceState(ctx, plcy, rResult, configResource)
	}
}

func (c *configTask) postCheckState(ctx context.Context) {
	// Actually run post check state (for policies that do not have a previous error).
	// No prepopulate run for post check as we will always check every resource.
	for i, a := range c.Task.GetConfig().GetConfigAssignments() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": a.GetConfigAssignment()})
		assgnmnt := c.assignments[a.GetConfigAssignment()]
		aResult := c.results.GetResults()[i]
		for i, osPolicy := range a.GetPolicies() {
			pResult := aResult.GetOsPolicyResults().GetResults()[i]
			postCheckOSPolicyState(ctx, assgnmnt, osPolicy, pResult)
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

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_APPLYING_CONFIG); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}
	c.validation(ctx)

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_APPLYING_CONFIG); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}
	c.checkState(ctx)

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_APPLYING_CONFIG); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}
	c.enforceState(ctx)

	if c.postCheckRequired {
		if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_APPLYING_CONFIG); err != nil {
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
