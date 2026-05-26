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
	"context"
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

func TestOSPolicyResourceMethods(t *testing.T) {
	ctx := context.Background()

	// Helper to create a fresh valid OSPolicyResource
	newPR := func(path string) *OSPolicyResource {
		return &OSPolicyResource{
			OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
				ResourceType: &agentendpointpb.OSPolicy_Resource_File_{
					File: &agentendpointpb.OSPolicy_Resource_FileResource{
						Path:  path,
						State: agentendpointpb.OSPolicy_Resource_FileResource_ABSENT,
					},
				},
			},
		}
	}

	tests := []struct {
		name      string
		setupFunc func(t *testing.T) error
		wantErr   error
	}{
		{
			name: "CheckState call before Validate, expect run before Validate error",
			setupFunc: func(t *testing.T) error {
				pr := newPR("/path/does/not/exist")
				return pr.CheckState(ctx)
			},
			wantErr: errors.New("CheckState run before Validate"),
		},
		{
			name: "EnforceState call before Validate, expect run before Validate error",
			setupFunc: func(t *testing.T) error {
				pr := newPR("/path/does/not/exist")
				return pr.EnforceState(ctx)
			},
			wantErr: errors.New("EnforceState run before Validate"),
		},
		{
			name: "Cleanup call before Validate, expect run before Validate error",
			setupFunc: func(t *testing.T) error {
				pr := newPR("/path/does/not/exist")
				return pr.Cleanup(ctx)
			},
			wantErr: errors.New("Cleanup run before Validate"),
		},
		{
			name: "PopulateOutput call before Validate, expect run before Validate error",
			setupFunc: func(t *testing.T) error {
				pr := newPR("/path/does/not/exist")
				return pr.PopulateOutput(nil)
			},
			wantErr: errors.New("PopulateOutput run before Validate"),
		},
		{
			name: "CheckState call after Validate, expect success",
			setupFunc: func(t *testing.T) error {
				pr := newPR("/path/does/not/exist")
				if err := pr.Validate(ctx); err != nil {
					return err
				}
				return pr.CheckState(ctx)
			},
			wantErr: nil,
		},
		{
			name: "EnforceState call after Validate, expect success",
			setupFunc: func(t *testing.T) error {
				tmpPath := utiltest.WriteToTempFileMust(t, "enforce-state-test", []byte("test"))
				pr := newPR(tmpPath)
				if err := pr.Validate(ctx); err != nil {
					return err
				}
				return pr.EnforceState(ctx)
			},
			wantErr: nil,
		},
		{
			name: "Cleanup call after Validate, expect success",
			setupFunc: func(t *testing.T) error {
				pr := newPR("/path/does/not/exist")
				if err := pr.Validate(ctx); err != nil {
					return err
				}
				return pr.Cleanup(ctx)
			},
			wantErr: nil,
		},
		{
			name: "PopulateOutput call after Validate, expect success",
			setupFunc: func(t *testing.T) error {
				pr := newPR("/path/does/not/exist")
				if err := pr.Validate(ctx); err != nil {
					return err
				}
				return pr.PopulateOutput(nil)
			},
			wantErr: nil,
		},
		{
			name: "Validate call with nil ResourceType, expect error",
			setupFunc: func(t *testing.T) error {
				pr := &OSPolicyResource{
					OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
						ResourceType: nil,
					},
				}
				return pr.Validate(ctx)
			},
			wantErr: errors.New("ResourceType field not set"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupFunc(t)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
