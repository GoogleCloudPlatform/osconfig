//  Copyright 2021 Google Inc. All Rights Reserved.
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

package config

import (
	"context"
	"io/ioutil"
	"path"
	"testing"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

func TestExecResourceDownload(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		name                 string
		erpb                 *agentendpointpb.OSPolicy_Resource_ExecResource
		wantValidatePath     string
		wantValidateContents string
		wantEnforcePath      string
		wantEnforceContents  string
		goos                 string
	}{
		{
			"Script NONE Linux",
			&agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
			},
			"script",
			"validate",
			"script",
			"enforce",
			"linux",
		},
		{
			"Script NONE Windows",
			&agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
			},
			"script.cmd",
			"validate",
			"script.cmd",
			"enforce",
			"windows",
		},
		{
			"Script SHELL Linux",
			&agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
			},
			"script.sh",
			"validate",
			"script.sh",
			"enforce",
			"linux",
		},
		{
			"Script SHELL Windows",
			&agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
			},
			"script.cmd",
			"validate",
			"script.cmd",
			"enforce",
			"windows",
		},
		{
			"Script POWERSHELL Windows",
			&agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL,
				},
			},
			"script.ps1",
			"validate",
			"script.ps1",
			"enforce",
			"windows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goos = tt.goos
			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Exec{
						Exec: tt.erpb,
					},
				},
			}
			defer pr.Cleanup(ctx)

			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			resource := pr.resource.(*execResource)

			if tt.wantValidatePath != path.Base(resource.validatePath) {
				t.Errorf("unexpected validate path: %q", resource.validatePath)
			}
			data, err := ioutil.ReadFile(resource.validatePath)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantValidateContents != string(data) {
				t.Errorf("unexpected validate contents: %q", data)
			}

			if tt.wantEnforcePath != path.Base(resource.enforcePath) {
				t.Errorf("unexpected enforce path: %q", resource.enforcePath)
			}
			data, err = ioutil.ReadFile(resource.enforcePath)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantEnforceContents != string(data) {
				t.Errorf("unexpected enforce contents: %q", data)
			}
		})
	}
}
