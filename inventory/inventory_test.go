package inventory

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/google/go-cmp/cmp"
)

func TestProvider(t *testing.T) {
	osInfo := osinfo.OSInfo{
		Hostname:      "testhost",
		LongName:      "testLong",
		ShortName:     "testShort",
		Version:       "testVersion",
		KernelVersion: "#1 SMP PREEMPT_DYNAMIC Debian 6.1.123-1 (2025-01-02)",
		KernelRelease: "6.1.0-29-cloud-amd64",
		Architecture:  "x86_64",
	}

	updates := packages.Packages{
		Yum: []*packages.PkgInfo{{Name: "YumPkgUpdate", Arch: "Arch", Version: "Version"}},
		Apt: []*packages.PkgInfo{{Name: "AptPkgUpdate", Arch: "Arch", Version: "Version"}},
	}

	installed := packages.Packages{
		Yum:    []*packages.PkgInfo{{Name: "YumInstalledPkg", Arch: "Arch", Version: "Version"}},
		GooGet: []*packages.PkgInfo{{Name: "GooGetInstalledPkg", Arch: "Arch", Version: "Version"}},
	}

	tests := []struct {
		name string
		stub *stubProvider
		want *InstanceInventory
	}{
		{
			name: "all providers failed, returns empty result",
			stub: &stubProvider{
				osinfo: func(_ context.Context) (osinfo.OSInfo, error) { return osinfo.OSInfo{}, fmt.Errorf("unexpected error") },
				packageUpdates: func(_ context.Context) (packages.Packages, error) {
					return packages.Packages{}, fmt.Errorf("unexpected error")
				},
				installedPackages: func(_ context.Context) (packages.Packages, error) {
					return packages.Packages{}, fmt.Errorf("unexpected error")
				},
			},
			want: &InstanceInventory{
				InstalledPackages: &packages.Packages{},
				PackageUpdates:    &packages.Packages{},
				LastUpdated:       "1970-01-01T10:00:00Z",
			},
		},
		{
			name: "all providers succeeded, returns all data",
			stub: &stubProvider{
				osinfo: func(_ context.Context) (osinfo.OSInfo, error) { return osInfo, nil },
				packageUpdates: func(_ context.Context) (packages.Packages, error) {
					return updates, nil
				},
				installedPackages: func(_ context.Context) (packages.Packages, error) {
					return installed, nil
				},
			},

			want: &InstanceInventory{
				Hostname:             "testhost",
				LongName:             "testLong",
				ShortName:            "testShort",
				Version:              "testVersion",
				Architecture:         "x86_64",
				KernelVersion:        "#1 SMP PREEMPT_DYNAMIC Debian 6.1.123-1 (2025-01-02)",
				KernelRelease:        "6.1.0-29-cloud-amd64",
				OSConfigAgentVersion: "",
				InstalledPackages: &packages.Packages{
					Yum:    []*packages.PkgInfo{{Name: "YumInstalledPkg", Arch: "Arch", Version: "Version"}},
					GooGet: []*packages.PkgInfo{{Name: "GooGetInstalledPkg", Arch: "Arch", Version: "Version"}},
				},
				PackageUpdates: &packages.Packages{
					Yum: []*packages.PkgInfo{{Name: "YumPkgUpdate", Arch: "Arch", Version: "Version"}},
					Apt: []*packages.PkgInfo{{Name: "AptPkgUpdate", Arch: "Arch", Version: "Version"}},
				},
				LastUpdated: "1970-01-01T10:00:00Z",
			},
		},
		{
			name: "some providers succeeded, returns available data",
			stub: &stubProvider{
				osinfo: func(_ context.Context) (osinfo.OSInfo, error) { return osInfo, nil },
				packageUpdates: func(_ context.Context) (packages.Packages, error) {
					return packages.Packages{}, fmt.Errorf("unexpected error")
				},
				installedPackages: func(_ context.Context) (packages.Packages, error) {
					return installed, nil
				},
			},

			want: &InstanceInventory{
				Hostname:             "testhost",
				LongName:             "testLong",
				ShortName:            "testShort",
				Version:              "testVersion",
				Architecture:         "x86_64",
				KernelVersion:        "#1 SMP PREEMPT_DYNAMIC Debian 6.1.123-1 (2025-01-02)",
				KernelRelease:        "6.1.0-29-cloud-amd64",
				OSConfigAgentVersion: "",
				PackageUpdates:       &packages.Packages{},
				InstalledPackages: &packages.Packages{
					Yum:    []*packages.PkgInfo{{Name: "YumInstalledPkg", Arch: "Arch", Version: "Version"}},
					GooGet: []*packages.PkgInfo{{Name: "GooGetInstalledPkg", Arch: "Arch", Version: "Version"}},
				},
				LastUpdated: "1970-01-01T10:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := defaultInventoryProvider{
				osInfoProvider:            tt.stub,
				packageUpdatesProvider:    tt.stub,
				installedPackagesProvider: tt.stub,
				clock:                     stubClock{},
			}

			ctx := context.Background()
			got := provider.Get(ctx)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected diff, diff:\n%s", diff)
			}

		})
	}

}

func TestNewProvider(t *testing.T) {
	provider := NewProvider()

	if provider == nil {
		t.Errorf("provider is not valid")
	}
}

type stubClock struct{}

func (sc stubClock) Now() time.Time {
	return time.UnixMicro(0).Add(10 * time.Hour)
}

type stubProvider struct {
	osinfo            func(context.Context) (osinfo.OSInfo, error)
	packageUpdates    func(context.Context) (packages.Packages, error)
	installedPackages func(context.Context) (packages.Packages, error)
}

func (p stubProvider) GetOSInfo(ctx context.Context) (osinfo.OSInfo, error) {
	return p.osinfo(ctx)
}

func (p stubProvider) GetInstalledPackages(ctx context.Context) (packages.Packages, error) {
	return p.installedPackages(ctx)
}

func (p stubProvider) GetPackageUpdates(ctx context.Context) (packages.Packages, error) {
	return p.packageUpdates(ctx)
}
