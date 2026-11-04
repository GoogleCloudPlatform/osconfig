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

// TraceResult reflects resource usage stats collected by Trace function.
type TraceResult struct {
	TraceMemoryResult
	CPUPeakPercent float64
	CPUMeanPercent float64
	Duration       time.Duration
	AllocMB        float64
}

type cpuSample struct {
	totalCPUTime time.Duration
	time         time.Time
}

// Trace collects memory and CPU usage with specified interval until ctx is cancelled.
func Trace(ctx context.Context, interval time.Duration, resultChannel chan TraceResult) {
	compactMemory()
	startTime := time.Now()

	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	startMB := memoryUsageMB()
	result := TraceResult{
		TraceMemoryResult: TraceMemoryResult{
			MemBeforeMB: startMB,
			MemPeakMB:   startMB,
			SampleCount: 1,
		},
	}
	runningAverageMB := startMB

	var lastSample cpuSample
	startCPUTime, err := getCPUTime()
	if err == nil {
		lastSample = cpuSample{totalCPUTime: startCPUTime, time: startTime}
	}

	var peakCPU, runningAverageCPU float64
	cpuSampleCount := 0

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

			currentCPUTime, err := getCPUTime()
			now := time.Now()
			if err == nil && !lastSample.time.IsZero() {
				timeDelta := now.Sub(lastSample.time).Seconds()
				cpuDelta := (currentCPUTime - lastSample.totalCPUTime).Seconds()
				if timeDelta > 0 {
					cpuPercent := (cpuDelta / timeDelta) * 100.0
					cpuSampleCount++
					runningAverageCPU += (cpuPercent - runningAverageCPU) / float64(cpuSampleCount)
					if cpuPercent > peakCPU {
						peakCPU = cpuPercent
					}
				}
				lastSample = cpuSample{totalCPUTime: currentCPUTime, time: now}
			}
		case <-ctx.Done():
			compactMemory()
			totalDuration := time.Since(startTime)

			var memAfter runtime.MemStats
			runtime.ReadMemStats(&memAfter)

			result.MemAfterMB = memoryUsageMB()
			result.MemMeanMB = runningAverageMB
			result.AllocMB = float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024
			result.Duration = totalDuration

			if finalCPUTime, err := getCPUTime(); err == nil && totalDuration > 0 && !lastSample.time.IsZero() {
				meanCPU := (finalCPUTime - startCPUTime).Seconds() / totalDuration.Seconds() * 100.0
				runningAverageCPU = meanCPU
				if meanCPU > peakCPU {
					peakCPU = meanCPU
				}
			}

			result.CPUPeakPercent = peakCPU
			result.CPUMeanPercent = runningAverageCPU

			resultChannel <- result
			return
		}
	}
}

// TraceMemory collects memory usage with specified interval until done channel is closed.
func TraceMemory(ctx context.Context, interval time.Duration, resultChannel chan TraceMemoryResult) {
	traceChan := make(chan TraceResult, 1)
	go Trace(ctx, interval, traceChan)
	res := <-traceChan
	resultChannel <- res.TraceMemoryResult
}
