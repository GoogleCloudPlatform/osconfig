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
	"errors"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

var errTest = errors.New("this is a test error")

type testResource struct {
	inDesiredState bool
	steps          int
}

func (r *testResource) InDesiredState() bool {
	return r.inDesiredState
}

func (r *testResource) Cleanup(ctx context.Context) error {
	return nil
}

func (r *testResource) Validate(ctx context.Context) error {
	if r.steps == 0 {
		return errTest
	}
	return nil
}

func (r *testResource) CheckState(ctx context.Context) error {
	if r.steps == 1 {
		return errTest
	}
	if r.steps == 3 && r.inDesiredState {
		return errTest
	}
	return nil
}

func (r *testResource) EnforceState(ctx context.Context) error {
	if r.steps == 2 {
		return errTest
	}
	r.inDesiredState = true
	return nil
}

func (r *testResource) ManagedResources() *config.ManagedResources {
	return nil
}

type agentEndpointServiceConfigTestServer struct {
	lastReportTaskCompleteRequest *agentendpointpb.ReportTaskCompleteRequest
	progressError                 chan struct{}
	progressCancel                chan struct{}
}

func (*agentEndpointServiceConfigTestServer) ReceiveTaskNotification(req *agentendpointpb.ReceiveTaskNotificationRequest, srv agentendpointpb.AgentEndpointService_ReceiveTaskNotificationServer) error {
	return status.Errorf(codes.Unimplemented, "method ReceiveTaskNotification not implemented")
}

func (*agentEndpointServiceConfigTestServer) StartNextTask(ctx context.Context, req *agentendpointpb.StartNextTaskRequest) (*agentendpointpb.StartNextTaskResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartNextTask not implemented")
}

func (s *agentEndpointServiceConfigTestServer) ReportTaskProgress(ctx context.Context, req *agentendpointpb.ReportTaskProgressRequest) (*agentendpointpb.ReportTaskProgressResponse, error) {
	select {
	case s.progressError <- struct{}{}:
	default:
		return nil, status.Errorf(codes.Unimplemented, "")
	}

	select {
	case s.progressCancel <- struct{}{}:
	default:
		return &agentendpointpb.ReportTaskProgressResponse{TaskDirective: agentendpointpb.TaskDirective_STOP}, nil
	}

	return &agentendpointpb.ReportTaskProgressResponse{TaskDirective: agentendpointpb.TaskDirective_CONTINUE}, nil
}

func (s *agentEndpointServiceConfigTestServer) ReportTaskComplete(ctx context.Context, req *agentendpointpb.ReportTaskCompleteRequest) (*agentendpointpb.ReportTaskCompleteResponse, error) {
	s.lastReportTaskCompleteRequest = req
	return &agentendpointpb.ReportTaskCompleteResponse{}, nil
}

func (*agentEndpointServiceConfigTestServer) RegisterAgent(ctx context.Context, req *agentendpointpb.RegisterAgentRequest) (*agentendpointpb.RegisterAgentResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterAgent not implemented")
}

func (*agentEndpointServiceConfigTestServer) ReportInventory(ctx context.Context, req *agentendpointpb.ReportInventoryRequest) (*agentendpointpb.ReportInventoryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReportInventory not implemented")
}

func configOutputGen(msg string, st agentendpointpb.ApplyConfigTaskOutput_State, results []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult) *agentendpointpb.ReportTaskCompleteRequest {
	return &agentendpointpb.ReportTaskCompleteRequest{
		TaskType:     agentendpointpb.TaskType_APPLY_CONFIG_TASK,
		ErrorMessage: msg,
		Output: &agentendpointpb.ReportTaskCompleteRequest_ApplyConfigTaskOutput{
			ApplyConfigTaskOutput: &agentendpointpb.ApplyConfigTaskOutput{State: st, OsPolicyResults: results},
		},
		InstanceIdToken: testIDToken,
	}
}

func genTestResource(id string) *agentendpointpb.OSPolicy_Resource {
	return &agentendpointpb.OSPolicy_Resource{
		Id: id,
	}
}

func genTestResourceCompliance(id string, steps int, inDesiredState bool) *agentendpointpb.OSPolicyResourceCompliance {
	// TODO: test various types of executions.
	ret := &agentendpointpb.OSPolicyResourceCompliance{
		OsPolicyResourceId: id,
		ConfigSteps:        make([]*agentendpointpb.OSPolicyResourceConfigStep, 4),
	}

	// Validation
	if steps > 0 {
		outcome := agentendpointpb.OSPolicyResourceConfigStep_FAILED
		state := agentendpointpb.OSPolicyComplianceState_UNKNOWN
		if steps > 1 {
			outcome = agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
		}
		ret.ConfigSteps[0] = &agentendpointpb.OSPolicyResourceConfigStep{
			Type:    agentendpointpb.OSPolicyResourceConfigStep_VALIDATION,
			Outcome: outcome,
		}
		ret.State = state
	}
	// DesiredStateCheck
	if steps > 1 {
		outcome := agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
		state := agentendpointpb.OSPolicyComplianceState_NON_COMPLIANT
		if steps == 2 && !inDesiredState {
			outcome = agentendpointpb.OSPolicyResourceConfigStep_FAILED
			state = agentendpointpb.OSPolicyComplianceState_UNKNOWN
		} else if inDesiredState {
			state = agentendpointpb.OSPolicyComplianceState_COMPLIANT
		}
		ret.ConfigSteps[1] = &agentendpointpb.OSPolicyResourceConfigStep{
			Type:    agentendpointpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK,
			Outcome: outcome,
		}
		ret.State = state
	}
	// EnforceDesiredState
	if steps > 2 {
		outcome := agentendpointpb.OSPolicyResourceConfigStep_FAILED
		state := agentendpointpb.OSPolicyComplianceState_UNKNOWN
		if steps > 3 {
			outcome = agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
		}
		ret.ConfigSteps[2] = &agentendpointpb.OSPolicyResourceConfigStep{
			Type:    agentendpointpb.OSPolicyResourceConfigStep_DESIRED_STATE_ENFORCEMENT,
			Outcome: outcome,
		}
		ret.State = state
	}
	// DesiredStateCheckPostEnforcement{
	if steps > 3 {
		outcome := agentendpointpb.OSPolicyResourceConfigStep_SUCCEEDED
		state := agentendpointpb.OSPolicyComplianceState_NON_COMPLIANT
		if steps == 4 {
			outcome = agentendpointpb.OSPolicyResourceConfigStep_FAILED
			state = agentendpointpb.OSPolicyComplianceState_UNKNOWN
		} else {
			state = agentendpointpb.OSPolicyComplianceState_COMPLIANT
		}
		ret.ConfigSteps[3] = &agentendpointpb.OSPolicyResourceConfigStep{
			Type:    agentendpointpb.OSPolicyResourceConfigStep_DESIRED_STATE_CHECK_POST_ENFORCEMENT,
			Outcome: outcome,
		}
		ret.State = state
	}
	return ret
}

func genTestPolicy(id string) *agentendpointpb.ApplyConfigTask_OSPolicy {
	return &agentendpointpb.ApplyConfigTask_OSPolicy{
		Id:   id,
		Mode: agentendpointpb.OSPolicy_ENFORCEMENT,
		Resources: []*agentendpointpb.OSPolicy_Resource{
			genTestResource("r1"),
		},
	}
}

func genTestPolicyResult(id string, steps int, inDesiredState bool) *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult {
	return &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
		OsPolicyId: id,
		OsPolicyResourceCompliances: []*agentendpointpb.OSPolicyResourceCompliance{
			genTestResourceCompliance("r1", steps, inDesiredState),
		},
	}
}

func TestRunApplyConfig(t *testing.T) {
	ctx := context.Background()
	sameStateTimeWindow = 0
	res := &testResource{}
	newResource = func(r *agentendpointpb.OSPolicy_Resource) resourceIface {
		return resourceIface(res)
	}

	testConfig := &agentendpointpb.ApplyConfigTask{
		OsPolicies: []*agentendpointpb.ApplyConfigTask_OSPolicy{
			genTestPolicy("p1"),
		},
	}

	tests := []struct {
		name                string
		wantComReq          *agentendpointpb.ReportTaskCompleteRequest
		step                *agentendpointpb.ApplyConfigTask
		callsBeforeCancel   int
		callsBeforeErr      int
		stepsBeforeErr      int
		startInDesiredState bool
	}{
		// Normal cases:
		{
			"InDesiredState",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 2, true),
				},
			),
			testConfig,
			5, 5, 5, true,
		},
		{
			"EnforceDesiredState",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 5, false),
				},
			),
			testConfig,
			5, 5, 5, false,
		},
		{
			"NilPolicies",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED, []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			&agentendpointpb.ApplyConfigTask{OsPolicies: nil},
			5, 5, 5, false,
		},
		{
			"NoPolicies",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED, []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			&agentendpointpb.ApplyConfigTask{OsPolicies: nil},
			5, 5, 5, false,
		},

		// Step error cases

		{
			"ValidateError",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 1, false),
				},
			),
			testConfig,
			5, 5, 0, false,
		},
		{
			"CheckStateError",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 2, false),
				},
			),
			testConfig,
			5, 5, 1, false,
		},
		{
			"EnforceError",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 3, false),
				},
			),
			testConfig,
			5, 5, 2, false,
		},
		{
			"PostCheckError",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 4, false),
				},
			),
			testConfig,
			5, 5, 3, false,
		},

		// Cases where task is canceled by server at various points.
		{
			"CancelAfterSTARTED",
			// No results generated.
			configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED, []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			testConfig,
			0, 5, 5, false,
		},

		// Cases where task has task level error.
		{
			"ErrorReportingSTARTED",
			// No results
			configOutputGen(`Error reporting continuing state: error reporting task progress STARTED: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			testConfig,
			5, 0, 5, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &agentEndpointServiceConfigTestServer{
				progressError:  make(chan struct{}, tt.callsBeforeErr),
				progressCancel: make(chan struct{}, tt.callsBeforeCancel),
			}
			tc, err := newTestClient(ctx, srv)
			if err != nil {
				t.Fatal(err)
			}
			defer tc.close()

			res.inDesiredState = tt.startInDesiredState
			res.steps = tt.stepsBeforeErr

			if err := tc.client.RunApplyConfig(ctx, &agentendpointpb.Task{TaskDetails: &agentendpointpb.Task_ApplyConfigTask{ApplyConfigTask: tt.step}}); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.wantComReq, srv.lastReportTaskCompleteRequest, protocmp.Transform()); diff != "" {
				t.Fatalf("ReportTaskCompleteRequest mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
