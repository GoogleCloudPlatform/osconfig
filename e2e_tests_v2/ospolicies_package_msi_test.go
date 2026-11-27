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
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
	"google.golang.org/api/osconfig/v1"
)

const (
	msiTestBucket     = "osconfig-agent-end2end-test-resources"
	msiTestObject     = "OSPolicies/googlechromestandaloneenterprise64.msi"
	msiTestGeneration = 1720172515592258
	msiTestRemoteURI  = "https://storage.googleapis.com/osconfig-agent-end2end-test-resources/OSPolicies/googlechromestandaloneenterprise64.msi"
	msiTestSHA256     = "1c039398595da512b3b37c202ac8b4d0e04fe3f5e64b1acf81559a42154331f2"
	msiTestLocalPath  = `C:\temp\test.msi`
)

const downloadLocalMsiScript = `$svc = Get-Service google_osconfig_agent -ErrorAction SilentlyContinue
if ($svc -and $svc.Status -eq 'Running') {
    Stop-Service google_osconfig_agent -Force -ErrorAction SilentlyContinue
}
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
New-Item -ItemType Directory -Force -Path 'C:\temp' | Out-Null
$tmpPath = '` + msiTestLocalPath + `.tmp'
(New-Object System.Net.WebClient).DownloadFile('` + msiTestRemoteURI + `', $tmpPath)
Move-Item -Force $tmpPath '` + msiTestLocalPath + `'`

const preinstallLocalMsiScript = downloadLocalMsiScript + `
$proc = Start-Process msiexec.exe -ArgumentList '/i', '` + msiTestLocalPath + `', '/qn', '/norestart' -Wait -NoNewWindow -PassThru
Write-Output "Pre-install msiexec exit code: $($proc.ExitCode)"`

type ospoliciesPackageMsiTestCase struct {
	name        string
	image       string
	machineType string
	timeout     time.Duration
}

type ospoliciesPackageMsiScenario struct {
	name           string
	mode           string
	preAgentScript string
	msi            *osconfig.OSPolicyResourcePackageResourceMSI
}

// osPolicyPackageMsiTestCases returns the list of Windows images for MSI package policy tests.
func osPolicyPackageMsiTestCases() []ospoliciesPackageMsiTestCase {
	return []ospoliciesPackageMsiTestCase{
		// Windows Server
		{
			name:        "windows-2016",
			image:       "projects/windows-cloud/global/images/family/windows-2016",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		{
			name:        "windows-2019",
			image:       "projects/windows-cloud/global/images/family/windows-2019",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
		{
			name:        "windows-2022",
			image:       "projects/windows-cloud/global/images/family/windows-2022",
			machineType: "e2-standard-4",
			timeout:     25 * time.Minute,
		},
	}
}

// osPolicyPackageMsiScenarios returns the MSI source, properties, and compliance scenarios to test.
func osPolicyPackageMsiScenarios() []ospoliciesPackageMsiScenario {
	return []ospoliciesPackageMsiScenario{
		{
			name: "gcs",
			mode: "ENFORCEMENT",
			msi: &osconfig.OSPolicyResourcePackageResourceMSI{
				Source: &osconfig.OSPolicyResourceFile{
					Gcs: &osconfig.OSPolicyResourceFileGcs{
						Bucket:     msiTestBucket,
						Object:     msiTestObject,
						Generation: msiTestGeneration,
					},
				},
			},
		},
		{
			name: "remote_uri_with_properties",
			mode: "ENFORCEMENT",
			msi: &osconfig.OSPolicyResourcePackageResourceMSI{
				Source: &osconfig.OSPolicyResourceFile{
					Remote: &osconfig.OSPolicyResourceFileRemote{
						Uri:            msiTestRemoteURI,
						Sha256Checksum: msiTestSHA256,
					},
				},
				Properties: []string{
					`INSTALLFOLDER=C:\CustomMsiPath`,
					"ALLUSERS=1",
				},
			},
		},
		{
			name:           "local_path",
			mode:           "ENFORCEMENT",
			preAgentScript: downloadLocalMsiScript,
			msi: &osconfig.OSPolicyResourcePackageResourceMSI{
				Source: &osconfig.OSPolicyResourceFile{
					LocalPath: msiTestLocalPath,
				},
			},
		},
		{
			name:           "already_installed_validation",
			mode:           "VALIDATION",
			preAgentScript: preinstallLocalMsiScript,
			msi: &osconfig.OSPolicyResourcePackageResourceMSI{
				Source: &osconfig.OSPolicyResourceFile{
					LocalPath: msiTestLocalPath,
				},
			},
		},
	}
}

// buildMsiPackageOSPolicyAssignment constructs an OSPolicyAssignment for the given MSI scenario.
func buildMsiPackageOSPolicyAssignment(vmName, mode string, msi *osconfig.OSPolicyResourcePackageResourceMSI) *osconfig.OSPolicyAssignment {
	return &osconfig.OSPolicyAssignment{
		InstanceFilter: &osconfig.OSPolicyAssignmentInstanceFilter{
			InclusionLabels: []*osconfig.OSPolicyAssignmentLabelSet{
				{
					Labels: map[string]string{"name": vmName},
				},
			},
		},
		Rollout: &osconfig.OSPolicyAssignmentRollout{
			DisruptionBudget: &osconfig.FixedOrPercent{Percent: 100},
			MinWaitDuration:  "0s",
		},
		OsPolicies: []*osconfig.OSPolicy{
			{
				Id:   "packageresourcemsi",
				Mode: mode,
				ResourceGroups: []*osconfig.OSPolicyResourceGroup{
					{
						Resources: []*osconfig.OSPolicyResource{
							{
								Id: "install-package",
								Pkg: &osconfig.OSPolicyResourcePackageResource{
									DesiredState: "INSTALLED",
									Msi:          msi,
								},
							},
						},
					},
				},
			},
		},
	}
}

// isMsiValidationReportCompliant verifies that the report shows COMPLIANT with only VALIDATION and DESIRED_STATE_CHECK steps.
func isMsiValidationReportCompliant(rep *osconfig.OSPolicyAssignmentReport) (string, bool) {
	if rep == nil || len(rep.OsPolicyCompliances) != 1 {
		got := 0
		if rep != nil {
			got = len(rep.OsPolicyCompliances)
		}
		return fmt.Sprintf("waiting for 1 os policy compliance, got %d", got), false
	}

	comp := rep.OsPolicyCompliances[0]
	if comp.ComplianceState != "COMPLIANT" {
		return fmt.Sprintf("os policy %q in state %s, want COMPLIANT", comp.OsPolicyId, comp.ComplianceState), false
	}
	if len(comp.OsPolicyResourceCompliances) != 1 {
		return fmt.Sprintf("waiting for 1 resource compliance, got %d", len(comp.OsPolicyResourceCompliances)), false
	}

	res := comp.OsPolicyResourceCompliances[0]
	if res.ComplianceState != "COMPLIANT" {
		return fmt.Sprintf("resource %q in state %s, want COMPLIANT", res.OsPolicyResourceId, res.ComplianceState), false
	}
	if len(res.ConfigSteps) != 2 {
		return fmt.Sprintf("expected 2 config steps (VALIDATION, DESIRED_STATE_CHECK), got %d", len(res.ConfigSteps)), false
	}
	if res.ConfigSteps[0].Type != "VALIDATION" || res.ConfigSteps[1].Type != "DESIRED_STATE_CHECK" {
		return fmt.Sprintf("unexpected config steps: [%s, %s]", res.ConfigSteps[0].Type, res.ConfigSteps[1].Type), false
	}

	return "validation mode COMPLIANT report verified without enforcement", true
}

// waitForMsiValidationCompliant polls the OS policy assignment report until VALIDATION mode reports COMPLIANT without enforcement.
func waitForMsiValidationCompliant(ctx context.Context, test *testenv.Test, vm *gcp.VM) error {
	assignmentID := vm.Name
	return gcp.PollUntil(ctx, test.Suite.Config.PollInterval, fmt.Sprintf("msi validation report for %s on %s", assignmentID, vm.Name), func(ctx context.Context) (string, bool, error) {
		rep, err := test.Suite.Compute.GetOSPolicyAssignmentReport(ctx, vm.Project, vm.Zone, vm.Name, assignmentID)
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("report not yet available (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read os policy assignment report: %w", err)
		}

		msg, ok := isMsiValidationReportCompliant(rep)
		return msg, ok, nil
	})
}

// waitForWindowsApplication polls OS Config Inventory until an application matching appName is reported.
func waitForWindowsApplication(ctx context.Context, test *testenv.Test, vm *gcp.VM, appName string) error {
	return gcp.PollUntil(ctx, test.Suite.Config.PollInterval, fmt.Sprintf("windows application %s on %s", appName, vm.Name), func(ctx context.Context) (string, bool, error) {
		inv, err := test.Suite.Compute.GetInventory(ctx, vm.Project, vm.Zone, vm.Name, "FULL")
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("inventory not yet published (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("failed to read inventory: %w", err)
		}
		for _, item := range inv.Items {
			if item.InstalledPackage != nil && item.InstalledPackage.WindowsApplication != nil {
				if strings.Contains(strings.ToLower(item.InstalledPackage.WindowsApplication.DisplayName), strings.ToLower(appName)) {
					return fmt.Sprintf("found application %q", item.InstalledPackage.WindowsApplication.DisplayName), true, nil
				}
			}
		}
		return fmt.Sprintf("waiting for application %q in inventory", appName), false, nil
	})
}

// TestOSPolicyPackageResourceMsi verifies that the OS Config agent installs and validates MSI packages
// from GCS, remote HTTPS URI with SHA256 checksum and custom properties, local file path, and pre-installed state.
func TestOSPolicyPackageResourceMsi(t *testing.T) {
	const wantCompliances = 1
	scenarios := osPolicyPackageMsiScenarios()
	tests := osPolicyPackageMsiTestCases()

	for _, sc := range scenarios {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s/%s", sc.name, tt.name), func(t *testing.T) {
				t.Parallel()

				test := testenv.New(t, tt.timeout)

				var vm *gcp.VM
				test.Step("create VM", func(ctx context.Context) error {
					customMetadata := map[string]string{
						"osconfig-disabled-features": "guestpolicies",
					}
					if sc.preAgentScript != "" {
						scriptKey, scriptContent := gcp.StartupScript(tt.image, sc.preAgentScript, "")
						customMetadata[scriptKey] = scriptContent
					}
					var err error
					vm, err = test.CreateVM(tt.image, tt.machineType, customMetadata)
					return err
				})

				test.Step("wait for agent inventory", func(ctx context.Context) error {
					if sc.mode == "VALIDATION" {
						return waitForWindowsApplication(ctx, test, vm, "Google Chrome")
					}
					_, err := test.WaitForInventory(vm)
					return err
				})

				test.Step("create os policy assignment", func(ctx context.Context) error {
					return test.CreateOSPolicyAssignment(vm, buildMsiPackageOSPolicyAssignment(vm.Name, sc.mode, sc.msi))
				})

				test.Step("wait for os policy compliance report", func(ctx context.Context) error {
					if sc.mode == "VALIDATION" {
						return waitForMsiValidationCompliant(ctx, test, vm)
					}
					return test.WaitForOSPolicyCompliance(vm, wantCompliances)
				})

				test.Step("verify package installed via inventory", func(ctx context.Context) error {
					return waitForWindowsApplication(ctx, test, vm, "Google Chrome")
				})
			})
		}
	}
}
