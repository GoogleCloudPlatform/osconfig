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
	"testing"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestPackageResourceValidate(t *testing.T) {
	ctx := context.Background()
	packages.YumExists = true
	packages.AptExists = true
	packages.GooGetExists = true
	packages.DpkgExists = true
	packages.RPMExists = true
	packages.ZypperExists = true
	packages.MSIExecExists = true
	var tests = []struct {
		name    string
		wantErr bool
		prpb    *agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource
		wantMP  ManagedPackage
	}{
		{
			"Blank",
			true,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{},
			ManagedPackage{},
		},
		{
			"AptInstalled",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Apt{
					Apt: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}},
			ManagedPackage{Apt: AptPackage{Install: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}},
		},
		{
			"AptRemoved",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Apt{
					Apt: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}},
			ManagedPackage{Apt: AptPackage{Remove: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_APT{Name: "foo"}}},
		},
		{
			"DebInstalled",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb_{
					Deb: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
						Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
							File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
			ManagedPackage{Deb: DebPackage{Install: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"DebRemoved",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb_{
					Deb: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
						Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
							File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
			ManagedPackage{Deb: DebPackage{Remove: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Deb{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"GoGetInstalled",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Googet{
					Googet: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}},
			ManagedPackage{GooGet: GooGetPackage{Install: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}},
		},
		{
			"GooGetRemoved",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Googet{
					Googet: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}},
			ManagedPackage{GooGet: GooGetPackage{Remove: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_GooGet{Name: "foo"}}},
		},
		{
			"MSIInstalled",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Msi{
					Msi: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
						Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
							File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
			ManagedPackage{MSI: MSIPackage{Install: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"MSIRemoved",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Msi{
					Msi: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
						Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
							File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
			ManagedPackage{MSI: MSIPackage{Remove: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_MSI{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"YumInstalled",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Yum{
					Yum: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}},
			ManagedPackage{Yum: YumPackage{Install: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}},
		},
		{
			"YumRemoved",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Yum{
					Yum: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}},
			ManagedPackage{Yum: YumPackage{Remove: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_YUM{Name: "foo"}}},
		},
		{
			"ZypperInstalled",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper_{
					Zypper: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}},
			ManagedPackage{Zypper: ZypperPackage{Install: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}},
		},
		{
			"ZypperRemoved",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper_{
					Zypper: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}},
			ManagedPackage{Zypper: ZypperPackage{Remove: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Zypper{Name: "foo"}}},
		},
		{
			"RPMInstalled",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_INSTALLED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Rpm{
					Rpm: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
						Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
							File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
			ManagedPackage{RPM: RPMPackage{Install: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
		{
			"RPMRemoved",
			false,
			&agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource{
				DesiredState: agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_REMOVED,
				SystemPackage: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_Rpm{
					Rpm: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
						Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
							File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
			ManagedPackage{RPM: RPMPackage{Remove: &agentendpointpb.ApplyConfigTask_Config_Resource_PackageResource_RPM{
				Source: &agentendpointpb.ApplyConfigTask_Config_Resource_File{
					File: &agentendpointpb.ApplyConfigTask_Config_Resource_File_LocalPath{LocalPath: "foo"}}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				ApplyConfigTask_Config_Resource: &agentendpointpb.ApplyConfigTask_Config_Resource{
					ResourceType: &agentendpointpb.ApplyConfigTask_Config_Resource_Pkg{Pkg: tt.prpb},
				},
			}
			err := pr.Validate(ctx)
			if err != nil && !tt.wantErr {
				t.Fatalf("Unexpected error: %v", err)
			}
			if err == nil && tt.wantErr {
				t.Fatal("Expected error and did not get one.")
			}

			wantMR := &ManagedResources{Packages: []ManagedPackage{tt.wantMP}}
			if err != nil {
				wantMR = nil
			}
			if diff := cmp.Diff(wantMR, pr.ManagedResources(), protocmp.Transform()); diff != "" {
				t.Errorf("OSPolicyResource does not match expectation: (-got +want)\n%s", diff)
			}

			if diff := cmp.Diff(tt.wantMP, pr.resource.(*packageResouce).policy, protocmp.Transform()); diff != "" {
				t.Errorf("packageResouce does not match expectation: (-got +want)\n%s", diff)
			}
		})
	}
}
