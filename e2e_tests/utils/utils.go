//  Copyright 2019 Google Inc. All Rights Reserved.
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

// Package utils contains helper utils for osconfig_tests.
package utils

import (
	"fmt"
	"math/rand"
	"path"
	"strings"
	"time"

	daisyCompute "github.com/GoogleCloudPlatform/compute-image-tools/daisy/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	api "google.golang.org/api/compute/v1"
	"google.golang.org/grpc/status"
)

var (
	yumInstallAgent = `
sed -i 's/repo_gpgcheck=1/repo_gpgcheck=0/g' /etc/yum.repos.d/google-cloud.repo
sleep 10
systemctl stop google-osconfig-agent
while ! yum install -y google-osconfig-agent; do
if [[ n -gt 3 ]]; then
  exit 1
fi
n=$[$n+1]
sleep 5
done
systemctl start google-osconfig-agent` + CurlPost

	zypperInstallAgent = `
sleep 10
systemctl stop google-osconfig-agent
zypper -n remove google-osconfig-agent
while ! zypper -n -i --no-gpg-checks install google-osconfig-agent; do
if [[ n -gt 2 ]]; then
  # Zypper repos are flaky, we retry 3 times then just continue, the agent may be installed fine.
  zypper --no-refresh -n -i --no-gpg-checks install google-osconfig-agent
  break
fi
n=$[$n+1]
sleep 5
done
systemctl start google-osconfig-agent` + CurlPost

	// CosSetup sets up serial logging on COS.
	CosSetup = `
sleep 10
sed -i 's/^#ForwardToConsole=no/ForwardToConsole=yes/' /etc/systemd/journald.conf
sed -i 's/^#MaxLevelConsole=info/MaxLevelConsole=debug/' /etc/systemd/journald.conf
MaxLevelConsole=debug
systemctl force-reload systemd-journald
systemctl restart google-osconfig-agent` + CurlPost

	// CurlPost indicates agent is installed.
	CurlPost = `
uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/guestInventory/LastUpdated
curl -X DELETE $uri -H "Metadata-Flavor: Google"

uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/osconfig_tests/install_done
curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
`

	windowsPost = `
$uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/guestInventory/LastUpdated'
Invoke-RestMethod -Method DELETE -Uri $uri -Headers @{"Metadata-Flavor" = "Google"}
Start-Sleep 10
$uri = 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/osconfig_tests/install_done'
Invoke-RestMethod -Method PUT -Uri $uri -Headers @{"Metadata-Flavor" = "Google"} -Body 1
`

	yumRepoSetup = `
cat > /etc/yum.repos.d/google-osconfig-agent.repo <<EOM
[google-osconfig-agent]
name=Google OSConfig Agent Repository
baseurl=https://packages.cloud.google.com/yum/repos/google-osconfig-agent-%s-%s
enabled=1
gpgcheck=%d
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
           https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM`

	zypperRepoSetup = `
cat > /etc/zypp/repos.d/google-osconfig-agent.repo <<EOM
[google-osconfig-agent]
name=Google OSConfig Agent Repository
baseurl=https://packages.cloud.google.com/yum/repos/google-osconfig-agent-%s-%s
enabled=1
gpgcheck=%d
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
           https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM`
)

// InstallOSConfigDeb installs the osconfig agent on deb based systems.
func InstallOSConfigDeb() string {
	if config.AgentRepo() == "" {
		return CurlPost
	}
	return fmt.Sprintf(`
sleep 10
systemctl stop google-osconfig-agent
echo 'deb http://packages.cloud.google.com/apt google-osconfig-agent-%s main' >> /etc/apt/sources.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
apt-get update
apt-get install -y google-osconfig-agent
systemctl start google-osconfig-agent`+CurlPost, config.AgentRepo())
}

// InstallOSConfigGooGet installs the osconfig agent on Windows systems.
func InstallOSConfigGooGet() string {
	if config.AgentRepo() == "" {
		return windowsPost
	}
	if config.AgentRepo() == "stable" {
		return `
c:\programdata\googet\googet.exe -noconfirm remove google-osconfig-agent
c:\programdata\googet\googet.exe -noconfirm install google-osconfig-agent` + windowsPost
	}
	return fmt.Sprintf(`
c:\programdata\googet\googet.exe -noconfirm remove google-osconfig-agent
c:\programdata\googet\googet.exe -noconfirm install -sources https://packages.cloud.google.com/yuck/repos/google-osconfig-agent-%s google-osconfig-agent
`+windowsPost, config.AgentRepo())
}

// InstallOSConfigSUSE installs the osconfig agent on suse systems.
func InstallOSConfigSUSE() string {
	if config.AgentRepo() == "" {
		return ""
	}
	if config.AgentRepo() == "staging" || config.AgentRepo() == "stable" {
		return fmt.Sprintf(zypperRepoSetup+zypperInstallAgent, "el8", config.AgentRepo(), 1)
	}
	return fmt.Sprintf(zypperRepoSetup+zypperInstallAgent, "el8", config.AgentRepo(), 0)
}

// InstallOSConfigEL9 installs the osconfig agent on el9 based systems. (RHEL)
func InstallOSConfigEL9() string {
	if config.AgentRepo() == "" {
		return CurlPost
	}
	if config.AgentRepo() == "stable" {
		return yumInstallAgent
	}
	if config.AgentRepo() == "staging" {
		return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el9", config.AgentRepo(), 1)
	}
	return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el9", config.AgentRepo(), 0)
}

// InstallOSConfigEL8 installs the osconfig agent on el8 based systems. (RHEL)
func InstallOSConfigEL8() string {
	if config.AgentRepo() == "" {
		return CurlPost
	}
	if config.AgentRepo() == "stable" {
		return yumInstallAgent
	}
	if config.AgentRepo() == "staging" {
		return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el8", config.AgentRepo(), 1)
	}
	return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el8", config.AgentRepo(), 0)
}

// InstallOSConfigEL7 installs the osconfig agent on el7 based systems.
func InstallOSConfigEL7() string {
	if config.AgentRepo() == "" {
		return CurlPost
	}
	if config.AgentRepo() == "stable" {
		return yumInstallAgent
	}
	if config.AgentRepo() == "staging" {
		return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el7", config.AgentRepo(), 1)
	}
	return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el7", config.AgentRepo(), 0)
}

// InstallOSConfigEL installs the osconfig agent on el based systems.
func InstallOSConfigEL(image string) string {
	switch {
	case strings.Contains(path.Base(image), "9"):
		return InstallOSConfigEL9()
	case strings.Contains(path.Base(image), "8"):
		return InstallOSConfigEL8()
	case strings.Contains(path.Base(image), "7"):
		return InstallOSConfigEL7()

	}
	return ""
}

// DowngradeAptImages is a single image that are used for testing downgrade case with apt-get
var DowngradeAptImages = map[string]string{
	"debian-cloud/debian-11": "projects/debian-cloud/global/images/debian-11-bullseye-v20231010",
}

// HeadAptImages is a map of names to image paths for public image families that use APT.
var HeadAptImages = map[string]string{
	// Debian images.
	"debian-cloud/debian-11": "projects/debian-cloud/global/images/debian-11-bullseye-v20231010",
	"debian-cloud/debian-12": "projects/debian-cloud/global/images/debian-12-bookworm-v20231010",

	// Ubuntu images.
	"ubuntu-os-cloud/ubuntu-2304": "projects/ubuntu-os-cloud/global/images/ubuntu-2304-lunar-amd64-v20231020",
	"ubuntu-os-cloud/ubuntu-2314": "projects/ubuntu-os-cloud/global/images/ubuntu-2310-mantic-amd64-v20231011",
}

// OldAptImages is a map of names to image paths for old (deprecated) images that use APT.
var OldAptImages = map[string]string{
	// Debian images.

	// Ubuntu images.
	"old/ubuntu-2004": "projects/ubuntu-os-cloud/global/images/ubuntu-2004-focal-v20230918",
}

// HeadSUSEImages is a map of names to image paths for public SUSE images.
var HeadSUSEImages = map[string]string{
	"suse-cloud/sles-12-sp5": "projects/suse-cloud/global/images/sles-12-sp5-v20230807-x86-64",
	"suse-cloud/sles-15-sp5": "projects/suse-cloud/global/images/sles-15-sp5-v20230921-x86-64",

	"suse-sap-cloud/sles-12-sp5-sap": "projects/suse-sap-cloud/global/images/sles-12-sp5-sap-v20231019-x86-64",
	"suse-sap-cloud/sles-15-sp5-sap": "projects/suse-sap-cloud/global/images/sles-15-sp5-sap-v20230921-x86-64",

	"suse-sap-cloud/sles-15-sp4-hardened-sap": "projects/suse-sap-cloud/global/images/sles-sap-15-sp4-hardened-v20230828-x86-64",
	"suse-sap-cloud/sles-15-sp5-hardened-sap": "projects/suse-sap-cloud/global/images/sles-sap-15-sp5-hardened-v20230921-x86-64",

	"opensuse-cloud/opensuse-leap-15-4": "projects/opensuse-cloud/global/images/opensuse-leap-15-4-v20230907-x86-64",
	"opensuse-cloud/opensuse-leap-15-5": "projects/opensuse-cloud/global/images/opensuse-leap-15-5-v20230908-x86-64",
}

// OldSUSEImages is a map of names to image paths for old SUSE images.
var OldSUSEImages = map[string]string{
	"old/sles-15-sp1-sap": "projects/suse-sap-cloud/global/images/sles-15-sp1-sap-v20221108-x86-64",
	"old/sles-15-sp2-sap": "projects/suse-sap-cloud/global/images/sles-15-sp2-sap-v20221108-x86-64",
	"old/sles-15-sp3-sap": "projects/suse-sap-cloud/global/images/sles-15-sp3-sap-v20221108-x86-64",
	"old/sles-15-sp4-sap": "projects/suse-sap-cloud/global/images/sles-15-sp4-sap-v20230623-x86-64",
}

// HeadEL7Images is a map of names to image paths for public EL7 image families. (RHEL, CentOS)
var HeadEL7Images = map[string]string{
	"centos-cloud/centos-7": "projects/centos-cloud/global/images/centos-7-v20231010",

	"rhel-cloud/rhel-7": "projects/rhel-cloud/global/images/rhel-7-v20231010",

	"rhel-sap-cloud/rhel-7-sap": "projects/rhel-sap-cloud/global/images/rhel-7-9-sap-v20231011",
}

// OldEL7Images is a map of names to image paths for old EL7 images.
var OldEL7Images = map[string]string{
	// Currently empty
}

// HeadEL8Images is a map of names to image paths for public EL8 image families. (RHEL, CentOS, Rocky)
var HeadEL8Images = map[string]string{
	"centos-cloud/centos-stream-8": "projects/centos-cloud/global/images/centos-stream-8-v20231010",

	"rhel-cloud/rhel-8": "projects/rhel-cloud/global/images/rhel-8-v20231010",

	"rhel-sap-cloud/rhel-8-1-sap": "projects/rhel-sap-cloud/global/images/rhel-8-1-sap-v20231010",
	"rhel-sap-cloud/rhel-8-2-sap": "projects/rhel-sap-cloud/global/images/rhel-8-2-sap-v20231010",
	"rhel-sap-cloud/rhel-8-4-sap": "projects/rhel-sap-cloud/global/images/rhel-8-4-sap-v20231010",
	"rhel-sap-cloud/rhel-8-6-sap": "projects/rhel-sap-cloud/global/images/rhel-8-6-sap-v20231010",
	"rhel-sap-cloud/rhel-8-8-sap": "projects/rhel-sap-cloud/global/images/rhel-8-8-sap-v20231010",

	"rocky-linux-cloud/rocky-linux-8":         "projects/rocky-linux-cloud/global/images/rocky-linux-8-v20231010",
	"rocky-linux-cloud/rocky-linux-8-opt-gcp": "projects/rocky-linux-cloud/global/images/rocky-linux-8-optimized-gcp-v20231010",
}

// OldEL8Images is a map of names to image paths for old EL8 images. (RHEL, CentOS, Rocky)
var OldEL8Images = map[string]string{
	// Currently empty
}

// HeadEL9Images is a map of names to image paths for public EL9 image families. (RHEL, CentOS, Rocky)
var HeadEL9Images = map[string]string{
	"centos-cloud/centos-stream-9": "projects/centos-cloud/global/images/centos-stream-9-v20231010",

	"rhel-cloud/rhel-9": "projects/rhel-cloud/global/images/rhel-9-v20231010",

	"rhel-sap-cloud/rhel-9-0-sap": "projects/rhel-sap-cloud/global/images/rhel-9-0-sap-v20231010",
	"rhel-sap-cloud/rhel-9-2-sap": "projects/rhel-sap-cloud/global/images/rhel-9-2-sap-v20231010",

	"rocky-linux-cloud/rocky-linux-9":         "projects/rocky-linux-cloud/global/images/rocky-linux-9-v20231010",
	"rocky-linux-cloud/rocky-linux-9-opt-gcp": "projects/rocky-linux-cloud/global/images/rocky-linux-9-optimized-gcp-v20231010",
}

// OldEL9Images is a map of names to image paths for old EL9 images. (RHEL, CentOS, Rocky)
var OldEL9Images = map[string]string{
	// Currently empty
}

// HeadELImages is a map of names to image paths for public EL image families. (RHEL, CentOS, Rocky)
var HeadELImages = func() (newMap map[string]string) {
	newMap = make(map[string]string)
	for k, v := range HeadEL7Images {
		newMap[k] = v
	}
	for k, v := range HeadEL8Images {
		newMap[k] = v
	}
	for k, v := range HeadEL9Images {
		newMap[k] = v
	}
	return
}()

// HeadWindowsImages is a map of names to image paths for public Windows image families.
var HeadWindowsImages = map[string]string{
	"windows-cloud/win-2016-dc-core": "projects/windows-cloud/global/images/windows-server-2016-dc-core-v20231011",
	"windows-cloud/win-2016-dc":      "projects/windows-cloud/global/images/windows-server-2016-dc-v20231011",
	"windows-cloud/win-2019-dc-core": "projects/windows-cloud/global/images/windows-server-2019-dc-core-v20231011",
	"windows-cloud/win-2019-dc":      "projects/windows-cloud/global/images/windows-server-2019-dc-v20231011",
	"windows-cloud/win-2022-dc-core": "projects/windows-cloud/global/images/windows-server-2022-dc-core-v20231011",
	"windows-cloud/win-2022-dc":      "projects/windows-cloud/global/images/windows-server-2022-dc-v20231011",
}

// OldWindowsImages is a map of names to image paths for old Windows images.
var OldWindowsImages = map[string]string{
	// Currently empty
}

// HeadCOSImages is a map of names to image paths for public COS image families.
var HeadCOSImages = map[string]string{
	"cos-cloud/cos-stable": "projects/cos-cloud/global/images/cos-stable-109-17800-0-51",
	"cos-cloud/cos-beta":   "projects/cos-cloud/global/images/cos-beta-109-17800-0-51",
	"cos-cloud/cos-dev":    "projects/cos-cloud/global/images/cos-dev-113-17965-0-0",
}

// RandString generates a random string of n length.
func RandString(n int) string {
	gen := rand.New(rand.NewSource(time.Now().UnixNano()))
	letters := "bdghjlmnpqrstvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[gen.Int63()%int64(len(letters))]
	}
	return string(b)
}

// GetStatusFromError return a string that contains all information
// about the error that is created from a status
func GetStatusFromError(err error) string {
	if s, ok := status.FromError(err); ok {
		return fmt.Sprintf("code: %q, message: %q, details: %q", s.Code(), s.Message(), s.Details())
	}
	return fmt.Sprintf("%v", err)
}

// This pool is just used for CreateComputeInstance so that we limit our calls to the API during the heavy create process.
var pool = make(chan struct{}, 10)

// CreateComputeInstance is an utility function to create gce instance
func CreateComputeInstance(metadataitems []*api.MetadataItems, client daisyCompute.Client, machineType, image, name, projectID, zone, serviceAccountEmail string, serviceAccountScopes []string) (*compute.Instance, error) {
	pool <- struct{}{}
	defer func() {
		<-pool
	}()
	var items []*api.MetadataItems

	// enable debug logging and guest-attributes for all test instances
	items = append(items, compute.BuildInstanceMetadataItem("enable-os-config-debug", "true"))
	items = append(items, compute.BuildInstanceMetadataItem("enable-guest-attributes", "true"))
	if config.AgentSvcEndpoint() != "" {
		items = append(items, compute.BuildInstanceMetadataItem("os-config-endpoint", config.AgentSvcEndpoint()))
	}

	for _, item := range metadataitems {
		items = append(items, item)
	}

	i := &api.Instance{
		Name:        name,
		MachineType: fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s", projectID, zone, machineType),
		NetworkInterfaces: []*api.NetworkInterface{
			{
				Subnetwork: fmt.Sprintf("projects/%s/regions/%s/subnetworks/default", projectID, zone[:len(zone)-2]),
				AccessConfigs: []*api.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
					},
				},
			},
		},
		Metadata: &api.Metadata{
			Items: items,
		},
		Disks: []*api.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &api.AttachedDiskInitializeParams{
					SourceImage: image,
					DiskType:    fmt.Sprintf("projects/%s/zones/%s/diskTypes/pd-balanced", projectID, zone),
				},
			},
		},
		ServiceAccounts: []*api.ServiceAccount{
			{
				Email:  serviceAccountEmail,
				Scopes: serviceAccountScopes,
			},
		},
		Labels: map[string]string{"name": name},
	}

	inst, err := compute.CreateInstance(client, projectID, zone, i)
	if err != nil {
		return nil, err
	}

	return inst, nil
}
