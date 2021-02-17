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
	"time"

	daisyCompute "github.com/GoogleCloudPlatform/compute-image-tools/daisy/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	api "google.golang.org/api/compute/v1"
	"google.golang.org/grpc/status"
)

var (
	yumInstallAgent = `
sleep 10
systemctl stop google-osconfig-agent
stop -q -n google-osconfig-agent  # required for EL6
while ! yum install -y google-osconfig-agent; do
if [[ n -gt 3 ]]; then
  exit 1
fi
n=$[$n+1]
sleep 5
done
systemctl start google-osconfig-agent
start -q -n google-osconfig-agent  # required for EL6` + CurlPost

	zypperInstallAgent = `
sleep 10
systemctl stop google-osconfig-agent
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

	// CurlPost indicates agent is installed.
	CurlPost = `
uri=http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/osconfig_tests/install_done
curl -X PUT --data "1" $uri -H "Metadata-Flavor: Google"
`

	windowsPost = `
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
repo_gpgcheck=1
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
repo_gpgcheck=1
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
		return `c:\programdata\googet\googet.exe -noconfirm remove google-osconfig-agent
c:\programdata\googet\googet.exe -noconfirm install google-osconfig-agent` + windowsPost
	}
	return fmt.Sprintf(`c:\programdata\googet\googet.exe -noconfirm remove google-osconfig-agent
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

// InstallOSConfigEL8 installs the osconfig agent on el8 based systems.
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

// InstallOSConfigEL6 installs the osconfig agent on el6 based systems.
func InstallOSConfigEL6() string {
	if config.AgentRepo() == "" {
		return CurlPost
	}
	if config.AgentRepo() == "stable" {
		return yumInstallAgent
	}
	if config.AgentRepo() == "staging" {
		return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el6", config.AgentRepo(), 1)
	}
	return fmt.Sprintf(yumRepoSetup+yumInstallAgent, "el6", config.AgentRepo(), 0)
}

// HeadAptImages is a map of names to image paths for public image families that use APT.
var HeadAptImages = map[string]string{
	// Debian images.
	"debian-cloud/debian-9":  "projects/debian-cloud/global/images/family/debian-9",
	"debian-cloud/debian-10": "projects/debian-cloud/global/images/family/debian-10",

	// Ubuntu images.
	"ubuntu-os-cloud/ubuntu-1604-lts": "projects/ubuntu-os-cloud/global/images/family/ubuntu-1604-lts",
	"ubuntu-os-cloud/ubuntu-1804-lts": "projects/ubuntu-os-cloud/global/images/family/ubuntu-1804-lts",
	"ubuntu-os-cloud/ubuntu-2004-lts": "projects/ubuntu-os-cloud/global/images/family/ubuntu-2004-lts",
}

// OldAptImages is a map of names to image paths for old images that use APT.
var OldAptImages = map[string]string{
	// Debian images.
	"old/debian-9":  "projects/debian-cloud/global/images/debian-9-stretch-v20191014",
	"old/debian-10": "projects/debian-cloud/global/images/debian-10-buster-v20191014",

	// Ubuntu images.
	"old/ubuntu-1604-lts": "projects/ubuntu-os-cloud/global/images/ubuntu-1604-xenial-v20191005",
	"old/ubuntu-1804-lts": "projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20191002",
	"old/ubuntu-2004-lts": "projects/ubuntu-os-cloud/global/images/ubuntu-2004-focal-v20200506",
}

// HeadSUSEImages is a map of names to image paths for public SUSE images.
var HeadSUSEImages = map[string]string{
	"suse-cloud/sles-12": "projects/suse-cloud/global/images/family/sles-12",
	"suse-cloud/sles-15": "projects/suse-cloud/global/images/family/sles-15",

	"opensuse-cloud/opensuse-leap": "projects/opensuse-cloud/global/images/family/opensuse-leap",
}

// OldSUSEImages is a map of names to image paths for old SUSE images.
var OldSUSEImages = map[string]string{
	"old/sles-12": "projects/compute-image-tools-test/global/images/sles-12-sp5-v20191209",
	"old/sles-15": "projects/compute-image-tools-test/global/images/sles-15-sp1-v20190625",

	"old/opensuse-leap": "projects/opensuse-cloud/global/images/opensuse-leap-15-1-v20190618",
}

// HeadEL6Images is a map of names to image paths for public EL6 images, we use the last
// published image here as EL6 is EOL.
// TODO: Either remove support for el6 or move this to a deprected test suite.
var HeadEL6Images = map[string]string{
	"rhel-cloud/rhel-6": "projects/rhel-cloud/global/images/rhel-6-v20201112",
}

// OldEL6Images is a map of names to image paths for old EL6 images.
var OldEL6Images = map[string]string{
	"old/rhel-6": "projects/rhel-cloud/global/images/rhel-6-v20191014",
}

// HeadEL7Images is a map of names to image paths for public EL7 image families.
var HeadEL7Images = map[string]string{
	"centos-cloud/centos-7": "projects/centos-cloud/global/images/family/centos-7",
	"rhel-cloud/rhel-7":     "projects/rhel-cloud/global/images/family/rhel-7",
}

// OldEL7Images is a map of names to image paths for old EL7 images.
var OldEL7Images = map[string]string{
	"old/centos-7": "projects/centos-cloud/global/images/centos-7-v20191014",
	"old/rhel-7":   "projects/rhel-cloud/global/images/rhel-7-v20191014",
}

// HeadEL8Images is a map of names to image paths for public EL8 image families.
var HeadEL8Images = map[string]string{
	"centos-cloud/centos-8": "projects/centos-cloud/global/images/family/centos-8",
	"rhel-cloud/rhel-8":     "projects/rhel-cloud/global/images/family/rhel-8",
}

// OldEL8Images is a map of names to image paths for old EL8 images.
var OldEL8Images = map[string]string{
	"old/centos-8": "projects/centos-cloud/global/images/centos-7-v20191014",
	"old/rhel-8":   "projects/rhel-cloud/global/images/rhel-7-v20191014",
}

// HeadELImages is a map of names to image paths for public EL image families.
var HeadELImages = func() (newMap map[string]string) {
	newMap = make(map[string]string)
	for k, v := range HeadEL6Images {
		newMap[k] = v
	}
	for k, v := range HeadEL7Images {
		newMap[k] = v
	}
	for k, v := range HeadEL8Images {
		newMap[k] = v
	}
	return
}()

// HeadWindowsImages is a map of names to image paths for public Windows image families.
var HeadWindowsImages = map[string]string{
	"windows-cloud/windows-2012-r2":      "projects/windows-cloud/global/images/family/windows-2012-r2",
	"windows-cloud/windows-2012-r2-core": "projects/windows-cloud/global/images/family/windows-2012-r2-core",
	"windows-cloud/windows-2016":         "projects/windows-cloud/global/images/family/windows-2016",
	"windows-cloud/windows-2016-core":    "projects/windows-cloud/global/images/family/windows-2016-core",
	"windows-cloud/windows-2019":         "projects/windows-cloud/global/images/family/windows-2019",
	"windows-cloud/windows-2019-core":    "projects/windows-cloud/global/images/family/windows-2019-core",
	"windows-cloud/windows-1909-core":    "projects/windows-cloud/global/images/family/windows-1909-core",
	"windows-cloud/windows-2004-core":    "projects/windows-cloud/global/images/family/windows-2004-core",
}

// OldWindowsImages is a map of names to image paths for old Windows images.
var OldWindowsImages = map[string]string{
	"old/windows-2012-r2":      "projects/windows-cloud/global/images/windows-server-2012-r2-dc-v20191008",
	"old/windows-2012-r2-core": "projects/windows-cloud/global/images/windows-server-2012-r2-dc-core-v20191008",
	"old/windows-2016":         "projects/windows-cloud/global/images/windows-server-2016-dc-v20191008",
	"old/windows-2016-core":    "projects/windows-cloud/global/images/windows-server-2016-dc-core-v20191008",
	"old/windows-2019":         "projects/windows-cloud/global/images/windows-server-2019-dc-v20191008",
	"old/windows-2019-core":    "projects/windows-cloud/global/images/windows-server-2019-dc-core-v20191008",
}

// HeadCOSImages is a map of names to image paths for public COS image families.
var HeadCOSImages = map[string]string{
	"cos-cloud/cos-stable": "projects/cos-cloud/global/images/family/cos-stable",
	"cos-cloud/cos-beta":   "projects/cos-cloud/global/images/family/cos-beta",
	"cos-cloud/cos-dev":    "projects/cos-cloud/global/images/family/cos-dev",
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
					DiskType:    fmt.Sprintf("projects/%s/zones/%s/diskTypes/pd-ssd", projectID, zone),
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
