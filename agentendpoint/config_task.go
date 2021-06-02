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
	"github.com/GoogleCloudPlatform/osconfig/pretty"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

// This is the maximum size in bytes of the OSPolicyResourceConfigStep ErrorMessage field.
// Any message longer than this will be truncated to at max this length.
const maxErrorMessage = 512

var newResource = func(r *agentendpointpb.OSPolicy_Resource) *resource {
	return &resource{resourceIface: resourceIface(&config.OSPolicyResource{OSPolicy_Resource: r})}
}

type configTask struct {
	StartedAt         time.Time `json:",omitempty"`
	client            *Client
	lastProgressState map[agentendpointpb.ApplyConfigTaskProgress_State]time.Time
	Task              *applyConfigTask
	policies          map[string]*policy
	TaskID            string
	results           []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult
}

type applyConfigTask struct {
	*agentendpointpb.ApplyConfigTask
}

type policy struct {
	resources map[string]*resource
}

type resource struct {
	resourceIface
	needsPostCheck       bool
	validateOrCheckError bool
}

type resourceIface interface {
	Validate(context.Context) error
	CheckState(context.Context) error
	EnforceState(context.Context) error
	PopulateOutput(*agentendpointpb.OSPolicyResourceCompliance) error
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

func truncateMessage(msg string, size int) string {
	cut := size / 2
	if len(msg) > size {
		return msg[:size-(cut+3)] + "..." + msg[len(msg)-cut:]
	}
	return msg
}

func validateConfigResource(ctx context.Context, res *resource, policyMR *config.ManagedResources, rCompliance *agentendpointpb.OSPolicyResourceCompliance, configResource *agentendpointpb.OSPolicy_Resource) (hasError bool) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	clog.Debugf(ctx, "Running step 'validate' on resource %q.", configResource.GetId())

	var errMessage string
	outcome := agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
	if err := res.Validate(ctx); err != nil {
		outcome = agentendpointpb.OSPolicyResourceConfigStep_FAILED
		hasError = true
		errMessage = fmt.Sprintf("Error validating resource: %v", err)
		clog.Errorf(ctx, errMessage)
	} else {
		// Detect any resource conflicts within this policy.
		if err := detectPolicyConflicts(res.ManagedResources(), policyMR); err != nil {
			outcome = agentendpointpb.OSPolicyResourceConfigStep_FAILED
			hasError = true
			errMessage = fmt.Sprintf("Resource conflict in policy: %v", err)
			clog.Errorf(ctx, errMessage)
		}
	}

	rCompliance.ConfigSteps = append(rCompliance.GetConfigSteps(), &agentendpointpb.OSPolicyResourceConfigStep{
		Type:         agentendpointpb.OSPolicyResourceConfigStep_VALIDATION,
		Outcome:      outcome,
		ErrorMessage: errMessage,
	})
	rCompliance.State = agentendpointpb.OSPolicyComplianceState_UNKNOWN
	return hasError
}

func checkConfigResourceState(ctx context.Context, res *resource, rCompliance *agentendpointpb.OSPolicyResourceCompliance, configResource *agentendpointpb.OSPolicy_Resource) (hasError bool) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	clog.Debugf(ctx, "Running step 'check state' on resource %q.", configResource.GetId())

	var errMessage string
	outcome := agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
	state := agentendpointpb.OSPolicyComplianceState_UNKNOWN
	err := res.CheckState(ctx)
	if err != nil {
		outcome = agentendpointpb.OSPolicyResourceConfigStep_FAILED
		hasError = true
		errMessage = fmt.Sprintf("Error running desired state check: %v", err)
		clog.Errorf(ctx, errMessage)
	} else if res.InDesiredState() {
		state = agentendpointpb.OSPolicyComplianceState_COMPLIANT
	} else {
		state = agentendpointpb.OSPolicyComplianceState_NON_COMPLIANT
	}

	rCompliance.ConfigSteps = append(rCompliance.GetConfigSteps(), &agentendpointpb.OSPolicyResourceConfigStep{
		Type:         agentendpointpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
		Outcome:      outcome,
		ErrorMessage: errMessage,
	})
	clog.Debugf(ctx, "Resource %q state is %s.", configResource.GetId(), state)
	rCompliance.State = state
	return hasError
}

func enforceConfigResourceState(ctx context.Context, res *resource, rCompliance *agentendpointpb.OSPolicyResourceCompliance, configResource *agentendpointpb.OSPolicy_Resource) (enforcementActionTaken, hasError bool) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	clog.Debugf(ctx, "Running step 'enforce state' on resource %q.", configResource.GetId())
	// Only enforce resources that need it.
	if res.InDesiredState() {
		clog.Debugf(ctx, "No enforcement required for %q.", configResource.GetId())
		return false, false
	}

	var errMessage string
	outcome := agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
	err := res.EnforceState(ctx)
	if err != nil {
		outcome = agentendpointpb.OSPolicyResourceConfigStep_FAILED
		hasError = true
		errMessage = fmt.Sprintf("Error running enforcement: %v", err)
		clog.Errorf(ctx, errMessage)
	}

	rCompliance.ConfigSteps = append(rCompliance.GetConfigSteps(), &agentendpointpb.OSPolicyResourceConfigStep{
		Type:         agentendpointpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
		Outcome:      outcome,
		ErrorMessage: errMessage,
	})
	// Resource is always in an unknown state after enforcement is run.
	// A COMPLIANT state will only happen after a post check.
	rCompliance.State = agentendpointpb.OSPolicyComplianceState_UNKNOWN
	return true, hasError
}

func postCheckConfigResourceState(ctx context.Context, res *resource, rCompliance *agentendpointpb.OSPolicyResourceCompliance, configResource *agentendpointpb.OSPolicy_Resource) {
	ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
	clog.Debugf(ctx, "Running step 'check state post enforcement' on resource %q.", configResource.GetId())
	if !res.needsPostCheck {
		clog.Debugf(ctx, "No post check required for %q.", configResource.GetId())
		return
	}

	var errMessage string
	outcome := agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
	state := agentendpointpb.OSPolicyComplianceState_UNKNOWN
	err := res.CheckState(ctx)
	if err != nil {
		outcome = agentendpointpb.OSPolicyResourceConfigStep_FAILED
		errMessage = fmt.Sprintf("Error running post config desired state check: %v", err)
		clog.Errorf(ctx, errMessage)
	} else if res.InDesiredState() {
		state = agentendpointpb.OSPolicyComplianceState_COMPLIANT
	} else {
		state = agentendpointpb.OSPolicyComplianceState_NON_COMPLIANT
	}

	rCompliance.ConfigSteps = append(rCompliance.GetConfigSteps(), &agentendpointpb.OSPolicyResourceConfigStep{
		Type:         agentendpointpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
		Outcome:      outcome,
		ErrorMessage: errMessage,
	})
	clog.Debugf(ctx, "Resource %q state is %s.", configResource.GetId(), state)
	rCompliance.State = state
}

func (c *configTask) postCheckState(ctx context.Context) {
	// Actually run post check state (for policies that do not have a previous error).
	// No prepopulate run for post check as we will always check every resource.
	for i, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": osPolicy.GetOsPolicyAssignment(), "policy_id": osPolicy.GetId()})
		plcy, ok := c.policies[osPolicy.GetId()]
		// This should not happen in the normal code flow since we only run postCheckState after
		// all policies have been evaluated.
		if !ok {
			clog.Errorf(ctx, "Unexpected Error: policy entry for %q is empty.", osPolicy.GetId())
			continue
		}
		pResult := c.results[i]
		for i, configResource := range osPolicy.GetResources() {
			res, ok := plcy.resources[configResource.GetId()]
			// This should only happen if there was a previous resource with a validate or check state error.
			if !ok || res == nil {
				continue
			}
			rCompliance := pResult.GetOsPolicyResourceCompliances()[i]
			postCheckConfigResourceState(ctx, res, rCompliance, configResource)
			clog.Infof(ctx, "Policy %q resource %q state: %s", osPolicy.GetId(), configResource.GetId(), rCompliance.GetState())
		}
	}
	return
}

func (c *configTask) generateBaseResults() {
	c.results = make([]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult, len(c.Task.GetOsPolicies()))
	for i, p := range c.Task.GetOsPolicies() {
		pResult := &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
			OsPolicyId:                  p.GetId(),
			OsPolicyAssignment:          p.GetOsPolicyAssignment(),
			OsPolicyResourceCompliances: make([]*agentendpointpb.OSPolicyResourceCompliance, len(p.GetResources())),
		}
		c.results[i] = pResult
		for i, r := range p.GetResources() {
			pResult.GetOsPolicyResourceCompliances()[i] = &agentendpointpb.OSPolicyResourceCompliance{
				OsPolicyResourceId: r.GetId(),
			}
		}
	}
}

func (c *configTask) cleanup(ctx context.Context) {
	for _, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"config_assignment": osPolicy.GetOsPolicyAssignment(), "policy_id": osPolicy.GetId()})
		plcy := c.policies[osPolicy.GetId()]
		for _, configResource := range osPolicy.GetResources() {
			ctx = clog.WithLabels(ctx, map[string]string{"resource_id": configResource.GetId()})
			res, ok := plcy.resources[configResource.GetId()]
			if !ok || res == nil {
				continue
			}
			if err := res.Cleanup(ctx); err != nil {
				clog.Warningf(ctx, "Error running resource cleanup:%v", err)
			}
		}
	}
}

func (c *configTask) run(ctx context.Context) error {
	clog.Infof(ctx, "Beginning ApplyConfigTask.")
	clog.Debugf(ctx, "ApplyConfigTask:\n%s", pretty.Format(c.Task.ApplyConfigTask))
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
	defer c.cleanup(ctx)

	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_APPLYING_CONFIG); err != nil {
		return c.handleErrorState(ctx, rcsErrMsg, err)
	}

	c.policies = map[string]*policy{}
	for i, osPolicy := range c.Task.GetOsPolicies() {
		ctx = clog.WithLabels(ctx, map[string]string{"os_policy_assignment": osPolicy.GetOsPolicyAssignment(), "os_policy_id": osPolicy.GetId()})
		clog.Infof(ctx, "Executing policy %q", osPolicy.GetId())

		pResult := c.results[i]
		plcy := &policy{resources: map[string]*resource{}}
		c.policies[osPolicy.GetId()] = plcy
		var policyMR *config.ManagedResources

		var validateOnly bool
		if osPolicy.GetMode() == agentendpointpb.OSPolicy_VALIDATION {
			clog.Debugf(ctx, "Policy running in VALIDATION mode, not running enforcement action for any resources.")
			validateOnly = true
		}

		for i, configResource := range osPolicy.GetResources() {
			rCompliance := pResult.GetOsPolicyResourceCompliances()[i]
			plcy.resources[configResource.GetId()] = newResource(configResource)
			res := plcy.resources[configResource.GetId()]
			if hasError := validateConfigResource(ctx, res, policyMR, rCompliance, configResource); hasError {
				res.validateOrCheckError = true
				break
			}
			if hasError := checkConfigResourceState(ctx, res, rCompliance, configResource); hasError {
				res.validateOrCheckError = true
				break
			}

			// Skip enforcement actions in VALIDATION mode.
			if validateOnly {
				continue
			}

			// Only errors in validate and check state constitute a serious error,
			// for enforce if any action is taken we still want to run post check.
			// We do however stop further execution of this polcy on enforce error.
			enforcementActionTaken, hasError := enforceConfigResourceState(ctx, res, rCompliance, configResource)
			if enforcementActionTaken {
				// On any change we trigger post check for all previous resouces,
				// even if there was an error.
				c.markPostCheckRequired()
			}
			// Still record output even if there was an error during enforcement.
			res.PopulateOutput(rCompliance)
			// Errors from enforcement are not classified as "serious" becasue we want post check to run for this resource.
			if hasError {
				break
			}
		}
	}

	// Run any post checks that we need to.
	c.postCheckState(ctx)

	if err := c.reportCompletedState(ctx, "", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED); err != nil {
		return err
	}
	clog.Infof(ctx, "Successfully completed ApplyConfigTask")
	return nil
}

// Mark all resources that have already completed as "needs post check".
func (c *configTask) markPostCheckRequired() {
	for _, osPolicy := range c.Task.GetOsPolicies() {
		plcy, ok := c.policies[osPolicy.GetId()]
		// This policy entry may not have been created yet by the loop in run().
		if !ok {
			continue
		}
		for _, configResource := range osPolicy.GetResources() {
			res, ok := plcy.resources[configResource.GetId()]
			// This resource entry may not have been created yet is this polciy is in mid-run.
			// We take no actions for resources that had a failure with validation or precheck.
			if !ok || res == nil || res.validateOrCheckError {
				continue
			}
			res.needsPostCheck = true
		}
	}
}

// RunApplyConfig runs an ApplyConfigTask.
func (c *Client) RunApplyConfig(ctx context.Context, task *agentendpointpb.Task) error {
	ctx = clog.WithLabels(ctx, task.GetServiceLabels())
	e := &configTask{
		TaskID: task.GetTaskId(),
		client: c,
		Task:   &applyConfigTask{task.GetApplyConfigTask()},
	}

	return e.run(ctx)
}
