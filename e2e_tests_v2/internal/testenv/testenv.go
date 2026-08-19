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

// Package testenv manages test execution lifecycle, VM provisioning, cleanup, and JUnit reporting.
package testenv

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/config"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/gcp"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/junitxml"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/scheduler"
	"github.com/google/uuid"
	"google.golang.org/api/osconfig/v1"
)

var (
	unsafeResourceName = regexp.MustCompile(`[^a-z0-9-]+`)

	globalMu      sync.Mutex
	globalSuite   *Suite
	globalInitErr error
)

// InstalledPackages represents packages collected by the agent and exported to guest attributes.
type InstalledPackages struct {
	Yum    []*PackageInfo `json:"yum,omitempty"`
	Rpm    []*PackageInfo `json:"rpm,omitempty"`
	Apt    []*PackageInfo `json:"apt,omitempty"`
	Deb    []*PackageInfo `json:"deb,omitempty"`
	Zypper []*PackageInfo `json:"zypper,omitempty"`
	COS    []*PackageInfo `json:"cos,omitempty"`
	Gem    []*PackageInfo `json:"gem,omitempty"`
	Pip    []*PackageInfo `json:"pip,omitempty"`
	GooGet []*PackageInfo `json:"googet,omitempty"`
	WUA    []*WUAPackage  `json:"wua,omitempty"`
	QFE    []*QFEPackage  `json:"qfe,omitempty"`
}

// PackageTypes returns a sorted list of all package manager types that have at least one installed package.
func (pkgs *InstalledPackages) PackageTypes() []string {
	if pkgs == nil {
		return nil
	}
	var types []string
	if len(pkgs.Deb) > 0 || len(pkgs.Apt) > 0 {
		types = append(types, "deb")
	}
	if len(pkgs.Rpm) > 0 || len(pkgs.Yum) > 0 {
		types = append(types, "rpm")
	}
	if len(pkgs.Zypper) > 0 {
		types = append(types, "zypper")
	}
	if len(pkgs.COS) > 0 {
		types = append(types, "cos")
	}
	if len(pkgs.GooGet) > 0 {
		types = append(types, "googet")
	}
	if len(pkgs.WUA) > 0 {
		types = append(types, "wua")
	}
	if len(pkgs.QFE) > 0 {
		types = append(types, "qfe")
	}
	if len(pkgs.Pip) > 0 {
		types = append(types, "pip")
	}
	if len(pkgs.Gem) > 0 {
		types = append(types, "gem")
	}
	sort.Strings(types)
	return types
}

// OSPackageTypes returns a sorted list of OS-level package manager types that have at least one installed package.
func (pkgs *InstalledPackages) OSPackageTypes() []string {
	if pkgs == nil {
		return nil
	}
	var types []string
	if len(pkgs.Deb) > 0 || len(pkgs.Apt) > 0 {
		types = append(types, "deb")
	}
	if len(pkgs.Rpm) > 0 || len(pkgs.Yum) > 0 {
		types = append(types, "rpm")
	}
	if len(pkgs.Zypper) > 0 {
		types = append(types, "zypper")
	}
	if len(pkgs.COS) > 0 {
		types = append(types, "cos")
	}
	if len(pkgs.GooGet) > 0 {
		types = append(types, "googet")
	}
	if len(pkgs.WUA) > 0 {
		types = append(types, "wua")
	}
	if len(pkgs.QFE) > 0 {
		types = append(types, "qfe")
	}
	sort.Strings(types)
	return types
}

// PackageInfo describes an installed package.
type PackageInfo struct {
	Name    string `json:"Name"`
	Arch    string `json:"Arch"`
	Version string `json:"Version"`
}

// WUAPackage describes a Windows Update Agent package.
type WUAPackage struct {
	Title string `json:"Title"`
}

// QFEPackage describes a Quick Fix Engineering package.
type QFEPackage struct {
	HotFixID string `json:"HotFixID"`
}

// HasPackages checks if all packages with the given names exist in any package list.
func (pkgs *InstalledPackages) HasPackages(names ...string) bool {
	if pkgs == nil {
		return false
	}
	if len(names) == 0 {
		return true
	}
	installed := make(map[string]bool)
	for _, pkgName := range pkgs.AllPackageNames() {
		installed[strings.ToLower(strings.TrimSpace(pkgName))] = true
	}
	for _, name := range names {
		target := strings.ToLower(strings.TrimSpace(name))
		if !installed[target] {
			return false
		}
	}
	return true
}

// HasPackage checks if a package with the given name exists in any package list.
func (pkgs *InstalledPackages) HasPackage(name string) bool {
	return pkgs.HasPackages(name)
}

// HasPackageIn checks if a package with the given name exists under a specific package manager.
func (pkgs *InstalledPackages) HasPackageIn(manager, name string) bool {
	if pkgs == nil {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(name))
	var list []*PackageInfo
	switch strings.ToLower(strings.TrimSpace(manager)) {
	case "deb":
		list = append(list, pkgs.Deb...)
		list = append(list, pkgs.Apt...)
	case "rpm":
		list = append(list, pkgs.Rpm...)
		list = append(list, pkgs.Yum...)
	case "zypper":
		list = pkgs.Zypper
	case "cos":
		list = pkgs.COS
	case "googet":
		list = pkgs.GooGet
	case "pip":
		list = pkgs.Pip
	case "gem":
		list = pkgs.Gem
	}

	for _, p := range list {
		if p != nil && strings.ToLower(strings.TrimSpace(p.Name)) == target {
			return true
		}
	}
	return false
}

// AllPackageNames returns all package names collected across all package managers.
func (pkgs *InstalledPackages) AllPackageNames() []string {
	if pkgs == nil {
		return nil
	}
	var names []string
	addNames := func(list []*PackageInfo) {
		for _, p := range list {
			if p != nil && p.Name != "" {
				names = append(names, p.Name)
			}
		}
	}

	addNames(pkgs.Deb)
	addNames(pkgs.Apt)
	addNames(pkgs.Rpm)
	addNames(pkgs.Yum)
	addNames(pkgs.Zypper)
	addNames(pkgs.COS)
	addNames(pkgs.GooGet)
	addNames(pkgs.Pip)
	addNames(pkgs.Gem)

	return names
}

// HasQFE returns true if there is at least one QFE package reported.
func (pkgs *InstalledPackages) HasQFE() bool {
	return pkgs != nil && len(pkgs.QFE) > 0
}

// HasWUA returns true if there is at least one WUA package reported.
func (pkgs *InstalledPackages) HasWUA() bool {
	return pkgs != nil && len(pkgs.WUA) > 0
}

// ExtractInstalledPackages extracts installed package information from an OS Config Inventory object.
func ExtractInstalledPackages(inv *osconfig.Inventory) *InstalledPackages {
	if inv == nil {
		return nil
	}
	pkgs := &InstalledPackages{}
	for _, item := range inv.Items {
		pkg := item.InstalledPackage
		if pkg == nil {
			continue
		}
		if pkg.AptPackage != nil {
			pkgs.Deb = append(pkgs.Deb, &PackageInfo{
				Name:    pkg.AptPackage.PackageName,
				Arch:    pkg.AptPackage.Architecture,
				Version: pkg.AptPackage.Version,
			})
		}
		if pkg.YumPackage != nil {
			pkgs.Rpm = append(pkgs.Rpm, &PackageInfo{
				Name:    pkg.YumPackage.PackageName,
				Arch:    pkg.YumPackage.Architecture,
				Version: pkg.YumPackage.Version,
			})
		}
		if pkg.ZypperPackage != nil {
			pkgs.Zypper = append(pkgs.Zypper, &PackageInfo{
				Name:    pkg.ZypperPackage.PackageName,
				Arch:    pkg.ZypperPackage.Architecture,
				Version: pkg.ZypperPackage.Version,
			})
		}
		if pkg.CosPackage != nil {
			pkgs.COS = append(pkgs.COS, &PackageInfo{
				Name:    pkg.CosPackage.PackageName,
				Arch:    pkg.CosPackage.Architecture,
				Version: pkg.CosPackage.Version,
			})
		}
		if pkg.GoogetPackage != nil {
			pkgs.GooGet = append(pkgs.GooGet, &PackageInfo{
				Name:    pkg.GoogetPackage.PackageName,
				Arch:    pkg.GoogetPackage.Architecture,
				Version: pkg.GoogetPackage.Version,
			})
		}
		if pkg.WuaPackage != nil {
			pkgs.WUA = append(pkgs.WUA, &WUAPackage{
				Title: pkg.WuaPackage.Title,
			})
		}
		if pkg.QfePackage != nil {
			pkgs.QFE = append(pkgs.QFE, &QFEPackage{
				HotFixID: pkg.QfePackage.HotFixId,
			})
		}
	}
	return pkgs
}

// ExtractAvailablePackages extracts available package updates from an OS Config Inventory object.
func ExtractAvailablePackages(inv *osconfig.Inventory) *InstalledPackages {
	if inv == nil {
		return nil
	}
	pkgs := &InstalledPackages{}
	for _, item := range inv.Items {
		pkg := item.AvailablePackage
		if pkg == nil {
			continue
		}
		if pkg.AptPackage != nil {
			pkgs.Deb = append(pkgs.Deb, &PackageInfo{
				Name:    pkg.AptPackage.PackageName,
				Arch:    pkg.AptPackage.Architecture,
				Version: pkg.AptPackage.Version,
			})
		}
		if pkg.YumPackage != nil {
			pkgs.Rpm = append(pkgs.Rpm, &PackageInfo{
				Name:    pkg.YumPackage.PackageName,
				Arch:    pkg.YumPackage.Architecture,
				Version: pkg.YumPackage.Version,
			})
		}
		if pkg.ZypperPackage != nil {
			pkgs.Zypper = append(pkgs.Zypper, &PackageInfo{
				Name:    pkg.ZypperPackage.PackageName,
				Arch:    pkg.ZypperPackage.Architecture,
				Version: pkg.ZypperPackage.Version,
			})
		}
		if pkg.CosPackage != nil {
			pkgs.COS = append(pkgs.COS, &PackageInfo{
				Name:    pkg.CosPackage.PackageName,
				Arch:    pkg.CosPackage.Architecture,
				Version: pkg.CosPackage.Version,
			})
		}
		if pkg.GoogetPackage != nil {
			pkgs.GooGet = append(pkgs.GooGet, &PackageInfo{
				Name:    pkg.GoogetPackage.PackageName,
				Arch:    pkg.GoogetPackage.Architecture,
				Version: pkg.GoogetPackage.Version,
			})
		}
		if pkg.WuaPackage != nil {
			pkgs.WUA = append(pkgs.WUA, &WUAPackage{
				Title: pkg.WuaPackage.Title,
			})
		}
		if pkg.QfePackage != nil {
			pkgs.QFE = append(pkgs.QFE, &QFEPackage{
				HotFixID: pkg.QfePackage.HotFixId,
			})
		}
	}
	return pkgs
}

// DecodePackages decodes, decompresses, and unmarshals gzipped base64 JSON package data from guest attributes.
func DecodePackages(encoded string) (*InstalledPackages, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, fmt.Errorf("package data attribute is empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	zr, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer zr.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zr); err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}

	var pkgs InstalledPackages
	if err := json.Unmarshal(buf.Bytes(), &pkgs); err != nil {
		return nil, fmt.Errorf("unmarshal JSON: %w", err)
	}

	return &pkgs, nil
}

// DecodeInstalledPackages decodes, decompresses, and unmarshals the InstalledPackages guest attribute.
func DecodeInstalledPackages(encoded string) (*InstalledPackages, error) {
	return DecodePackages(encoded)
}

// Suite contains process-scoped test dependencies.
type Suite struct {
	Config    config.Config
	Compute   *gcp.Client
	Scheduler *scheduler.Scheduler
	Reporter  *junitxml.Reporter
	RunID     string
}

// Init initializes the shared test suite with the given configuration.
func Init(cfg config.Config) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	computeClient, err := gcp.NewClient(context.Background(), cfg)
	if err != nil {
		globalInitErr = err
		return err
	}

	sched, err := scheduler.New(cfg.MaxConcurrentVMs)
	if err != nil {
		globalInitErr = err
		return err
	}

	runID := time.Now().UTC().Format("20060102t150405z")

	globalSuite = &Suite{
		Config:    cfg,
		Compute:   computeClient,
		Scheduler: sched,
		Reporter:  junitxml.NewReporter(),
		RunID:     runID,
	}
	return nil
}

// Close writes the JUnit XML report to disk and cleans up suite resources.
func Close() error {
	globalMu.Lock()
	suite := globalSuite
	globalMu.Unlock()

	if suite == nil || suite.Reporter == nil || suite.Config.ArtifactFile == "" {
		return nil
	}
	return suite.Reporter.WriteToFile(suite.Config.ArtifactFile)
}

// Test manages resources and assertions for an individual test case.
type Test struct {
	t         *testing.T
	Suite     *Suite
	Context   context.Context
	AttemptID string
	TestName  string
	Project   string
	Zone      string
	failures  []string
}

// New creates and initializes a Test environment for the calling test.
// If timeout is > 0, it overrides the suite's configured TestTimeout.
func New(t *testing.T, timeout time.Duration) *Test {
	t.Helper()

	globalMu.Lock()
	suite := globalSuite
	initErr := globalInitErr
	globalMu.Unlock()

	if initErr != nil {
		t.Fatalf("test initialization failed: %v", initErr)
	}
	if suite == nil {
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("load config: %v", err)
		}
		if err := Init(cfg); err != nil {
			t.Fatalf("init test: %v", err)
		}
		globalMu.Lock()
		suite = globalSuite
		globalMu.Unlock()
	}

	acquireCtx, acquireCancel := context.WithTimeout(context.Background(), suite.Config.TestTimeout)
	defer acquireCancel()

	lease, err := suite.Scheduler.Acquire(acquireCtx)
	if err != nil {
		t.Fatalf("acquire capacity lease: %v", err)
	}
	t.Cleanup(lease.Release)

	testTimeout := suite.Config.TestTimeout
	if timeout > 0 {
		testTimeout = timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	attemptID := uuid.NewString()[:8]

	test := &Test{
		t:         t,
		Suite:     suite,
		Context:   ctx,
		AttemptID: attemptID,
		TestName:  t.Name(),
		Project:   suite.Config.Project,
		Zone:      suite.Config.Zone,
	}

	start := time.Now()
	t.Cleanup(func() {
		duration := time.Since(start)
		suiteName := strings.Split(t.Name(), "/")[0]
		var failureMsg string
		if t.Failed() {
			if len(test.failures) > 0 {
				failureMsg = strings.Join(test.failures, "\n")
			} else {
				failureMsg = "test failed"
			}
		}
		suite.Reporter.Record(suiteName, t.Name(), "e2etests_test", duration, failureMsg, t.Skipped(), "")
	})

	return test
}

// Step executes a named phase of the test, recording timing and logging execution.
func (test *Test) Step(name string, fn func(ctx context.Context) error) {
	test.t.Helper()

	test.t.Logf("=== STEP: %s ===", name)
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			elapsed := time.Since(start).Round(time.Millisecond)
			errMsg := fmt.Sprintf("step %q panicked after %s: %v\n%s", name, elapsed, r, debug.Stack())
			test.failures = append(test.failures, errMsg)
			test.t.Fatalf("%s", errMsg)
		}
	}()

	err := fn(test.Context)
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		errMsg := fmt.Sprintf("step %q failed after %s: %v", name, elapsed, err)
		test.failures = append(test.failures, errMsg)
		test.t.Fatalf("%s", errMsg)
	}

	test.t.Logf("--- STEP PASSED: %s (%s) ---", name, elapsed)
}

// CreateVM provisions a new isolated Compute Engine instance and registers cleanup.
func (test *Test) CreateVM(image, machineType string, customMetadata map[string]string) (*gcp.VM, error) {
	test.t.Helper()

	name := resourceName(test.Suite.RunID, test.TestName, test.AttemptID)
	planned := gcp.VM{
		Project: test.Project,
		Zone:    test.Zone,
		Name:    name,
	}

	// Register serial output recording and idempotent VM deletion on test cleanup
	test.t.Cleanup(func() {
		diagCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		serial, err := test.Suite.Compute.SerialOutput(diagCtx, planned.Project, planned.Zone, planned.Name)
		artDir := filepath.Dir(test.Suite.Config.ArtifactFile)
		if err == nil && serial != "" && artDir != "" && artDir != "." {
			_ = os.MkdirAll(artDir, 0o755)
			logFile := filepath.Join(artDir, fmt.Sprintf("%s-serial.log", planned.Name))
			_ = os.WriteFile(logFile, []byte(serial), 0o644)
		} else if err == nil && serial != "" {
			logFile := fmt.Sprintf("%s-serial.log", planned.Name)
			_ = os.WriteFile(logFile, []byte(serial), 0o644)
		}

		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), test.Suite.Config.CleanupTimeout)
		defer cleanupCancel()
		if err := test.Suite.Compute.DeleteVM(cleanupCtx, planned.Project, planned.Zone, planned.Name); err != nil {
			test.t.Errorf("cleanup instance %s/%s/%s: %v", planned.Project, planned.Zone, planned.Name, err)
		}
	})

	metadata := map[string]string{
		"enable-osconfig":            "true",
		"enable-guest-attributes":    "true",
		"osconfig-disabled-features": "tasks,guestpolicies",
		"osconfig-poll-interval":     "1",
	}

	scriptKey, scriptContent := gcp.DefaultStartupScript(image)
	metadata[scriptKey] = scriptContent
	maps.Copy(metadata, customMetadata)

	if machineType == "" {
		machineType = "e2-standard-2"
	}

	request := gcp.VMRequest{
		Project:        planned.Project,
		Zone:           planned.Zone,
		Name:           planned.Name,
		Image:          image,
		MachineType:    machineType,
		Network:        test.Suite.Config.Network,
		Subnetwork:     test.Suite.Config.Subnetwork,
		ServiceAccount: test.Suite.Config.ServiceAccount,
		Metadata:       metadata,
		Labels: map[string]string{
			"e2e-run":     labelValue(test.Suite.RunID),
			"e2e-test":    labelValue(test.TestName),
			"e2e-attempt": labelValue(test.AttemptID),
		},
	}

	test.t.Logf("Creating VM %q (image: %s, zone: %s)", planned.Name, image, planned.Zone)
	vm, err := test.Suite.Compute.CreateVM(test.Context, request)
	if err != nil {
		return nil, err
	}

	return vm, nil
}

// WaitForInventory polls the public OS Config API until inventory is reported for the instance.
func (test *Test) WaitForInventory(vm *gcp.VM) (*osconfig.Inventory, error) {
	test.t.Helper()

	test.t.Logf("Waiting for OS inventory to be reported on %q", vm.Name)

	var result *osconfig.Inventory
	err := gcp.PollUntil(test.Context, test.Suite.Config.PollInterval, fmt.Sprintf("inventory on %s", vm.Name), func(ctx context.Context) (string, bool, error) {
		inv, err := test.Suite.Compute.GetInventory(ctx, vm.Project, vm.Zone, vm.Name, "FULL")
		if err != nil {
			if gcp.IsNotFound(err) || gcp.IsTransientComputeError(err) {
				return fmt.Sprintf("inventory not yet published (%v)", err), false, nil
			}
			return "", false, fmt.Errorf("read inventory: %w", err)
		}

		if inv == nil || inv.OsInfo == nil || strings.TrimSpace(inv.OsInfo.Hostname) == "" {
			return "inventory OsInfo not yet populated", false, nil
		}

		result = inv
		return fmt.Sprintf("Hostname=%s ShortName=%s Items=%d", inv.OsInfo.Hostname, inv.OsInfo.ShortName, len(inv.Items)), true, nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func resourceName(runID, testName, attempt string) string {
	base := labelValue(fmt.Sprintf("inv-%s-%s", runID, testName))
	if len(base) > 53 {
		base = strings.Trim(base[:53], "-")
	}
	return strings.Trim(fmt.Sprintf("%s-%s", base, attempt), "-")
}

func labelValue(value string) string {
	value = strings.ToLower(value)
	value = unsafeResourceName.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "unknown"
	}
	if len(value) > 63 {
		value = strings.Trim(value[:63], "-")
	}
	return value
}
