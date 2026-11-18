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
	osconfigv1 "google.golang.org/api/osconfig/v1"
	osconfigv1beta "google.golang.org/api/osconfig/v1beta"
)

const (
	debOldGcsfuseVersion      = "2.3.2"
	rpmOldGcsfuseVersion      = "2.3.2-1"
	windowsOldSapAgentVersion = "3.15@952132455"
)

type guestPolicyOSTarget struct {
	name               string
	image              string
	packageName        string
	initialVersion     string
	wantPackageManager string
	aptDistribution    string
	repository         *osconfigv1beta.PackageRepository
	machineType        string
	timeout            time.Duration
}

var guestPolicyOSTargets = []guestPolicyOSTarget{
	// Debian
	{
		name:               "debian-12",
		image:              "projects/debian-cloud/global/images/family/debian-12",
		packageName:        "gcsfuse",
		initialVersion:     debOldGcsfuseVersion,
		wantPackageManager: "deb",
		aptDistribution:    "gcsfuse-bookworm",
		repository:         aptPackageRepo("gcsfuse-bookworm"),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "debian-13",
		image:              "projects/debian-cloud/global/images/family/debian-13",
		packageName:        "gcsfuse",
		initialVersion:     debOldGcsfuseVersion,
		wantPackageManager: "deb",
		aptDistribution:    "gcsfuse-trixie",
		repository:         aptPackageRepo("gcsfuse-trixie"),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	// Ubuntu
	{
		name:               "ubuntu-2204-lts",
		image:              "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts",
		packageName:        "gcsfuse",
		initialVersion:     debOldGcsfuseVersion,
		wantPackageManager: "deb",
		aptDistribution:    "gcsfuse-jammy",
		repository:         aptPackageRepo("gcsfuse-jammy"),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "ubuntu-2404-lts",
		image:              "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64",
		packageName:        "gcsfuse",
		initialVersion:     debOldGcsfuseVersion,
		wantPackageManager: "deb",
		aptDistribution:    "gcsfuse-noble",
		repository:         aptPackageRepo("gcsfuse-noble"),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	// Enterprise Linux (RHEL, CentOS Stream, Rocky Linux)
	{
		name:               "rhel-8",
		image:              "projects/rhel-cloud/global/images/family/rhel-8",
		packageName:        "gcsfuse",
		initialVersion:     rpmOldGcsfuseVersion,
		wantPackageManager: "rpm",
		repository:         yumPackageRepo(),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "rhel-9",
		image:              "projects/rhel-cloud/global/images/family/rhel-9",
		packageName:        "gcsfuse",
		initialVersion:     rpmOldGcsfuseVersion,
		wantPackageManager: "rpm",
		repository:         yumPackageRepo(),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "centos-stream-9",
		image:              "projects/centos-cloud/global/images/family/centos-stream-9",
		packageName:        "gcsfuse",
		initialVersion:     rpmOldGcsfuseVersion,
		wantPackageManager: "rpm",
		repository:         yumPackageRepo(),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "rocky-linux-8",
		image:              "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
		packageName:        "gcsfuse",
		initialVersion:     rpmOldGcsfuseVersion,
		wantPackageManager: "rpm",
		repository:         yumPackageRepo(),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "rocky-linux-9",
		image:              "projects/rocky-linux-cloud/global/images/family/rocky-linux-9",
		packageName:        "gcsfuse",
		initialVersion:     rpmOldGcsfuseVersion,
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
		initialVersion:     rpmOldGcsfuseVersion,
		wantPackageManager: "zypper",
		repository:         zypperPackageRepo(),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "sles-15",
		image:              "projects/suse-cloud/global/images/family/sles-15",
		packageName:        "gcsfuse",
		initialVersion:     rpmOldGcsfuseVersion,
		wantPackageManager: "zypper",
		repository:         zypperPackageRepo(),
		machineType:        "e2-standard-2",
		timeout:            15 * time.Minute,
	},
	{
		name:               "opensuse-leap-15",
		image:              "projects/opensuse-cloud/global/images/family/opensuse-leap",
		packageName:        "gcsfuse",
		initialVersion:     rpmOldGcsfuseVersion,
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
		initialVersion:     windowsOldSapAgentVersion,
		wantPackageManager: "googet",
		repository:         gooPackageRepo(),
		machineType:        "e2-standard-4",
		timeout:            25 * time.Minute,
	},
	{
		name:               "windows-2019",
		image:              "projects/windows-cloud/global/images/family/windows-2019",
		packageName:        "google-cloud-sap-agent",
		initialVersion:     windowsOldSapAgentVersion,
		wantPackageManager: "googet",
		repository:         gooPackageRepo(),
		machineType:        "e2-standard-4",
		timeout:            25 * time.Minute,
	},
	{
		name:               "windows-2022",
		image:              "projects/windows-cloud/global/images/family/windows-2022",
		packageName:        "google-cloud-sap-agent",
		initialVersion:     windowsOldSapAgentVersion,
		wantPackageManager: "googet",
		repository:         gooPackageRepo(),
		machineType:        "e2-standard-4",
		timeout:            25 * time.Minute,
	},
}

func guestPolicyMetadataWithPreScript(image, preScript string) map[string]string {
	metadata := map[string]string{
		"osconfig-disabled-features": "tasks",
	}
	if preScript == "" {
		return metadata
	}
	key, script := gcp.StartupScript(image, preScript, "")
	metadata[key] = script
	return metadata
}

func installOldPackagePreScript(tc guestPolicyOSTarget) string {
	switch tc.wantPackageManager {
	case "deb":
		return fmt.Sprintf(`systemctl stop google-osconfig-agent || true
while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do sleep 2; done
echo "deb [trusted=yes] %s %s main" > /etc/apt/sources.list.d/gcsfuse-preinstall.list
apt-get update
apt-get remove -y %s || true
apt-get install -y --allow-downgrades %s=%s
rm -f /etc/apt/sources.list.d/gcsfuse-preinstall.list`,
			aptTestRepoBaseURL, tc.aptDistribution, tc.packageName, tc.packageName, tc.initialVersion)

	case "rpm":
		return fmt.Sprintf(`systemctl stop google-osconfig-agent || true
cat > /etc/yum.repos.d/gcsfuse-preinstall.repo <<EOM
[gcsfuse-preinstall]
name=gcsfuse-preinstall
baseurl=%s
enabled=1
gpgcheck=0
EOM
dnf remove -y %[2]s || yum remove -y %[2]s || true
dnf install -y --nogpgcheck %[2]s-%[3]s || yum install -y --nogpgcheck %[2]s-%[3]s
rm -f /etc/yum.repos.d/gcsfuse-preinstall.repo`,
			gcsfuseYumRepoBaseURL, tc.packageName, tc.initialVersion)

	case "zypper":
		return fmt.Sprintf(`systemctl stop google-osconfig-agent || true
zypper -n remove %s || true
zypper ar --no-gpgcheck -f "%s" "gcsfuse-preinstall"
zypper --no-gpg-checks refresh
zypper --no-gpg-checks -n install --oldpackage %s-%s
zypper rr "gcsfuse-preinstall" || true`,
			tc.packageName, gcsfuseYumRepoBaseURL, tc.packageName, tc.initialVersion)

	case "googet":
		return fmt.Sprintf(`Stop-Service google_osconfig_agent -ErrorAction SilentlyContinue
c:\programdata\googet\googet.exe addrepo sap-preinstall %s
c:\programdata\googet\googet.exe -noconfirm remove %s
c:\programdata\googet\googet.exe -noconfirm install %s.x86_64.%s
c:\programdata\googet\googet.exe rmrepo sap-preinstall`,
			gooTestRepoURL, tc.packageName, tc.packageName, tc.initialVersion)

	default:
		return ""
	}
}

func createPackageGuestPolicy(vm *gcp.VM, packageName, desiredState string, repo *osconfigv1beta.PackageRepository) *osconfigv1beta.GuestPolicy {
	policy := &osconfigv1beta.GuestPolicy{
		Assignment: &osconfigv1beta.Assignment{
			Instances: []string{fmt.Sprintf("zones/%s/instances/%s", vm.Zone, vm.Name)},
		},
		Packages: []*osconfigv1beta.Package{
			{
				Name:         packageName,
				DesiredState: desiredState,
			},
		},
	}
	if repo != nil {
		policy.PackageRepositories = []*osconfigv1beta.PackageRepository{repo}
	}
	return policy
}

// TestGuestPoliciesPackageInstallDoesNotUpdate verifies that a package with DesiredState: INSTALLED
// does not overwrite or update an older version that is already installed on the VM.
func TestGuestPoliciesPackageInstallDoesNotUpdate(t *testing.T) {
	for _, tc := range guestPolicyOSTargets {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM with pre-installed older package version", func(ctx context.Context) error {
				var err error
				preScript := installOldPackagePreScript(tc)
				vm, err = test.CreateVM(tc.image, tc.machineType, guestPolicyMetadataWithPreScript(tc.image, preScript))
				return err
			})

			test.Step("verify initial old package version in inventory", func(ctx context.Context) error {
				_, err := test.WaitForPackageVersion(vm, tc.wantPackageManager, tc.packageName, tc.initialVersion)
				return err
			})

			test.Step("apply INSTALLED policy and verify old version kept", func(ctx context.Context) error {
				policy := createPackageGuestPolicy(vm, tc.packageName, "INSTALLED", tc.repository)
				if err := test.CreateGuestPolicy(vm, policy); err != nil {
					return err
				}
				// Wait one agent poll cycle (~65s) to allow GuestPolicy evaluation, then verify version did not change.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(65 * time.Second):
				}
				_, err := test.WaitForPackageVersion(vm, tc.wantPackageManager, tc.packageName, tc.initialVersion)
				return err
			})
		})
	}
}

// TestGuestPoliciesPackageUpdate verifies that a package with DesiredState: UPDATED
// upgrades an existing older version to the newer version available in the repository.
func TestGuestPoliciesPackageUpdate(t *testing.T) {
	for _, tc := range guestPolicyOSTargets {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM with pre-installed older package version", func(ctx context.Context) error {
				var err error
				preScript := installOldPackagePreScript(tc)
				vm, err = test.CreateVM(tc.image, tc.machineType, guestPolicyMetadataWithPreScript(tc.image, preScript))
				return err
			})

			test.Step("verify initial old package version in inventory", func(ctx context.Context) error {
				_, err := test.WaitForPackageVersion(vm, tc.wantPackageManager, tc.packageName, tc.initialVersion)
				return err
			})

			test.Step("apply UPDATED policy and verify version upgraded", func(ctx context.Context) error {
				policy := createPackageGuestPolicy(vm, tc.packageName, "UPDATED", tc.repository)
				if err := test.CreateGuestPolicy(vm, policy); err != nil {
					return err
				}
				_, err := test.WaitForPackageVersionChanged(vm, tc.wantPackageManager, tc.packageName, tc.initialVersion)
				return err
			})
		})
	}
}

// TestGuestPoliciesPackageRemoval verifies that a package with DesiredState: REMOVED
// uninstalls an existing package from the VM.
func TestGuestPoliciesPackageRemoval(t *testing.T) {
	for _, tc := range guestPolicyOSTargets {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tc.timeout)

			var vm *gcp.VM
			test.Step("create VM with pre-installed package", func(ctx context.Context) error {
				var err error
				preScript := installOldPackagePreScript(tc)
				vm, err = test.CreateVM(tc.image, tc.machineType, guestPolicyMetadataWithPreScript(tc.image, preScript))
				return err
			})

			test.Step("verify package is installed initially", func(ctx context.Context) error {
				_, err := test.WaitForPackageVersion(vm, tc.wantPackageManager, tc.packageName, tc.initialVersion)
				return err
			})

			test.Step("apply REMOVED policy and verify package uninstalled", func(ctx context.Context) error {
				policy := createPackageGuestPolicy(vm, tc.packageName, "REMOVED", nil)
				if err := test.CreateGuestPolicy(vm, policy); err != nil {
					return err
				}
				return test.WaitForPackages(vm, nil, []string{tc.packageName})
			})
		})
	}
}

// TestGuestPoliciesPackageInstallation verifies that a package with DesiredState: INSTALLED
// is installed on a VM where it was not previously present.
func TestGuestPoliciesPackageInstallation(t *testing.T) {
	for _, tc := range guestPolicyOSTargets {
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

			var inv *osconfigv1.Inventory
			test.Step("wait for OS Config agent ready", func(ctx context.Context) error {
				var err error
				inv, err = test.WaitForInventory(vm)
				return err
			})

			test.Step("verify package is not installed initially", func(ctx context.Context) error {
				pkgs := testenv.ExtractInstalledPackages(inv)
				if pkgs != nil && pkgs.HasPackageIn(tc.wantPackageManager, tc.packageName) {
					return fmt.Errorf("package %q is already installed under %s before creating guest policy", tc.packageName, tc.wantPackageManager)
				}
				return nil
			})

			test.Step("apply INSTALLED policy and verify package installed", func(ctx context.Context) error {
				policy := createPackageGuestPolicy(vm, tc.packageName, "INSTALLED", tc.repository)
				if err := test.CreateGuestPolicy(vm, policy); err != nil {
					return err
				}
				return test.WaitForPackageInManager(vm, tc.wantPackageManager, tc.packageName)
			})
		})
	}
}
