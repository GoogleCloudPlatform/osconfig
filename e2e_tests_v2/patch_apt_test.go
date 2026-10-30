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
	"fmt"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
	"google.golang.org/api/osconfig/v1"
)

var aptTestCases = []patchJobTestCase{
	// Debian
	{
		name:        "debian-12",
		image:       "projects/debian-cloud/global/images/family/debian-12",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	{
		name:        "debian-13",
		image:       "projects/debian-cloud/global/images/family/debian-13",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	// Ubuntu
	{
		name:        "ubuntu-2204-lts",
		image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
	{
		name:        "ubuntu-2404-lts",
		image:       "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
		machineType: "e2-standard-2",
		timeout:     35 * time.Minute,
	},
}

// TestOSPatchAptExcludes verifies that APT patching with dist-upgrade and package excludes succeeds.
// This migrates "[APT dist-upgrade, excludes]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchAptExcludes(t *testing.T) {
	for _, tc := range aptTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.extraMeta {
					meta[k] = v
				}
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, meta)
				return err
			})

			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job with APT excludes", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e apt excludes patch job test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
						Apt: &osconfig.AptSettings{
							Type:     "DIST",
							Excludes: []string{"pkg1", "/pkg2/"},
						},
					},
					Duration: fmt.Sprintf("%ds", int(tc.timeout.Seconds())),
				}
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				_, err = test.WaitForPatchJob(job.Name)
				return err
			})
		})
	}
}

// TestOSPatchAptExclusivePackages verifies that APT patching with dist-upgrade and exclusive packages succeeds.
// This migrates "[APT dist-upgrade, exclusive packages]" from e2e_tests/test_suites/patch/patch.go.
func TestOSPatchAptExclusivePackages(t *testing.T) {
	for _, tc := range aptTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.extraMeta {
					meta[k] = v
				}
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, meta)
				return err
			})

			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job with APT exclusive packages", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e apt exclusive packages patch job test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
						Apt: &osconfig.AptSettings{
							Type:              "DIST",
							ExclusivePackages: []string{"pkg1"},
						},
					},
					Duration: fmt.Sprintf("%ds", int(tc.timeout.Seconds())),
				}
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				_, err = test.WaitForPatchJob(job.Name)
				return err
			})
		})
	}
}

func aptDowngradeStartupScript() string {
	return `#!/bin/bash
mkdir -p /tmp/pkg/DEBIAN
cat <<'EOF' > /tmp/pkg/DEBIAN/control
Package: osconfig-test-downgrade
Version: 2.0
Architecture: all
Maintainer: test
Description: test
EOF
chmod 0755 /tmp/pkg/DEBIAN
dpkg-deb --root-owner-group -b /tmp/pkg /tmp/pkg_2.0.deb && dpkg -i /tmp/pkg_2.0.deb
sed -i 's/2.0/1.0/' /tmp/pkg/DEBIAN/control
mkdir -p /var/local/apt-repo
dpkg-deb --root-owner-group -b /tmp/pkg /var/local/apt-repo/pkg_1.0.deb
size=$(stat -c%s /var/local/apt-repo/pkg_1.0.deb)
md5=$(md5sum /var/local/apt-repo/pkg_1.0.deb | cut -d' ' -f1)
sha256=$(sha256sum /var/local/apt-repo/pkg_1.0.deb | cut -d' ' -f1)
dpkg-deb -I /var/local/apt-repo/pkg_1.0.deb control > /var/local/apt-repo/Packages
cat <<EOF >> /var/local/apt-repo/Packages
Filename: ./pkg_1.0.deb
Size: $size
MD5sum: $md5
SHA256: $sha256

EOF
echo "deb [trusted=yes] file:/var/local/apt-repo ./" > /etc/apt/sources.list.d/local.list
cat <<'EOF' > /etc/apt/preferences.d/downgrade
Package: osconfig-test-downgrade
Pin: version 1.0
Pin-priority: 9999
EOF
apt-get update || true
`
}

// TestOSPatchAptDowngrade verifies that APT dist-upgrade patching succeeds even when a package needs to be downgraded.
// It sets up a local repository with a pinned older package version to trigger the APT downgrade condition
// ("E: Packages were downgraded and -y was used without --allow-downgrades.") and verifies that the agent
// automatically retries with --allow-downgrades.
func TestOSPatchAptDowngrade(t *testing.T) {
	testCases := []patchJobTestCase{
		{
			name:        "debian-12",
			image:       "projects/debian-cloud/global/images/family/debian-12",
			machineType: "e2-standard-2",
			timeout:     35 * time.Minute,
			extraMeta: map[string]string{
				gcp.MetadataKeyLinuxStartupScript: aptDowngradeStartupScript(),
			},
		},
		{
			name:        "debian-13",
			image:       "projects/debian-cloud/global/images/family/debian-13",
			machineType: "e2-standard-2",
			timeout:     35 * time.Minute,
			extraMeta: map[string]string{
				gcp.MetadataKeyLinuxStartupScript: aptDowngradeStartupScript(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				meta := map[string]string{
					"osconfig-disabled-features": "guestpolicies",
				}
				for k, v := range tc.extraMeta {
					meta[k] = v
				}
				var err error
				vm, err = test.CreateVM(tc.image, tc.machineType, meta)
				return err
			})

			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				_, err := test.WaitForInventory(vm)
				return err
			})

			test.Step("execute and await patch job with APT downgrade", func(ctx context.Context) error {
				req := &osconfig.ExecutePatchJobRequest{
					Description: fmt.Sprintf("e2e apt downgrade patch job test for %s", vm.Name),
					InstanceFilter: &osconfig.PatchInstanceFilter{
						Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
					},
					PatchConfig: &osconfig.PatchConfig{
						RebootConfig: "DEFAULT",
						Apt: &osconfig.AptSettings{
							Type: "DIST",
						},
					},
					Duration: fmt.Sprintf("%ds", int(tc.timeout.Seconds())),
				}
				job, err := test.ExecutePatchJob(req)
				if err != nil {
					return err
				}
				_, err = test.WaitForPatchJob(job.Name)
				return err
			})
		})
	}
}
