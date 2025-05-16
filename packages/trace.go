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
	done := make(chan bool)
	result := utiltrace.TraceMemoryResult{}

	go utiltrace.TraceMemory(done, time.Duration(100*time.Millisecond), &result)
	startTime := time.Now()

	pkgs, err := p.provider.GetInstalledPackages(ctx)
	duration := time.Since(startTime)

	close(done)
	p.handleTraceResult(result, duration)

	return pkgs, err
}
