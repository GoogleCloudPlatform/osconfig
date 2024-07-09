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
	testingPkgsProjectName = "gce-pkg-osconfig-testing"

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
)

// getRegionFromZone extracts the region from a zone name.
// Example: If zone is "us-central1-a", this would return "us-central1"
func getRegionFromZone(zone string) string {
	parts := strings.Split(zone, "-")
	return strings.Join(parts[:len(parts)-1], "-")
}

// pickTestRegionForArtifactRegistry selects a random zone from the configured zones to pull osconfig-agent package from AR & selected-region
func pickTestRegionForArtifactRegistry() string {
	zones := config.Zones()

	if len(zones) == 0 {
		// default region for tests
		return "us-central1"
	}

	zoneKeys := make([]string, 0, len(zones))
	for k := range zones {
		zoneKeys = append(zoneKeys, k)
	}
	randomIndex := rand.Intn(len(zoneKeys))
	randomZone := zoneKeys[randomIndex]

	return getRegionFromZone(randomZone)
}

// getRepoLineForApt returns the repo line that should be added to apt sources.list file
func getRepoLineForApt(osName string) string {
	repo := config.AgentRepo()
	if repo == "testing" {
		testRegion := pickTestRegionForArtifactRegistry()
		return fmt.Sprintf("deb ar+https://%s-apt.pkg.dev/projects/%s google-osconfig-agent-%s-testing main",
			testRegion, testingPkgsProjectName, osName)
	}
	return fmt.Sprintf("deb http://packages.cloud.google.com/apt google-osconfig-agent-%s-%s main", osName, repo)
}

// InstallOSConfigDeb installs the osconfig agent on deb based systems.
func InstallOSConfigDeb(image string) string {
	if config.AgentRepo() == "" {
		return CurlPost
	}
	osName := getDebOsName(image)
	return fmt.Sprintf(`
sleep 10
systemctl stop google-osconfig-agent

# install gnupg2 if not exist
apt-get update
apt-get install -y gnupg2

# install apt-transport-artifact-registry
apt-get install -y apt-transport-artifact-registry

echo '%s' >> /etc/apt/sources.list

curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
apt-get update
apt-get install -y google-osconfig-agent
systemctl start google-osconfig-agent`+CurlPost, getRepoLineForApt(osName))
}

// InstallOSConfigGooGet installs the osconfig agent on Windows systems.
func InstallOSConfigGooGet() string {
	if config.AgentRepo() == "" {
		return windowsPost
	}
	removeAgentCmd := `c:\programdata\googet\googet.exe -noconfirm remove google-osconfig-agent`
	if config.AgentRepo() == "stable" {
		return fmt.Sprintf(`
%s
c:\programdata\googet\googet.exe -noconfirm install google-osconfig-agent`, removeAgentCmd) + windowsPost
	} else if config.AgentRepo() == "testing" {
		testRegion := pickTestRegionForArtifactRegistry()
		agentRepo := config.AgentRepo()
		return fmt.Sprintf(`
%s
c:\programdata\googet\googet.exe addrepo google-osconfig-agent-googet-testing https://%s-googet.pkg.dev/projects/%s/repos/google-osconfig-agent-googet-%s

# set useoauth to true in repo new file
$filePath = 'C:\ProgramData\GooGet\repos\google-osconfig-agent-googet-testing.repo'
(Get-Content $filePath) -replace 'useoauth: false', 'useoauth: true' | Set-Content $filePath

c:\programdata\googet\googet.exe -noconfirm install google-osconfig-agent`+windowsPost, removeAgentCmd, testRegion, testingPkgsProjectName, agentRepo)
	}
	return fmt.Sprintf(`
%s
c:\programdata\googet\googet.exe -noconfirm install -sources https://packages.cloud.google.com/yuck/repos/google-osconfig-agent-%s google-osconfig-agent
`+windowsPost, removeAgentCmd, config.AgentRepo())
}

// getYumRepoBaseURL returns the repo baseUrl that should be added to repo file google-osconfig-agent.repo
func getYumRepoBaseURL(osType string) string {
	agentRepo := config.AgentRepo()
	if agentRepo == "testing" {
		testRegion := pickTestRegionForArtifactRegistry()
		return fmt.Sprintf("https://%s-yum.pkg.dev/projects/%s/google-osconfig-agent-%s-testing", testRegion, testingPkgsProjectName, osType)
	}
	return fmt.Sprintf("https://packages.cloud.google.com/yum/repos/google-osconfig-agent-%s-%s", osType, agentRepo)
}

func getYumRepoSetup(osType string) string {
	gpgcheck := 1

	// According to doc, pkg name differ according to ELv version
	// doc: https://cloud.google.com/artifact-registry/docs/os-packages/rpm/configure#prepare-yum
	format := "dnf"
	if osType == "el7" {
		format = "yum"
	}

	repoConfig := fmt.Sprintf(`
# install yum-plugin-artifact-registry
yum makecache
yum install -y %s-plugin-artifact-registry

cat > /etc/yum.repos.d/google-osconfig-agent.repo <<EOM
[google-osconfig-agent]
name=Google OSConfig Agent Repository
baseurl=%s
enabled=1
gpgcheck=%d
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
			https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM`, format, getYumRepoBaseURL(osType), gpgcheck)

	return repoConfig
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
		return getYumRepoSetup("el9") + yumInstallAgent
	}
	return getYumRepoSetup("el9") + yumInstallAgent
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
		return getYumRepoSetup("el8") + yumInstallAgent
	}
	return getYumRepoSetup("el8") + yumInstallAgent
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
		return getYumRepoSetup("el7") + yumInstallAgent
	}
	return getYumRepoSetup("el7") + yumInstallAgent
}

// containsAnyOf checks if a string contains any substring from a given list.
func containsAnyOf(str string, substrings []string) bool {
	for _, substring := range substrings {
		if strings.Contains(str, substring) {
			return true
		}
	}
	return false
}

// InstallOSConfigEL installs the osconfig agent on el based systems.
func InstallOSConfigEL(image string) string {
	imageName := path.Base(image)
	switch {
	case image == "9" || containsAnyOf(imageName, []string{"rhel-9", "rhel-sap-9", "centos-stream-9", "rocky-linux-9"}):
		return InstallOSConfigEL9()
	case image == "8" || containsAnyOf(imageName, []string{"rhel-8", "rhel-sap-8", "centos-stream-8", "rocky-linux-8"}):
		return InstallOSConfigEL8()
	case image == "7" || containsAnyOf(imageName, []string{"rhel-7", "rhel-sap-7", "centos-7"}):
		return InstallOSConfigEL7()
	}
	return ""
}

func getZypperRepoSetup(osType string) string {

	gpgcheck := 1

	// TODO: Allow SUSE tests to pull packages from test project.
	repoConfig := fmt.Sprintf(`
cat > /etc/zypp/repos.d/google-osconfig-agent.repo <<EOM
[google-osconfig-agent]
name=Google OSConfig Agent Repository
baseurl=%s
enabled=1
gpgcheck=%d
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
			https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM`, getYumRepoBaseURL(osType), gpgcheck)

	return repoConfig
}

// InstallOSConfigSUSE installs the osconfig agent on suse systems.
func InstallOSConfigSUSE() string {
	if config.AgentRepo() == "" {
		return ""
	}
	if config.AgentRepo() == "staging" || config.AgentRepo() == "stable" {
		return getZypperRepoSetup("el8") + zypperInstallAgent
	}
	return getZypperRepoSetup("el8") + zypperInstallAgent
}

// getDebOsType returns the equivalent os_name for deb version (e.g. debian-11 --> bullseye)
func getDebOsName(image string) string {
	imageName := path.Base(image)
	switch {
	case image == "10" || containsAnyOf(imageName, []string{"debian-10", "buster"}):
		return "buster"
	case image == "11" || containsAnyOf(imageName, []string{"debian-11", "bullseye"}):
		return "bullseye"
	case image == "12" || containsAnyOf(imageName, []string{"debian-12", "bookworm"}):
		return "bookworm"
	}
	return ""
}

// DowngradeBullseyeAptImages is a single image that are used for testing downgrade case with apt-get
var DowngradeBullseyeAptImages = map[string]string{
	"debian-cloud/debian-11": "projects/debian-cloud/global/images/debian-11-bullseye-v20231010",
}

// HeadBusterAptImages empty for now as debian-10 wil reach EOL and some of its repos are not reachable anymore.
var HeadBusterAptImages = map[string]string{
	// Debian images.
}

// HeadBullseyeAptImages is a map of names to image paths for public debian-11 images
var HeadBullseyeAptImages = map[string]string{
	// Debian images.
	"debian-cloud/debian-11": "projects/debian-cloud/global/images/family/debian-11",
}

// HeadBookwormAptImages is a map of names to image paths for public debian-12 images
var HeadBookwormAptImages = map[string]string{
	// Debian images.
	"debian-cloud/debian-12": "projects/debian-cloud/global/images/family/debian-12",
}

// HeadSUSEImages is a map of names to image paths for public SUSE images.
var HeadSUSEImages = func() map[string]string {
	imgsMap := make(map[string]string)

	// TODO: enable SUSE tests to use testing pkgs after Artifact Registry supports zypper installation from private repos
	if config.AgentRepo() != "testing" {
		imgsMap = map[string]string{
			"suse-cloud/sles-12-sp5": "projects/suse-cloud/global/images/family/sles-12",
			"suse-cloud/sles-15-sp5": "projects/suse-cloud/global/images/family/sles-15",

			"suse-sap-cloud/sles-12-sp5-sap": "projects/suse-sap-cloud/global/images/family/sles-12-sp5-sap",
			"suse-sap-cloud/sles-15-sp5-sap": "projects/suse-sap-cloud/global/images/family/sles-15-sp5-sap",

			"suse-sap-cloud/sles-15-sp5-hardened-sap": "projects/suse-sap-cloud/global/images/family/sles-sap-15-sp5-hardened",

			"opensuse-cloud/opensuse-leap-15": "projects/opensuse-cloud/global/images/family/opensuse-leap",
		}
	}
	return imgsMap
}()

// OldSUSEImages is a map of names to image paths for public SUSE images.
var OldSUSEImages = func() map[string]string {
	imgsMap := make(map[string]string)

	// TODO: enable SUSE tests to use testing pkgs after Artifact Registry supports zypper installation from private repos
	if config.AgentRepo() != "testing" {
		imgsMap = map[string]string{
			"old/sles-15-sp2-sap": "projects/suse-sap-cloud/global/images/sles-15-sp2-sap-v20231214-x86-64",
			"old/sles-15-sp3-sap": "projects/suse-sap-cloud/global/images/sles-15-sp3-sap-v20231214-x86-64",
			"old/sles-15-sp4-sap": "projects/suse-sap-cloud/global/images/sles-15-sp4-sap-v20240208-x86-64",
		}
	}
	return imgsMap
}()

// HeadEL8Images is a map of names to image paths for public EL8 image families. (RHEL, CentOS, Rocky)
var HeadEL8Images = map[string]string{
	"rhel-cloud/rhel-8": "projects/rhel-cloud/global/images/family/rhel-8",

	"rhel-sap-cloud/rhel-8-4-sap": "projects/rhel-sap-cloud/global/images/family/rhel-8-4-sap-ha",
	"rhel-sap-cloud/rhel-8-6-sap": "projects/rhel-sap-cloud/global/images/family/rhel-8-6-sap-ha",
	"rhel-sap-cloud/rhel-8-8-sap": "projects/rhel-sap-cloud/global/images/family/rhel-8-8-sap-ha",

	"rocky-linux-cloud/rocky-linux-8":               "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
	"rocky-linux-cloud/rocky-linux-8-optimized-gcp": "projects/rocky-linux-cloud/global/images/family/rocky-linux-8-optimized-gcp",
}

// OldEL8Images is a map of names to image paths for old EL8 images. (RHEL, CentOS, Rocky)
var OldEL8Images = map[string]string{
	// Currently empty
}

// HeadEL9Images is a map of names to image paths for public EL9 image families. (RHEL, CentOS, Rocky)
var HeadEL9Images = map[string]string{
	"centos-cloud/centos-stream-9": "projects/centos-cloud/global/images/family/centos-stream-9",

	"rhel-cloud/rhel-9": "projects/rhel-cloud/global/images/family/rhel-9",

	"rhel-sap-cloud/rhel-9-0-sap": "projects/rhel-sap-cloud/global/images/family/rhel-9-0-sap-ha",
	"rhel-sap-cloud/rhel-9-2-sap": "projects/rhel-sap-cloud/global/images/family/rhel-9-2-sap-ha",

	"rocky-linux-cloud/rocky-linux-9":               "projects/rocky-linux-cloud/global/images/family/rocky-linux-9",
	"rocky-linux-cloud/rocky-linux-9-optimized-gcp": "projects/rocky-linux-cloud/global/images/family/rocky-linux-9-optimized-gcp",
}

// OldEL9Images is a map of names to image paths for old EL9 images. (RHEL, CentOS, Rocky)
var OldEL9Images = map[string]string{
	// Currently empty
}

// HeadELImages is a map of names to image paths for public EL image families. (RHEL, CentOS, Rocky)
var HeadELImages = func() (newMap map[string]string) {
	newMap = make(map[string]string)
	for k, v := range HeadEL8Images {
		newMap[k] = v
	}
	for k, v := range HeadEL9Images {
		newMap[k] = v
	}
	return
}()

// HeadAptImages is a map of names to image paths for public EL image families. (RHEL, CentOS, Rocky)
var HeadAptImages = func() (newMap map[string]string) {
	newMap = make(map[string]string)
	for k, v := range HeadBusterAptImages {
		newMap[k] = v
	}
	for k, v := range HeadBullseyeAptImages {
		newMap[k] = v
	}
	for k, v := range HeadBookwormAptImages {
		newMap[k] = v
	}
	return
}()

// HeadWindowsImages is a map of names to image paths for public Windows image families.
var HeadWindowsImages = map[string]string{
	"windows-cloud/windows-2016":      "projects/windows-cloud/global/images/family/windows-2016",
	"windows-cloud/windows-2016-core": "projects/windows-cloud/global/images/family/windows-2016-core",
	"windows-cloud/windows-2019":      "projects/windows-cloud/global/images/family/windows-2019",
	"windows-cloud/windows-2019-core": "projects/windows-cloud/global/images/family/windows-2019-core",

	// Testing of win-2022-dc disabled because of https://techcommunity.microsoft.com/t5/windows-server-for-it-pro/faulty-patches-on-server-2022/m-p/4028125

	/*
		"windows-cloud/windows-2022":         "projects/windows-cloud/global/images/family/windows-2022",
		"windows-cloud/windows-2022-core":    "projects/windows-cloud/global/images/family/windows-2022-core",
	*/
}

// OldWindowsImages is a map of names to image paths for old Windows images.
var OldWindowsImages = map[string]string{
	// Currently empty
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
