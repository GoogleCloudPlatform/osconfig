package utiltrace

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestTraceMemory(t *testing.T) {
	tests := []struct {
		name         string
		memoryLevels []float64
		want         TraceMemoryResult
	}{
		{
			name:         "memory level after execution is excluded from {mean, peak}",
			memoryLevels: []float64{10, 9},
			want:         TraceMemoryResult{MemAfterMB: 9, MemBeforeMB: 10, MemPeakMB: 10, MemMeanMB: 10, SampleCount: 1},
		},
		{
			name:         "highest non-final value is captured as peak",
			memoryLevels: []float64{10, 20, 30, 20, 10},
			want:         TraceMemoryResult{MemPeakMB: 30, MemMeanMB: 20, MemBeforeMB: 10, MemAfterMB: 10, SampleCount: 4},
		},
		{
			name:         "zero values are tolerated",
			memoryLevels: []float64{0, 0, 0},
			want:         TraceMemoryResult{MemBeforeMB: 0, MemMeanMB: 0, MemAfterMB: 0, SampleCount: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan bool)
			mockMemoryApi(t, tt.memoryLevels, done)

			got := TraceMemoryResult{}
			TraceMemory(done, time.Millisecond, &got)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func mockMemoryApi(t *testing.T, levels []float64, done chan bool) {
	prevMemoryUsageMB, prevCompactMemory := memoryUsageMB, compactMemory
	t.Cleanup(func() { memoryUsageMB, compactMemory = prevMemoryUsageMB, prevCompactMemory })

	if len(levels) < 2 {
		t.Fatal("prerequisite failed: test.levels must contain at least 2 elements: {before, after}")
	}
	beforeLevelIdx := 0
	afterLevelIdx := len(levels) - 1
	levelIdx := beforeLevelIdx
	closed := false
	memoryUsageMB = func() float64 {
		usage := levels[levelIdx]
		if levelIdx < afterLevelIdx {
			levelIdx += 1
		}
		if levelIdx == afterLevelIdx && !closed {
			closed = true
			close(done)
		}
		return usage
	}

	compactMemoryCallsCount := 0
	compactMemory = func() {
		if !(levelIdx == beforeLevelIdx || levelIdx == afterLevelIdx) {
			t.Errorf("compactMemory() must only be called for measuring before/after (levels[%d], levels[%d]) memory levels, was called for in-between levels[%d], levels: %v", beforeLevelIdx, afterLevelIdx, levelIdx, levels)
		}
		compactMemoryCallsCount += 1
	}

	t.Cleanup(func() {
		if compactMemoryCallsCount != 2 {
			t.Error("compactMemory() must be called twice to get normalized before/after memory levels")
		}
	})
}
