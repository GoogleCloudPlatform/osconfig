package packages

import (
	"context"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltrace"
)

type tracingInstalledPackagesProvider struct {
	tracedProvider InstalledPackagesProvider
	osInfoProvider osinfo.Provider
}

// TracingInstalledPackagesProvider creates an InstalledPackagesProvider decorator that traces the execution time, memory usage, and OS information for each call.
func TracingInstalledPackagesProvider(tracedProvider InstalledPackagesProvider, osInfoProvider osinfo.Provider) InstalledPackagesProvider {
	return tracingInstalledPackagesProvider{tracedProvider: tracedProvider, osInfoProvider: osInfoProvider}
}

func (p tracingInstalledPackagesProvider) GetInstalledPackages(ctx context.Context) (Packages, error) {
	ctx, cancel := context.WithCancel(ctx)
	resultChannel := make(chan utiltrace.TraceMemoryResult)
	go utiltrace.TraceMemory(ctx, 100*time.Millisecond, resultChannel)

	startTime := time.Now()
	pkgs, err := p.tracedProvider.GetInstalledPackages(ctx)
	duration := time.Since(startTime)

	cancel()
	result := <-resultChannel

	osinfo, osinfoErr := p.osInfoProvider.GetOSInfo(ctx)
	if osinfoErr != nil {
		clog.Errorf(ctx, "GetOSInfo() error: %v", osinfoErr)
	}
	logTraceResult(ctx, result, duration, osinfo)

	return pkgs, err
}

func logTraceResult(ctx context.Context, result utiltrace.TraceMemoryResult, duration time.Duration, osinfo osinfo.OSInfo) {
	clog.Debugf(
		ctx,
		"GetInstalledPackages: %.3fs, memory %+.2f MB (=%.2f-%.2f), peak %.2f MB, mean %.2f MB (%d samples), OS: %s@%s, hostname: %s",
		duration.Seconds(),
		result.MemAfterMB-result.MemBeforeMB,
		result.MemAfterMB,
		result.MemBeforeMB,
		result.MemPeakMB,
		result.MemMeanMB,
		result.SampleCount,
		osinfo.ShortName,
		osinfo.KernelRelease,
		osinfo.Hostname,
	)
}
