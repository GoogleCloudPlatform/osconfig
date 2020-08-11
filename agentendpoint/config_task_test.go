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

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

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

func configOutputGen(msg string, st agentendpointpb.ApplyConfigTaskOutput_State, results *agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults) *agentendpointpb.ReportTaskCompleteRequest {
	return &agentendpointpb.ReportTaskCompleteRequest{
		TaskType:     agentendpointpb.TaskType_APPLY_CONFIG_TASK,
		ErrorMessage: msg,
		Output: &agentendpointpb.ReportTaskCompleteRequest_ApplyConfigTaskOutput{
			ApplyConfigTaskOutput: &agentendpointpb.ApplyConfigTaskOutput{State: st, ConfigAssignmentResults: results},
		},
		InstanceIdToken: testIDToken,
	}
}

func genTestResource(id string) *agentendpointpb.ApplyConfigTask_Config_Resource {
	return &agentendpointpb.ApplyConfigTask_Config_Resource{
		Id: id,
	}
}

func genTestResourceResult(id string, steps int) *agentendpointpb.ApplyConfigTaskOutput_ResourceResult {
	// TODO: test various types of executions.
	ret := &agentendpointpb.ApplyConfigTaskOutput_ResourceResult{
		Id:             id,
		ExecutionSteps: make([]*agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep, 4),
	}

	if steps > 0 {
		ret.ExecutionSteps[0] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_Validation{
				Validation: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_ResourceResult_Validation_OK,
				},
			}}
	}
	if steps > 1 {
		ret.ExecutionSteps[1] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_CheckDesiredState{
				CheckDesiredState: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredState_IN_DESIRED_STATE,
				},
			}}
	}
	if steps > 2 {
		ret.ExecutionSteps[2] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_EnforceDesiredState{
				EnforceDesiredState: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_ResourceResult_EnforceDesiredState_SUCCESS,
				},
			}}
	}
	if steps > 4 {
		ret.ExecutionSteps[3] = &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep{
			Step: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_ExcecutionStep_CheckDesiredStatePostEnforcement{
				CheckDesiredStatePostEnforcement: &agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement{
					Outcome: agentendpointpb.ApplyConfigTaskOutput_ResourceResult_CheckDesiredStatePostEnforcement_IN_DESIRED_STATE,
				},
			}}
	}
	return ret
}

func genTestPolicy(id string) *agentendpointpb.ApplyConfigTask_Config_OSPolicy {
	return &agentendpointpb.ApplyConfigTask_Config_OSPolicy{
		Id:   id,
		Mode: agentendpointpb.ApplyConfigTask_Config_OSPolicy_ENFORCEMENT,
		Resources: []*agentendpointpb.ApplyConfigTask_Config_Resource{
			genTestResource("r1"),
			genTestResource("r2"),
		},
	}
}

func genTestPolicyResult(id string, steps int) *agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult {
	return &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
		Id: id,
		Result: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult_ResourceResults{
			ResourceResults: &agentendpointpb.ApplyConfigTaskOutput_ResourceResults{
				Results: []*agentendpointpb.ApplyConfigTaskOutput_ResourceResult{
					genTestResourceResult("r1", steps),
					genTestResourceResult("r2", steps),
				},
			},
		},
	}
}

func genTestAssignment(id string) *agentendpointpb.ApplyConfigTask_Config_ConfigAssignment {
	return &agentendpointpb.ApplyConfigTask_Config_ConfigAssignment{
		ConfigAssignment: id,
		Policies: []*agentendpointpb.ApplyConfigTask_Config_OSPolicy{
			genTestPolicy("p1"), genTestPolicy("p2"),
		},
	}
}

func genTestAssignmentResult(id string, steps int) *agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult {
	return &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
		ConfigAssignment: id,
		Result: &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult_OsPolicyResults{
			OsPolicyResults: &agentendpointpb.ApplyConfigTaskOutput_OSPolicyResults{
				Results: []*agentendpointpb.ApplyConfigTaskOutput_OSPolicyResult{
					genTestPolicyResult("p1", steps),
					genTestPolicyResult("p2", steps),
				},
			},
		},
	}
}

func TestRunApplyConfig(t *testing.T) {
	ctx := context.Background()

	testConfig := &agentendpointpb.ApplyConfigTask_Config{
		ConfigAssignments: []*agentendpointpb.ApplyConfigTask_Config_ConfigAssignment{
			genTestAssignment("a1"),
			genTestAssignment("a2"),
		},
	}

	tests := []struct {
		name              string
		wantComReq        *agentendpointpb.ReportTaskCompleteRequest
		step              *agentendpointpb.ApplyConfigTask_Config
		callsBeforeCancel int
		callsBeforeErr    int
	}{
		// Normal cases:
		{
			"InDesiredState",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
					Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
						genTestAssignmentResult("a1", 2),
						genTestAssignmentResult("a2", 2),
					},
				},
			),
			testConfig,
			5, 5,
		},
		{
			"NilAssignments",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED, &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{}),
			&agentendpointpb.ApplyConfigTask_Config{ConfigAssignments: nil},
			5, 5,
		},
		{
			"NoAssignments",
			configOutputGen("", agentendpointpb.ApplyConfigTaskOutput_SUCCEEDED, &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{}),
			&agentendpointpb.ApplyConfigTask_Config{ConfigAssignments: []*agentendpointpb.ApplyConfigTask_Config_ConfigAssignment{}},
			5, 5,
		},

		// Cases where task is canceled by server at various points.
		{
			"CancelSTARTED",
			// No results generated.
			configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED, &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{}),
			testConfig,
			0, 5,
		},
		{
			"CancelVALIDATING",
			// This generates results, but never populates.
			configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
					Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
						genTestAssignmentResult("a1", 0),
						genTestAssignmentResult("a2", 0),
					},
				},
			),
			testConfig,
			1, 5,
		},
		{
			"CancelCHECKING_DESIRED_STATE",
			// Populates results up through VALIDATE only.
			configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
					Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
						genTestAssignmentResult("a1", 1),
						genTestAssignmentResult("a2", 1),
					},
				},
			),
			testConfig,
			2, 5,
		},
		{
			"CancelENFORCING_DESIRED_STATE",
			// Populates results up through CHECKING_DESIRED_STATE only.
			configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
					Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
						genTestAssignmentResult("a1", 2),
						genTestAssignmentResult("a2", 2),
					},
				},
			),
			testConfig,
			3, 5,
		},
		// Need to implement more steps before testing this case.
		/*
			{
				"CancelCHECKING_DESIRED_STATE_POST_ENFORCEMENT",
				// Populates results up through ENFORCING_DESIRED_STATE only.
				configOutputGen(errServerCancel.Error(), agentendpointpb.ApplyConfigTaskOutput_CANCELLED,
					&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
						Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
							genTestAssignmentResult("a1", 3),
							genTestAssignmentResult("a2", 3),
						},
					},
				),
				testConfig,
				4, 5,
			},
		*/

		// Cases where task has task level error.
		{
			"ErrorSTARTED",
			// No results
			configOutputGen(`Error reporting continuing state: error reporting task progress STARTED: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{}),
			testConfig, 5,
			0,
		},
		{
			"ErrorVALIDATING",
			// This generates results, but never populates.
			configOutputGen(`Error reporting continuing state: error reporting task progress VALIDATING: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
					Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
						genTestAssignmentResult("a1", 0),
						genTestAssignmentResult("a2", 0),
					},
				},
			),
			testConfig, 5,
			1,
		},
		{
			"ErrorCHECKING_DESIRED_STATE",
			// Populates results up through VALIDATE only.
			configOutputGen(`Error reporting continuing state: error reporting task progress CHECKING_DESIRED_STATE: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
					Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
						genTestAssignmentResult("a1", 1),
						genTestAssignmentResult("a2", 1),
					},
				},
			),
			testConfig, 5,
			2,
		},
		{
			"ErrorENFORCING_DESIRED_STATE",
			// Populates results up through CHECKING_DESIRED_STATE only.
			configOutputGen(`Error reporting continuing state: error reporting task progress ENFORCING_DESIRED_STATE: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
				&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
					Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
						genTestAssignmentResult("a1", 2),
						genTestAssignmentResult("a2", 2),
					},
				},
			),
			testConfig, 5,
			3,
		},
		// Need to implement more steps before testing this case.
		/*
			{
				"ErrorCHECKING_DESIRED_STATE_POST_ENFORCEMENT",
				// Populates results up through ENFORCING_DESIRED_STATE only.
				configOutputGen(`Error reporting continuing state: error reporting task progress CHECKING_DESIRED_STATE_POST_ENFORCEMENT: error calling ReportTaskProgress: code: "Unimplemented", message: "", details: []`, agentendpointpb.ApplyConfigTaskOutput_FAILED,
					&agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults{
						Results: []*agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResult{
							genTestAssignmentResult("a1", 3),
							genTestAssignmentResult("a2", 3),
						},
					},
				),
				testConfig,
				5, 4,
			},
		*/
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

			if err := tc.client.RunApplyConfig(ctx, &agentendpointpb.Task{TaskDetails: &agentendpointpb.Task_ApplyConfigTask{ApplyConfigTask: &agentendpointpb.ApplyConfigTask{Config: tt.step}}}); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.wantComReq, srv.lastReportTaskCompleteRequest, protocmp.Transform()); diff != "" {
				t.Fatalf("ReportTaskCompleteRequest mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
