package utiltrace

import (
	"context"
	"runtime"
	"runtime/debug"
	"time"
)

var compactMemory = func() {
	runtime.GC()
	debug.FreeOSMemory()
}

var memoryUsageMB = func() float64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	bytes := stats.Sys - stats.HeapReleased
	return float64(bytes) / 1024 / 1024
}

// TraceMemoryResult reflects memory usage stats collected by TraceMemory function
type TraceMemoryResult struct {
	MemBeforeMB float64
	MemAfterMB  float64
	MemPeakMB   float64
	MemMeanMB   float64
	SampleCount int
}

// TraceMemory collects memory usage with specified interval until done channel is closed
func TraceMemory(ctx context.Context, interval time.Duration, resultChannel chan TraceMemoryResult) {
	compactMemory()
	startMB := memoryUsageMB()
	result := TraceMemoryResult{
		MemBeforeMB: startMB,
		MemPeakMB:   startMB,
		SampleCount: 1,
	}
	runningAverageMB := startMB

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentMB := memoryUsageMB()
			result.SampleCount++
			runningAverageMB += (currentMB - runningAverageMB) / float64(result.SampleCount)
			if result.MemPeakMB < currentMB {
				result.MemPeakMB = currentMB
			}
		case <-ctx.Done():
			compactMemory()
			result.MemAfterMB = memoryUsageMB()
			result.MemMeanMB = runningAverageMB
			resultChannel <- result
			return
		}
	}
}
