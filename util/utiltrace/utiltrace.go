package utiltrace

import (
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
func TraceMemory(done chan bool, interval time.Duration, result *TraceMemoryResult) {
	compactMemory()
	startMB := memoryUsageMB()

	runningAverageMB := startMB
	result.MemBeforeMB = startMB
	result.MemPeakMB = startMB
	result.SampleCount = 1

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentMB := memoryUsageMB()
			result.SampleCount += 1
			runningAverageMB += (currentMB - runningAverageMB) / float64(result.SampleCount)
			if result.MemPeakMB < currentMB {
				result.MemPeakMB = currentMB
			}
		case <-done:
			compactMemory()
			result.MemAfterMB = memoryUsageMB()
			result.MemMeanMB = runningAverageMB
			return
		}
	}
}
