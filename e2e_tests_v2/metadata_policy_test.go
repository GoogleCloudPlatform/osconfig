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

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
)

// TestMetadataPolicy verifies that the OS Config agent applies a configuration defined in instance metadata.
func TestMetadataPolicy(t *testing.T) {
	tests := softwareRecipeTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			test := testenv.New(t, tt.timeout)

			var vm *gcp.VM
			test.Step("create VM", func(ctx context.Context) error {
				scriptKey, scriptContent := gcp.RecipeInstallStartupScript(tt.image, defaultRecipeName)
				var err error
				vm, err = createRecipeTestVM(test, tt, scriptKey, scriptContent)
				return err
			})

			test.Step("wait for agent install", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributeInstallDone)
			})

			test.Step("apply metadata policy and restart agent", func(ctx context.Context) error {
				mdJSON, err := testenv.BuildMetadataPolicyJSON(defaultRecipeName)
				if err != nil {
					return err
				}
				return test.AddMetadata(vm, map[string]string{
					"gce-software-declaration":  mdJSON,
					gcp.MetadataKeyRestartAgent: "true",
				})
			})

			test.Step("verify recipe installation", func(ctx context.Context) error {
				return test.WaitForGuestAttribute(vm, gcp.GuestAttributeRecipeInstalled)
			})
		})
	}
}
