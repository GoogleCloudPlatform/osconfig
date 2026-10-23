//go:build e2e

//  Copyright 2026 Google LLC
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

package e2etests_test

import (
	"context"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
	osconfigv1beta "google.golang.org/api/osconfig/v1beta"
)

const (
	aptTestRepoBaseURL    = "http://packages.cloud.google.com/apt"
	aptRaptureGpgKey      = "https://packages.cloud.google.com/apt/doc/apt-key.gpg"
	gcsfuseYumRepoBaseURL = "https://packages.cloud.google.com/yum/repos/gcsfuse-el7-x86_64"
	gooTestRepoURL        = "https://packages.cloud.google.com/yuck/repos/google-cloud-sap-agent-windows"
)

var yumRaptureGpgKeys = []string{
	"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
	"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
}

func aptPackageRepo(distribution string) *osconfigv1beta.PackageRepository {
	return &osconfigv1beta.PackageRepository{
		Apt: &osconfigv1beta.AptRepository{
			ArchiveType:  "DEB",
			Uri:          aptTestRepoBaseURL,
			Distribution: distribution,
			Components:   []string{"main"},
			GpgKey:       aptRaptureGpgKey,
		},
	}
}

func yumPackageRepo() *osconfigv1beta.PackageRepository {
	return &osconfigv1beta.PackageRepository{
		Yum: &osconfigv1beta.YumRepository{
			Id:          "gcsfuse",
			DisplayName: "gcsfuse",
			BaseUrl:     gcsfuseYumRepoBaseURL,
			GpgKeys:     yumRaptureGpgKeys,
		},
	}
}

func zypperPackageRepo() *osconfigv1beta.PackageRepository {
	return &osconfigv1beta.PackageRepository{
		Zypper: &osconfigv1beta.ZypperRepository{
			Id:          "gcsfuse",
			DisplayName: "gcsfuse",
			BaseUrl:     gcsfuseYumRepoBaseURL,
			GpgKeys:     yumRaptureGpgKeys,
		},
	}
}

func gooPackageRepo() *osconfigv1beta.PackageRepository {
	return &osconfigv1beta.PackageRepository{
		Goo: &osconfigv1beta.GooRepository{
			Name: "google-cloud-sap-agent",
			Url:  gooTestRepoURL,
		},
	}
}

type guestPolicyNewRepoTestCase struct {
	name               string
	image              string
	packageName        string
	wantPackageManager string
	repository         *osconfigv1beta.PackageRepository
	machineType        string
	timeout            time.Duration
}

// TestGuestPoliciesPackageRepositories verifies that the OS Config agent configures
// a new PackageRepository (Apt, Yum, Zypper, GooGet) defined in an OS Config GuestPolicy
// and installs a package from that new repository.
// This test migrates the "Add a new package from new repository" scenario from
// test_suites/guestpolicies/guest_policies.go.
func TestGuestPoliciesPackageRepositories(t *testing.T) {
	testCases := []guestPolicyNewRepoTestCase{
		// Debian
		{
			name:               "debian-12",
			image:              "projects/debian-cloud/global/images/family/debian-12",
			packageName:        "gcsfuse",
			wantPackageManager: "deb",
			repository:         aptPackageRepo("gcsfuse-bookworm"),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		// Ubuntu
		{
			name:               "ubuntu-2204-lts",
			image:              "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
			packageName:        "gcsfuse",
			wantPackageManager: "deb",
			repository:         aptPackageRepo("gcsfuse-jammy"),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		{
			name:               "ubuntu-2404-lts",
			image:              "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
			packageName:        "gcsfuse",
			wantPackageManager: "deb",
			repository:         aptPackageRepo("gcsfuse-noble"),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		// Enterprise Linux (RHEL, CentOS Stream, Rocky Linux)
		{
			name:               "rhel-8",
			image:              "projects/rhel-cloud/global/images/family/rhel-8",
			packageName:        "gcsfuse",
			wantPackageManager: "rpm",
			repository:         yumPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		{
			name:               "rhel-9",
			image:              "projects/rhel-cloud/global/images/family/rhel-9",
			packageName:        "gcsfuse",
			wantPackageManager: "rpm",
			repository:         yumPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		{
			name:               "centos-stream-9",
			image:              "projects/centos-cloud/global/images/family/centos-stream-9",
			packageName:        "gcsfuse",
			wantPackageManager: "rpm",
			repository:         yumPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		{
			name:               "rocky-linux-8",
			image:              "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
			packageName:        "gcsfuse",
			wantPackageManager: "rpm",
			repository:         yumPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		{
			name:               "rocky-linux-9",
			image:              "projects/rocky-linux-cloud/global/images/family/rocky-linux-9",
			packageName:        "gcsfuse",
			wantPackageManager: "rpm",
			repository:         yumPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		// SUSE / openSUSE
		{
			name:               "sles-12",
			image:              "projects/suse-cloud/global/images/family/sles-12",
			packageName:        "gcsfuse",
			wantPackageManager: "zypper",
			repository:         zypperPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		{
			name:               "sles-15",
			image:              "projects/suse-cloud/global/images/family/sles-15",
			packageName:        "gcsfuse",
			wantPackageManager: "zypper",
			repository:         zypperPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		{
			name:               "opensuse-leap-15",
			image:              "projects/opensuse-cloud/global/images/family/opensuse-leap",
			packageName:        "gcsfuse",
			wantPackageManager: "zypper",
			repository:         zypperPackageRepo(),
			machineType:        "e2-standard-2",
			timeout:            15 * time.Minute,
		},
		// Windows Server
		{
			name:               "windows-2016",
			image:              "projects/windows-cloud/global/images/family/windows-2016",
			packageName:        "google-cloud-sap-agent",
			wantPackageManager: "googet",
			repository:         gooPackageRepo(),
			machineType:        "e2-standard-4",
			timeout:            25 * time.Minute,
		},
		{
			name:               "windows-2019",
			image:              "projects/windows-cloud/global/images/family/windows-2019",
			packageName:        "google-cloud-sap-agent",
			wantPackageManager: "googet",
			repository:         gooPackageRepo(),
			machineType:        "e2-standard-4",
			timeout:            25 * time.Minute,
		},
		{
			name:               "windows-2022",
			image:              "projects/windows-cloud/global/images/family/windows-2022",
			packageName:        "google-cloud-sap-agent",
			wantPackageManager: "googet",
			repository:         gooPackageRepo(),
			machineType:        "e2-standard-4",
			timeout:            25 * time.Minute,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, map[string]string{
					"osconfig-disabled-features": "tasks",
				})
				return err
			})

			test.Step("create guest policy with package repository", func(ctx context.Context) error {
				policy := &osconfigv1beta.GuestPolicy{
					Assignment: &osconfigv1beta.Assignment{
						InstanceNamePrefixes: []string{vm.Name},
					},
					Packages: []*osconfigv1beta.Package{
						{
							Name:         tc.packageName,
							DesiredState: "UPDATED",
						},
					},
					PackageRepositories: []*osconfigv1beta.PackageRepository{
						tc.repository,
					},
				}
				_, err := test.CreateGuestPolicy(policy)
				return err
			})

			test.Step("verify package installed from new repository", func(ctx context.Context) error {
				_, err := test.WaitForPackageInstalled(vm, tc.wantPackageManager, tc.packageName)
				return err
			})
		})
	}
}
