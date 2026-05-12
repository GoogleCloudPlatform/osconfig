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
	"fmt"
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

func TestErrorBeforeValidate(t *testing.T) {
	pr := &OSPolicyResource{
		OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
			ResourceType: nil,
		},
	}
	ctx := context.Background()

	tests := []struct {
		funcName string
		fn       func() error
	}{
		{
			funcName: "CheckState",
			fn:       func() error { return pr.CheckState(ctx) },
		},
		{
			funcName: "EnforceState",
			fn:       func() error { return pr.EnforceState(ctx) },
		},
		{
			funcName: "Cleanup",
			fn:       func() error { return pr.Cleanup(ctx) },
		},
		{
			funcName: "PopulateOutput",
			fn:       func() error { return pr.PopulateOutput(nil) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.funcName, func(t *testing.T) {
			err := tt.fn()
			utiltest.AssertErrorMatch(t, err, fmt.Errorf("%v run before Validate", tt.funcName))
		})
	}
}

func TestValidateNilResourceType(t *testing.T) {
	pr := &OSPolicyResource{
		OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
			ResourceType: nil,
		},
	}
	err := pr.Validate(context.Background())
	utiltest.AssertErrorMatch(t, err, errors.New("ResourceType field not set"))
}
