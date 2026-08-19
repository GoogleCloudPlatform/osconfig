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
	"fmt"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/testenv"
)

func TestMain(m *testing.M) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "E2E configuration error: %v\n", err)
		os.Exit(2)
	}

	if err := testenv.Init(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "E2E initialization error: %v\n", err)
		os.Exit(2)
	}

	code := m.Run()

	if err := testenv.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JUnit report: %v\n", err)
	}

	os.Exit(code)
}
