//  Copyright 2017 Google Inc. All Rights Reserved.
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

// +build !windows

package inventory

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/common"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func decodePackages(str string) packages.Packages {
	decoded, _ := base64.StdEncoding.DecodeString(str)
	zr, _ := gzip.NewReader(bytes.NewReader(decoded))
	var buf bytes.Buffer
	io.Copy(&buf, zr)
	zr.Close()

	var pkgs packages.Packages
	json.Unmarshal(buf.Bytes(), &pkgs)
	return pkgs
}

func TestWrite(t *testing.T) {
	inv := &InstanceInventory{
		Hostname:      "Hostname",
		LongName:      "LongName",
		ShortName:     "ShortName",
		Architecture:  "Architecture",
		KernelVersion: "KernelVersion",
		Version:       "Version",
		InstalledPackages: packages.Packages{
			Yum: []packages.PkgInfo{{Name: "Name", Arch: "Arch", Version: "Version"}},
			WUA: []packages.WUAPackage{{Title: "Title"}},
			QFE: []packages.QFEPackage{{HotFixID: "HotFixID"}},
		},
		PackageUpdates: packages.Packages{
			Apt: []packages.PkgInfo{{Name: "Name", Arch: "Arch", Version: "Version"}},
		},
	}

	want := map[string]bool{
		"Hostname":          false,
		"LongName":          false,
		"ShortName":         false,
		"Architecture":      false,
		"KernelVersion":     false,
		"Version":           false,
		"InstalledPackages": false,
		"PackageUpdates":    false,
	}

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.String()

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(r.Body); err != nil {
			t.Fatal(err)
		}

		switch url {
		case "/Hostname":
			if buf.String() != inv.Hostname {
				t.Errorf("did not get expected Hostname, got: %q, want: %q", buf.String(), inv.Hostname)
			}
			want["Hostname"] = true
		case "/LongName":
			if buf.String() != inv.LongName {
				t.Errorf("did not get expected LongName, got: %q, want: %q", buf.String(), inv.LongName)
			}
			want["LongName"] = true
		case "/ShortName":
			if buf.String() != inv.ShortName {
				t.Errorf("did not get expected ShortName, got: %q, want: %q", buf.String(), inv.ShortName)
			}
			want["ShortName"] = true
		case "/Architecture":
			if buf.String() != inv.Architecture {
				t.Errorf("did not get expected Architecture, got: %q, want: %q", buf.String(), inv.Architecture)
			}
			want["Architecture"] = true
		case "/KernelVersion":
			if buf.String() != inv.KernelVersion {
				t.Errorf("did not get expected KernelVersion, got: %q, want: %q", buf.String(), inv.KernelVersion)
			}
			want["KernelVersion"] = true
		case "/Version":
			if buf.String() != inv.Version {
				t.Errorf("did not get expected Version, got: %q, want: %q", buf.String(), inv.Version)
			}
			want["Version"] = true
		case "/InstalledPackages":
			got := decodePackages(buf.String())
			if !reflect.DeepEqual(got, inv.InstalledPackages) {
				t.Errorf("did not get expected InstalledPackages, got: %q, want: %q", got, inv.InstalledPackages)
			}
			want["InstalledPackages"] = true
		case "/PackageUpdates":
			got := decodePackages(buf.String())
			if !reflect.DeepEqual(got, inv.PackageUpdates) {
				t.Errorf("did not get expected PackageUpdates, got: %q, want: %q", got, inv.PackageUpdates)
			}
			want["PackageUpdates"] = true
		default:
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, url)
		}
	}))
	defer svr.Close()

	write(inv, svr.URL)

	for k, v := range want {
		if v {
			continue
		}
		t.Errorf("writeInventory call did not write %q", k)
	}
}

func TestInventoryInfoRpm(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	rpmQueryResult := `foo x86_64 1.2.3-4
`
	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)
	AppFs.Create("/usr/bin/rpmquery")

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(rpmQueryResult), nil
	}

	config.SetVersion("1")

	ii := Get()

	assertion.Equal("test-hostname", ii.Hostname, "hostname does not match")
	assertion.Equal("Debian buster", ii.LongName, "long name does not match")
	assertion.Equal("test-kernel", ii.KernelVersion, "kernel version does not match")
	assertion.Equal("foo", ii.InstalledPackages.Rpm[0].Name, "package name does not match")
	assertion.Equal("1.2.3-4", ii.InstalledPackages.Rpm[0].Version, "package version does not match")
}

func TestInventoryInfoDeb(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)
	AppFs.Create("/usr/bin/dpkg-query")

	debQueryResult := `foo amd64 1.2.3-4
`

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(debQueryResult), nil
	}

	config.SetVersion("1")

	ii := Get()

	assertion.Equal("test-hostname", ii.Hostname, "hostname does not match")
	assertion.Equal("Debian buster", ii.LongName, "long name does not match")
	assertion.Equal("test-kernel", ii.KernelVersion, "kernel version does not match")
	assertion.Equal("foo", ii.InstalledPackages.Deb[0].Name, "package name does not match")
	assertion.Equal("1.2.3-4", ii.InstalledPackages.Deb[0].Version, "package version does not match")

}

func TestInventoryInfoGem(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)
	AppFs.Create("/usr/bin/gem")

	gemQueryResult := `
	   *** LOCAL GEMS ***

	   bar (1.2.3)
`

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(gemQueryResult), nil
	}

	config.SetVersion("1")

	ii := Get()

	assertion.Equal("test-hostname", ii.Hostname, "hostname does not match")
	assertion.Equal("Debian buster", ii.LongName, "long name does not match")
	assertion.Equal("test-kernel", ii.KernelVersion, "kernel version does not match")
	assertion.Equal("bar", ii.InstalledPackages.Gem[0].Name, "package name does not match")
	assertion.Equal("1.2.3", ii.InstalledPackages.Gem[0].Version, "package version does not match")

}

func TestInventoryInfoPip(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)
	AppFs.Create("/usr/bin/pip")

	pipQueryResult := `foo (1.2.3)
bar (1.2.3)
`

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(pipQueryResult), nil
	}

	config.SetVersion("1")

	ii := Get()

	assertion.Equal("test-hostname", ii.Hostname, "hostname does not match")
	assertion.Equal("Debian buster", ii.LongName, "long name does not match")
	assertion.Equal("test-kernel", ii.KernelVersion, "kernel version does not match")
	assertion.Equal("foo", ii.InstalledPackages.Pip[0].Name, "package name does not match")
	assertion.Equal("1.2.3", ii.InstalledPackages.Pip[0].Version, "package version does not match")

}

func TestInventoryInfoZypper(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `NAME="openSUSE Leap"
VERSION="15.0"
ID="opensuse-leap"
ID_LIKE="suse opensuse"
VERSION_ID="15.0"
PRETTY_NAME="openSUSE Leap 15.0"
ANSI_COLOR="0;32"
CPE_NAME="cpe:/o:opensuse:leap:15.0"
BUG_REPORT_URL="https://bugs.opensuse.org"
HOME_URL="https://www.opensuse.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)
	AppFs.Create("/usr/bin/zypper")

	zypperQueryResult := `
		Repository                          | Name                                        | Category    | Severity  | Interactive | Status     | Summary
		------------------------------------+---------------------------------------------+-------------+-----------+-------------+------------+------------------------------------------------------------
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1206 | security    | low       | ---         | applied    | Security update for bzip2
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1221 | security    | moderate  | ---         | applied    | Security update for libxslt
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1229 | recommended | moderate  | ---         | not needed | Recommended update for sensors
		SLE-Module-Basesystem15-SP1-Updates | SUSE-SLE-Module-Basesystem-15-SP1-2019-1258 | recommended | moderate  | ---         | needed     | Recommended update for postfix
`

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	// this is to test package updates
	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(zypperQueryResult), nil
	}

	config.SetVersion("1")

	ii := Get()

	assertion.Equal("test-hostname", ii.Hostname, "hostname does not match")
	assertion.Equal("openSUSE Leap 15.0", ii.LongName, "long name does not match")
	assertion.Equal("15.0", ii.Version, "version does not match")
	assertion.Equal("test-kernel", ii.KernelVersion, "kernel version does not match")
	assertion.Equal(2, len(ii.InstalledPackages.ZypperPatches), "unexpected number of patches")
	assertion.Equal("SUSE-SLE-Module-Basesystem-15-SP1-2019-1206", ii.InstalledPackages.ZypperPatches[0].Name, "package name does not match")
	assertion.Equal("security", ii.InstalledPackages.ZypperPatches[0].Category, "package category does not match")
	assertion.Equal("low", ii.InstalledPackages.ZypperPatches[0].Severity, "package severity does not match")

}

func TestInventoryInfoInstalledPackageQueryError(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	pkgManagerBinaries := []string{"/usr/bin/pip", "/usr/bin/gem", "/usr/bin/dpkg-query", "/usr/bin/rpmquery", "/usr/bin/zypper"}

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return nil, errors.New("error listing installed packages")
	}

	config.SetVersion("1")

	for _, bin := range pkgManagerBinaries {
		AppFs.Create(bin)
		ii := Get()
		s := reflect.ValueOf(&ii.InstalledPackages).Elem()
		for i := 0; i < s.NumField(); i++ {
			// assert that the there is no installed packages
			assertion.Nil(s.Field(i).Interface(), fmt.Sprintf("%s must fail", bin))
		}

		AppFs.Remove(bin)
	}
}

func TestInventoryInfoGetAptUpdates(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	aptupdates := `
		Reading package lists... Done
		Building dependency tree
		Reading state information... Done
		Calculating upgrade... Done
		The following NEW packages will be installed:
		  firmware-linux-free linux-image-4.9.0-9-amd64
		The following packages will be upgraded:
		  google-cloud-sdk linux-image-amd64
		2 upgraded, 2 newly installed, 0 to remove and 0 not upgraded.
		Inst libldap-common [2.4.45+dfsg-1ubuntu1.2] (2.4.45+dfsg-1ubuntu1.3 Ubuntu:18.04/bionic-updates, Ubuntu:18.04/bionic-security [all])
		Inst firmware-linux-free (3.4 Debian:9.9/stable [all])
		Inst google-cloud-sdk [245.0.0-0] (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		Inst linux-image-4.9.0-9-amd64 (4.9.168-1+deb9u2 Debian-Security:9/stable [amd64])
		Inst linux-image-amd64 [4.9+80+deb9u6] (4.9+80+deb9u7 Debian:9.9/stable [amd64])
		Conf firmware-linux-free (3.4 Debian:9.9/stable [all])
		Conf google-cloud-sdk (246.0.0-0 cloud-sdk-stretch:cloud-sdk-stretch [all])
		Conf linux-image-4.9.0-9-amd64 (4.9.168-1+deb9u2 Debian-Security:9/stable [amd64])
		Conf linux-image-amd64 (4.9+80+deb9u7 Debian:9.9/stable [amd64])
`
	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = true
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(aptupdates), nil
	}

	config.SetVersion("1")

	ii := Get()
	assertion.Equal(3, len(ii.PackageUpdates.Apt), "number of package updates does not match")
	assertion.Equal("libldap-common", ii.PackageUpdates.Apt[0].Name, "update package name does not match")
	assertion.Equal("2.4.45+dfsg-1ubuntu1.3", ii.PackageUpdates.Apt[0].Version, "update package version does not match")

}

func TestInventoryInfoGetGemUpdates(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	gemupdates := `
	   foo (1.2.8 < 1.3.2)
	   bar (1.0.0 < 1.1.2)
`
	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = true
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(gemupdates), nil
	}

	config.SetVersion("1")

	ii := Get()
	assertion.Equal(2, len(ii.PackageUpdates.Gem), "number of updates does not match")
	assertion.Equal("foo", ii.PackageUpdates.Gem[0].Name, "update package name does not match")
	assertion.Equal("1.3.2", ii.PackageUpdates.Gem[0].Version, "update package version does not match")

}

func TestInventoryInfoGetPipUpdates(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	pipupdates := `
	   foo (4.5.3) - Latest: 4.6.0 [repo]
	   bar (1.3) - Latest: 1.4 [repo]
`
	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = false
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = true
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(pipupdates), nil
	}

	config.SetVersion("1")

	ii := Get()
	assertion.Equal(2, len(ii.PackageUpdates.Pip), "number of updates does not match")
	assertion.Equal("foo", ii.PackageUpdates.Pip[0].Name, "update package name does not match")
	assertion.Equal("4.6.0", ii.PackageUpdates.Pip[0].Version, "update package version does not match")

}

func TestInventoryInfoGetZypperUpdates(t *testing.T) {
	var AppFs = afero.NewMemMapFs()
	assertion := assert.New(t)
	releaseFile := "/etc/os-release"
	AppFs.Create(releaseFile)
	fcontent := `PRETTY_NAME="Debian buster"
NAME="Debian GNU/Linux"
VERSION_ID="10"
VERSION="10 (buster)"
VERSION_CODENAME=buster
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

	afero.WriteFile(AppFs, releaseFile, []byte(fcontent), 644)

	common.OsHostname = func() (string, error) {
		return "test-hostname", nil
	}

	osinfo.GetUname = func() ([]byte, error) {
		return []byte("test-kernel"), nil
	}

	common.ReadFile = func(file string) ([]byte, error) {
		return []byte(fcontent), nil
	}

	zypperupdates := `
		      S | Repository          | Name                   | Current Version | Available Version | Arch
		      --+---------------------+------------------------+-----------------+-------------------+-------
		      v | SLES12-SP3-Updates  | at                     | 3.1.14-7.3      | 3.1.14-8.3.1      | x86_64
		      v | SLES12-SP3-Updates  | autoyast2-installation | 3.2.17-1.3      | 3.2.22-2.9.2      | noarch
`
	common.Exists = func(name string) bool {
		if _, err := AppFs.Stat(name); err != nil {
			return false
		}
		return true
	}

	packages.ZypperExists = true
	packages.AptExists = false
	packages.GemExists = false
	packages.PipExists = false
	packages.YumExists = false

	osinfo.Architecture = func(arch string) string {
		return "x86_64"
	}

	common.Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
		return []byte(zypperupdates), nil
	}

	config.SetVersion("1")

	ii := Get()

	assertion.Equal(2, len(ii.PackageUpdates.Zypper), "number of updates does not match")
	assertion.Equal("at", ii.PackageUpdates.Zypper[0].Name, "update package name does not match")
	assertion.Equal("3.1.14-8.3.1", ii.PackageUpdates.Zypper[0].Version, "update package version does not match")

}
