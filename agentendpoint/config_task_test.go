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
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/GoogleCloudPlatform/osconfig/config"
	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

type testResource struct{}

func (r *testResource) InDesiredState() bool {
	return false
}

func (r *testResource) Cleanup(ctx context.Context) error {
	return nil
}

func (r *testResource) Validate(ctx context.Context) error {
	return nil
}

func (r *testResource) CheckState(ctx context.Context) error {
	return nil
}

func (r *testResource) EnforceState(ctx context.Context) error {
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

func (*agentEndpointServiceConfigTestServer) LookupEffectiveGuestPolicies(ctx context.Context, req *agentendpointpb.LookupEffectiveGuestPoliciesRequest) (*agentendpointpb.LookupEffectiveGuestPoliciesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LookupEffectiveGuestPolicies not implemented")
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

func genTestResource(id string) *agentendpointpb.ApplyConfigTask_OSPolicy_Resource {
	return &agentendpointpb.ApplyConfigTask_OSPolicy_Resource{
		Id: id,
	}
}

func genTestResourceResult(id string, steps int) *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult {
	// TODO: test various types of executions.
	ret := &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult{
		Id:             id,
		ExecutionSteps: make([]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep, 4),
	}

	// Validation
	if steps > 0 {
		ret.ExecutionSteps[0] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_Validation{
				Validation: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_Validation{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_Validation_OK,
				},
			}}
	}
	// CheckDesiredState
	if steps > 1 {
		ret.ExecutionSteps[1] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_DesiredStateCheck{
				DesiredStateCheck: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheck_NOT_IN_DESIRED_STATE,
				},
			}}
	}
	// EnforceDesiredState
	if steps > 2 {
		ret.ExecutionSteps[2] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_DesiredStateEnforcement{
				DesiredStateEnforcement: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateEnforcement{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateEnforcement_SUCCESS,
				},
			}}
	}
	// CheckDesiredStatePostEnforcement{
	if steps > 3 {
		ret.ExecutionSteps[3] = &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_ExecutionStep_DesiredStateCheckPostEnforcement{
				DesiredStateCheckPostEnforcement: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheckPostEnforcement{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult_DesiredStateCheckPostEnforcement_NOT_IN_DESIRED_STATE,
				},
			}}
	}
	return ret
}

func genTestPolicy(id string) *agentendpointpb.ApplyConfigTask_OSPolicy {
	return &agentendpointpb.ApplyConfigTask_OSPolicy{
		Id:   id,
		Mode: agentendpointpb.ApplyConfigTask_OSPolicy_ENFORCEMENT,
		Resources: []*agentendpointpb.ApplyConfigTask_OSPolicy_Resource{
			genTestResource("r1"),
			genTestResource("r2"),
		},
	}
}

func genTestPolicyResult(id string, steps int) *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult {
	return &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
		Id: id,
		ResourceResults: []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResult{
			genTestResourceResult("r1", steps),
			genTestResourceResult("r2", steps),
		},
	}
}

func TestRunApplyConfig(t *testing.T) {
	ctx := context.Background()
	sameStateTimeWindow = 0
	newResource = func(r *agentendpointpb.ApplyConfigTask_OSPolicy_Resource) resourceIface {
		return resourceIface(&testResource{})
	}

	testConfig := &agentendpointpb.ApplyConfigTask{
		OsPolicies: []*agentendpointpb.ApplyConfigTask_OSPolicy{
			genTestPolicy("p1"),
			genTestPolicy("p2"),
		},
	}

	tests := []struct {
		name              string
		wantComReq        *agentendpointpb.ReportTaskCompleteRequest
		step              *agentendpointpb.ApplyConfigTask
		callsBeforeCancel int
		callsBeforeErr    int
	}{

		// Normal cases:
		{
			"InDesiredState",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 4),
					genTestPolicyResult("p2", 4),
				},
			),
			testConfig,
			5, 5,
		},
		{
			"NilPolicies",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED, []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			&agentendpointpb.ApplyConfigTask{OsPolicies: nil},
			5, 5,
		},
		{
			"NoPolicies",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED, []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			&agentendpointpb.ApplyConfigTask{OsPolicies: nil},
			5, 5,
		},

		// Cases where task is canceled by server at various points.
		{
			"CancelSTARTED",
			// No results generated.
			configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED, []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			testConfig,
			0, 5,
		},
		{
			"CancelENFORCING_DESIRED_STATE",
			// Populates results up through CHECKING_DESIRED_STATE only.
			configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 2),
					genTestPolicyResult("p2", 2),
				},
			),
			testConfig,
			1, 5,
		},

		// Cases where task has task level error.
		{
			"ErrorSTARTED",
			// No results
			configOutputGen(`Error reporting continuing state: error reporting task progress STARTED: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{}),
			testConfig, 5,
			0,
		},
		{
			"ErrorENFORCING_DESIRED_STATE",
			// Populates results up through CHECKING_DESIRED_STATE only.
			configOutputGen(`Error reporting continuing state: error reporting task progress APPLYING_CONFIG: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
				[]*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", 2),
					genTestPolicyResult("p2", 2),
				},
			),
			testConfig, 5,
			1,
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

			if err := tc.client.RunApplyConfig(ctx, &agentendpointpb.Task{TaskDetails: &agentendpointpb.Task_ApplyConfigTask{ApplyConfigTask: tt.step}}); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.wantComReq, srv.lastReportTaskCompleteRequest, protocmp.Transform()); diff != "" {
				t.Fatalf("ReportTaskCompleteRequest mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
