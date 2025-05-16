package packages

import (
	"context"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltrace"
)

type tracingInstalledPackagesProvider struct {
	provider          InstalledPackagesProvider
	handleTraceResult HandleTraceResult
}

type HandleTraceResult func(stats utiltrace.TraceMemoryResult, duration time.Duration)

func TracingInstalledPackagesProvider(provider InstalledPackagesProvider, handleTraceResult HandleTraceResult) InstalledPackagesProvider {
	return tracingInstalledPackagesProvider{
		provider:          provider,
		handleTraceResult: handleTraceResult,
	}
}

func (p tracingInstalledPackagesProvider) GetInstalledPackages(ctx context.Context) (Packages, error) {
	traceCtx, cancel := context.WithCancel(ctx)
	resultChannel := make(chan utiltrace.TraceMemoryResult)
	go utiltrace.TraceMemory(traceCtx, 100*time.Millisecond, resultChannel)

	startTime := time.Now()
	pkgs, err := p.provider.GetInstalledPackages(ctx)
	duration := time.Since(startTime)

	cancel()
	result := <-resultChannel
	p.handleTraceResult(result, duration)

	return pkgs, err
}
