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

package testenv

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/config"
)

// StepTiming records the duration and outcome of a single test step.
type StepTiming struct {
	Name        string  `json:"name"`
	DurationSec float64 `json:"duration_sec"`
	Status      string  `json:"status"`
}

// TestTiming records execution timings for a single test case.
type TestTiming struct {
	TestName     string       `json:"test_name"`
	Suite        string       `json:"suite"`
	OS           string       `json:"os,omitempty"`
	Status       string       `json:"status"`
	QueueWaitSec float64      `json:"queue_wait_sec"`
	Steps        []StepTiming `json:"steps"`
	CleanupSec   float64      `json:"cleanup_sec"`
	TotalSec     float64      `json:"total_sec"`
}

// SuiteTiming is the JSON report structure for an entire E2E test run.
type SuiteTiming struct {
	RunID            string       `json:"run_id"`
	StartedAt        time.Time    `json:"started_at"`
	TotalWallSec     float64      `json:"total_wall_sec"`
	MaxConcurrentVMs int          `json:"max_concurrent_vms"`
	PollIntervalSec  float64      `json:"poll_interval_sec"`
	Tests            []TestTiming `json:"tests"`
}

// TimingRecorder collects per-test and per-step timing metrics across a test run.
type TimingRecorder struct {
	mu        sync.Mutex
	runID     string
	startedAt time.Time
	cfg       config.Config
	tests     []TestTiming
}

// NewTimingRecorder creates a TimingRecorder for the given run.
func NewTimingRecorder(runID string, cfg config.Config) *TimingRecorder {
	return &TimingRecorder{
		runID:     runID,
		startedAt: time.Now().UTC(),
		cfg:       cfg,
	}
}

// Record stores timing data for a completed test case.
func (r *TimingRecorder) Record(tt TestTiming) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tests = append(r.tests, tt)
}

// Snapshot returns a copy of the collected suite timings sorted by slowest test first.
func (r *TimingRecorder) Snapshot() SuiteTiming {
	r.mu.Lock()
	tests := make([]TestTiming, len(r.tests))
	copy(tests, r.tests)
	r.mu.Unlock()

	sort.Slice(tests, func(i, j int) bool {
		return tests[i].TotalSec > tests[j].TotalSec
	})

	return SuiteTiming{
		RunID:            r.runID,
		StartedAt:        r.startedAt,
		TotalWallSec:     roundSec(time.Since(r.startedAt)),
		MaxConcurrentVMs: r.cfg.MaxConcurrentVMs,
		PollIntervalSec:  roundSec(r.cfg.PollInterval),
		Tests:            tests,
	}
}

// WriteJSON writes the timing snapshot as formatted JSON to path.
func (r *TimingRecorder) WriteJSON(path string) error {
	if r == nil || path == "" {
		return nil
	}
	snap := r.Snapshot()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal timings: %w", err)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create timings dir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write timings file %s: %w", path, err)
	}
	return nil
}

// WriteSummary writes a concise human-readable timing breakdown to w.
func (r *TimingRecorder) WriteSummary(w io.Writer) {
	if r == nil || w == nil {
		return
	}
	snap := r.Snapshot()
	if len(snap.Tests) == 0 {
		return
	}

	fmt.Fprintf(w, "\n=== E2E TIMING SUMMARY (run=%s, wall=%.1fs, tests=%d, max_vms=%d, poll=%.0fs) ===\n",
		snap.RunID, snap.TotalWallSec, len(snap.Tests), snap.MaxConcurrentVMs, snap.PollIntervalSec)

	type stepAgg struct {
		name  string
		count int
		total float64
		max   float64
	}
	aggMap := make(map[string]*stepAgg)
	addAgg := func(name string, sec float64) {
		a, ok := aggMap[name]
		if !ok {
			a = &stepAgg{name: name}
			aggMap[name] = a
		}
		a.count++
		a.total += sec
		if sec > a.max {
			a.max = sec
		}
	}

	for _, t := range snap.Tests {
		addAgg("[queue_wait]", t.QueueWaitSec)
		addAgg("[cleanup]", t.CleanupSec)
		for _, s := range t.Steps {
			addAgg(s.Name, s.DurationSec)
		}
	}

	aggs := make([]*stepAgg, 0, len(aggMap))
	for _, a := range aggMap {
		aggs = append(aggs, a)
	}
	sort.Slice(aggs, func(i, j int) bool {
		return aggs[i].total > aggs[j].total
	})

	fmt.Fprintf(w, "\nBy phase/step (sorted by cumulative time):\n")
	fmt.Fprintf(w, "  %-38s %5s %8s %8s %9s\n", "STEP", "COUNT", "AVG(s)", "MAX(s)", "TOTAL(s)")
	for _, a := range aggs {
		avg := a.total / float64(a.count)
		fmt.Fprintf(w, "  %-38s %5d %8.1f %8.1f %9.1f\n", a.name, a.count, avg, a.max, a.total)
	}

	limit := min(10, len(snap.Tests))
	fmt.Fprintf(w, "\nSlowest tests (top %d):\n", limit)
	fmt.Fprintf(w, "  %-48s %-7s %8s %8s %8s  %s\n", "TEST", "STATUS", "TOTAL(s)", "QUEUE(s)", "CLEAN(s)", "SLOWEST STEP")
	for _, t := range snap.Tests[:limit] {
		slowest := "-"
		var maxStep float64
		for _, s := range t.Steps {
			if s.DurationSec >= maxStep {
				maxStep = s.DurationSec
				slowest = fmt.Sprintf("%s (%.1fs)", s.Name, s.DurationSec)
			}
		}
		fmt.Fprintf(w, "  %-48s %-7s %8.1f %8.1f %8.1f  %s\n", t.TestName, t.Status, t.TotalSec, t.QueueWaitSec, t.CleanupSec, slowest)
	}
	fmt.Fprintln(w)
}

func roundSec(d time.Duration) float64 {
	if d < 0 {
		return 0
	}
	return math.Round(d.Seconds()*1000) / 1000
}
