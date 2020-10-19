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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
	"github.com/GoogleCloudPlatform/osconfig/util"
	"github.com/google/go-cmp/cmp"
)

func TestFileResourceValidate(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "foo")
	if err := ioutil.WriteFile(tmpFile, nil, 0644); err != nil {
		t.Fatal(err)
	}

	var tests = []struct {
		name   string
		frpb   *agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource
		wantMR ManagedFile
	}{
		{
			"Absent",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path:  "path",
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_ABSENT,
			},
			ManagedFile{
				Path:  "path",
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_ABSENT,
			},
		},
		{
			"Present",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path:  "path",
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
			},
			ManagedFile{
				Path:       "path",
				State:      agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
				Permisions: defaultFilePerms,
			},
		},
		{
			"ContentsMatch",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path:  "path",
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_CONTENTS_MATCH,
			},
			ManagedFile{
				Path:       "path",
				State:      agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_CONTENTS_MATCH,
				Permisions: defaultFilePerms,
			},
		},
		{
			"Permissions",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path:        "path",
				State:       agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
				Permissions: "0777",
			},
			ManagedFile{
				Path:       "path",
				State:      agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
				Permisions: 0777,
			},
		},
		{
			"LocalPath",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path: "path",
				Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_File{
					File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_LocalPath{
							LocalPath: tmpFile,
						},
					},
				},
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
			},
			ManagedFile{
				Path:       "path",
				State:      agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
				Permisions: defaultFilePerms,
				checksum:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				source:     tmpFile,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				ApplyConfigTask_OSPolicy_Resource: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource{
					ResourceType: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_{
						File: tt.frpb,
					},
				},
			}
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diff := cmp.Diff(pr.ManagedResources(), &ManagedResources{Files: []ManagedFile{tt.wantMR}}, cmp.AllowUnexported(ManagedFile{})); diff != "" {
				t.Errorf("OSPolicyResource does not match expectation: (-got +want)\n%s", diff)
			}
			if diff := cmp.Diff(pr.resource.(*fileResource).managedFile, tt.wantMR, cmp.AllowUnexported(ManagedFile{})); diff != "" {
				t.Errorf("fileResource does not match expectation: (-got +want)\n%s", diff)
			}
		})
	}
}

func TestFileResourceCheckState(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "foo")
	if err := ioutil.WriteFile(tmpFile, []byte("foo"), 0644); err != nil {
		t.Fatal(err)
	}
	tmpFile2 := filepath.Join(tmpDir, "bar")
	if err := ioutil.WriteFile(tmpFile2, []byte("bar"), 0644); err != nil {
		t.Fatal(err)
	}

	var tests = []struct {
		name               string
		frpb               *agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource
		wantInDesiredState bool
	}{
		{
			"AbsentAndAbsent",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path:  filepath.Join(tmpDir, "dne"),
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_ABSENT,
			},
			true,
		},
		{
			"AbsentAndPresent",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path:  tmpFile,
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_ABSENT,
			},
			false,
		},
		{
			"PresentAndAbsent",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path: filepath.Join(tmpDir, "dne"),
				Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_File{
					File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_LocalPath{
							LocalPath: tmpFile,
						},
					},
				},
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
			},
			false,
		},
		{
			"PresentAndPresent",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path:  tmpFile,
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
			},
			true,
		},
		{
			"ContentsMatchLocalPath",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path: tmpFile,
				Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_File{
					File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_LocalPath{
							LocalPath: tmpFile,
						},
					},
				},
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_CONTENTS_MATCH,
			},
			true,
		},
		{
			"ContentsDontMatchLocalPath",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path: tmpFile2,
				Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_File{
					File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_LocalPath{
							LocalPath: tmpFile,
						},
					},
				},
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_CONTENTS_MATCH,
			},
			false,
		},
		{
			"ContentsDontMatchDNE",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path: filepath.Join(tmpDir, "dne"),
				Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_File{
					File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File{
						File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_LocalPath{
							LocalPath: tmpFile,
						},
					},
				},
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_CONTENTS_MATCH,
			},
			false,
		},
		{
			"ContentMatchFromContent",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path: tmpFile,
				Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_Content{
					Content: "foo",
				},
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_CONTENTS_MATCH,
			},
			true,
		},
		{
			"ContentsDontMatchFromContent",
			&agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
				Path: tmpFile,
				Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_Content{
					Content: "bar",
				},
				State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_CONTENTS_MATCH,
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &OSPolicyResource{
				ApplyConfigTask_OSPolicy_Resource: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource{
					ResourceType: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_{
						File: tt.frpb,
					},
				},
			}
			if err := pr.Validate(ctx); err != nil {
				t.Fatalf("Unexpected Validate error: %v", err)
			}

			if err := pr.CheckState(ctx); err != nil {
				t.Fatalf("Unexpected CheckState error: %v", err)
			}

			if tt.wantInDesiredState != pr.InDesiredState() {
				t.Fatalf("Unexpected InDesiredState, want: %t, got: %t", tt.wantInDesiredState, pr.InDesiredState())
			}
		})
	}
}

func TestFileResourceEnforceStateAbsent(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "foo")
	if err := ioutil.WriteFile(tmpFile, []byte("foo"), 0644); err != nil {
		t.Fatal(err)
	}

	frpb := &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
		Path:  tmpFile,
		State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_ABSENT,
	}
	pr := &OSPolicyResource{
		ApplyConfigTask_OSPolicy_Resource: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource{
			ResourceType: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_{File: frpb},
		},
	}
	if err := pr.Validate(ctx); err != nil {
		t.Fatalf("Unexpected Validate error: %v", err)
	}

	if err := pr.EnforceState(ctx); err != nil {
		t.Fatalf("Unexpected EnforceState error: %v", err)
	}

	if util.Exists(tmpFile) {
		t.Error("tmpFile still exists after EnforceState")
	}
}

func TestFileResourceEnforceStatePresent(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	srcFile := filepath.Join(tmpDir, "foo")
	if err := ioutil.WriteFile(srcFile, []byte("foo"), 0644); err != nil {
		t.Fatal(err)
	}
	wantFile := filepath.Join(tmpDir, "bar")

	frpb := &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource{
		Path: wantFile,
		Source: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_File{
			File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File{
				File: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_LocalPath{
					LocalPath: srcFile,
				},
			},
		},
		State: agentendpointpb.ApplyConfigTask_OSPolicy_Resource_FileResource_PRESENT,
	}
	pr := &OSPolicyResource{
		ApplyConfigTask_OSPolicy_Resource: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource{
			ResourceType: &agentendpointpb.ApplyConfigTask_OSPolicy_Resource_File_{File: frpb},
		},
	}
	if err := pr.Validate(ctx); err != nil {
		t.Fatalf("Unexpected Validate error: %v", err)
	}

	if err := pr.EnforceState(ctx); err != nil {
		t.Fatalf("Unexpected EnforceState error: %v", err)
	}

	match, err := contentsMatch(wantFile, pr.resource.(*fileResource).managedFile.checksum)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Fatal("Repo file contents do not match after enforcement")
	}
}
