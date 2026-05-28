//  Copyright 2020 Google Inc. All Rights Reserved.
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
	"errors"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

func init() {
	packages.YumExists = true
	packages.AptExists = true
	packages.GooGetExists = true
	packages.DpkgExists = true
	packages.RPMExists = true
	packages.ZypperExists = true
	packages.MSIExists = true
}

func newTestOSPolicyFileResource() *OSPolicyResource {
	return &OSPolicyResource{
		OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
			ResourceType: &agentendpointpb.OSPolicy_Resource_File_{
				File: &agentendpointpb.OSPolicy_Resource_FileResource{
					Path:  "/path/does/not/exist",
					State: agentendpointpb.OSPolicy_Resource_FileResource_ABSENT,
				},
			},
		},
	}
}

func TestOSPolicyResource_Validate(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name    string
		pr      *OSPolicyResource
		wantErr error
	}{
		{
			name: "Validate call with nil ResourceType, expect error",
			pr: &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: nil,
				},
			},
			wantErr: errors.New("ResourceType field not set"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pr.Validate(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestOSPolicyResource_CheckState(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name    string
		setup   func(t *testing.T, pr *OSPolicyResource) error
		pr      *OSPolicyResource
		wantErr error
	}{
		{
			name:    "CheckState call before Validate, expect run before Validate error",
			pr:      newTestOSPolicyFileResource(),
			wantErr: errors.New("CheckState run before Validate"),
		},
		{
			name: "CheckState call after Validate, expect success",
			setup: func(t *testing.T, pr *OSPolicyResource) error {
				return pr.Validate(ctx)
			},
			pr:      newTestOSPolicyFileResource(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(t, tt.pr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}
			err := tt.pr.CheckState(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestOSPolicyResource_EnforceState(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name    string
		setup   func(t *testing.T, pr *OSPolicyResource) error
		pr      *OSPolicyResource
		wantErr error
	}{
		{
			name:    "EnforceState call before Validate, expect run before Validate error",
			pr:      newTestOSPolicyFileResource(),
			wantErr: errors.New("EnforceState run before Validate"),
		},
		{
			name: "EnforceState call after Validate, expect success",
			setup: func(t *testing.T, pr *OSPolicyResource) error {
				tmpPath := utiltest.WriteToTempFileMust(t, "enforce-state-test", []byte("test"))
				pr.OSPolicy_Resource.GetFile().Path = tmpPath
				return pr.Validate(ctx)
			},
			pr:      newTestOSPolicyFileResource(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(t, tt.pr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}
			err := tt.pr.EnforceState(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestOSPolicyResource_Cleanup(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name    string
		setup   func(t *testing.T, pr *OSPolicyResource) error
		pr      *OSPolicyResource
		wantErr error
	}{
		{
			name:    "Cleanup call before Validate, expect run before Validate error",
			pr:      newTestOSPolicyFileResource(),
			wantErr: errors.New("Cleanup run before Validate"),
		},
		{
			name: "Cleanup call after Validate, expect success",
			setup: func(t *testing.T, pr *OSPolicyResource) error {
				return pr.Validate(ctx)
			},
			pr:      newTestOSPolicyFileResource(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(t, tt.pr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}
			err := tt.pr.Cleanup(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestOSPolicyResource_PopulateOutput(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name    string
		setup   func(t *testing.T, pr *OSPolicyResource) error
		pr      *OSPolicyResource
		wantErr error
	}{
		{
			name:    "PopulateOutput call before Validate, expect run before Validate error",
			pr:      newTestOSPolicyFileResource(),
			wantErr: errors.New("PopulateOutput run before Validate"),
		},
		{
			name: "PopulateOutput call after Validate, expect success",
			setup: func(t *testing.T, pr *OSPolicyResource) error {
				return pr.Validate(ctx)
			},
			pr:      newTestOSPolicyFileResource(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(t, tt.pr); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}
			err := tt.pr.PopulateOutput(nil)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
