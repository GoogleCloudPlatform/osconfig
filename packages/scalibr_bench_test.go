//  Copyright 2026 Google Inc. All Rights Reserved.
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

//go:build benchmark

package packages

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltrace"
)

type benchResult struct {
	duration  time.Duration
	allocMB   float64
	memPeakMB float64
	cpuPeak   float64
	cpuMean   float64
	pkgsCount int
}

// countPackages returns the total number of all collected packages.
func countPackages(pkgs Packages) int {
	return len(pkgs.Deb) + len(pkgs.Rpm) + len(pkgs.COS) + len(pkgs.Snap) +
		len(pkgs.Pip) + len(pkgs.Gem) + len(pkgs.Npm) + len(pkgs.Maven) +
		len(pkgs.Go) + len(pkgs.Cargo) + len(pkgs.Composer) + len(pkgs.Swift) +
		len(pkgs.Pub) + len(pkgs.Nuget)
}

// runBenchmark runs a single iteration of installed packages extraction with metrics tracing.
func runBenchmark(ctx context.Context, osinfoProvider osinfo.Provider, extractors []string) (benchResult, error) {
	runtime.GC()

	traceCtx, cancelTrace := context.WithCancel(ctx)
	resChan := make(chan utiltrace.TraceResult, 1)
	go utiltrace.Trace(traceCtx, 20*time.Millisecond, resChan)

	provider := &scalibrInstalledPackagesProvider{
		extractors:     extractors,
		osinfoProvider: osinfoProvider,
	}
	pkgs, err := provider.GetInstalledPackages(ctx)
	cancelTrace()

	metrics := <-resChan
	if err != nil {
		return benchResult{}, fmt.Errorf("GetInstalledPackages error: %w", err)
	}

	pkgsCount := countPackages(pkgs)

	return benchResult{
		duration:  metrics.Duration,
		allocMB:   metrics.AllocMB,
		memPeakMB: metrics.MemPeakMB,
		cpuPeak:   metrics.CPUPeakPercent,
		cpuMean:   metrics.CPUMeanPercent,
		pkgsCount: pkgsCount,
	}, nil
}

// logBenchResult calculates averages over runs and outputs the formatted benchmark table row.
func logBenchResult(t *testing.T, name string, res benchResult, runs int) {
	if runs <= 0 {
		return
	}
	r := float64(runs)
	avgDuration := res.duration / time.Duration(runs)
	avgAllocMB := res.allocMB / r
	avgPeakRAM := res.memPeakMB / r
	avgPeakCPU := res.cpuPeak / r
	avgMeanCPU := res.cpuMean / r

	t.Logf("| `%s` | %v | %.2f MB | %.2f MB | %.1f%% | %.1f%% | %d |",
		name,
		avgDuration,
		avgAllocMB,
		avgPeakRAM,
		avgPeakCPU,
		avgMeanCPU,
		res.pkgsCount,
	)
}

func TestScalibrBenchmark(t *testing.T) {
	allLanguageExtractors := []string{
		"python/wheelegg",
		"python/uvlock",
		"ruby/gemspec",
		"javascript/packagejson",
		"java/archive",
		"java/gradlelockfile",
		"java/pomxml",
		"go/binary",
		"go/gomod",
		"rust/cargolock",
		"rust/cargoauditable",
		"php/composerlock",
		"swift/packageresolved",
		"dart/pubspec",
		"dotnet/nugetcpm",
	}

	benchmarks := []struct {
		name       string
		extractors []string
	}{
		{name: "All language extractors", extractors: allLanguageExtractors},
		{name: "python/wheelegg", extractors: []string{"python/wheelegg"}},
		{name: "python/uvlock", extractors: []string{"python/uvlock"}},
		{name: "ruby/gemspec", extractors: []string{"ruby/gemspec"}},
		{name: "javascript/packagejson", extractors: []string{"javascript/packagejson"}},
		{name: "java/archive", extractors: []string{"java/archive"}},
		{name: "java/gradlelockfile", extractors: []string{"java/gradlelockfile"}},
		{name: "java/pomxml", extractors: []string{"java/pomxml"}},
		{name: "go/binary", extractors: []string{"go/binary"}},
		{name: "go/gomod", extractors: []string{"go/gomod"}},
		{name: "rust/cargolock", extractors: []string{"rust/cargolock"}},
		{name: "rust/cargoauditable", extractors: []string{"rust/cargoauditable"}},
		{name: "php/composerlock", extractors: []string{"php/composerlock"}},
		{name: "swift/packageresolved", extractors: []string{"swift/packageresolved"}},
		{name: "dart/pubspec", extractors: []string{"dart/pubspec"}},
		{name: "dotnet/nugetcpm", extractors: []string{"dotnet/nugetcpm"}},
	}

	ctx := context.Background()
	osinfoProvider := osinfo.NewProvider()

	t.Log("\n=========================================================================")
	t.Log("### SCALIBR Extractors Benchmark Results")
	t.Log("=========================================================================")
	t.Log("| Scenario | Avg Scan Time | Avg Heap Alloc | Peak RAM RSS | Peak CPU % | Mean CPU % | Pkgs Found |")
	t.Log("| --- | --- | --- | --- | --- | --- | --- |")

	runs := 10
	for _, bm := range benchmarks {
		var total benchResult
		for i := 0; i < runs; i++ {
			res, err := runBenchmark(ctx, osinfoProvider, bm.extractors)
			if err != nil {
				t.Fatalf("Benchmark scenario %s failed: %v", bm.name, err)
			}
			total.duration += res.duration
			total.allocMB += res.allocMB
			total.memPeakMB += res.memPeakMB
			total.cpuPeak += res.cpuPeak
			total.cpuMean += res.cpuMean
			total.pkgsCount = res.pkgsCount
		}

		logBenchResult(t, bm.name, total, runs)
	}
}
