//  Copyright 2018 Google Inc. All Rights Reserved.
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

// Package agentconfig stores and retrieves configuration settings for the OS Config agent.
package agentconfig

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"golang.org/x/oauth2/jws"
)

const (
	// metadataIP is the documented metadata server IP address.
	metadataIP = "169.254.169.254"
	// metadataHostEnv is the environment variable specifying the
	// GCE metadata hostname.
	metadataHostEnv = "GCE_METADATA_HOST"
	// InstanceMetadata is the compute metadata URL.
	InstanceMetadata = "http://metadata.google.internal/computeMetadata/v1/instance"
	// IdentityTokenPath is the instance identity token path.
	IdentityTokenPath = "instance/service-accounts/default/identity?audience=osconfig.googleapis.com&format=full"
	// ReportURL is the guest attributes endpoint.
	ReportURL = InstanceMetadata + "/guest-attributes"

	googetRepoDir      = "C:/ProgramData/GooGet/repos"
	googetRepoFilePath = googetRepoDir + "/google_osconfig_managed.repo"
	zypperRepoDir      = "/etc/zypp/repos.d"
	zypperRepoFilePath = zypperRepoDir + "/google_osconfig_managed.repo"
	yumRepoDir         = "/etc/yum.repos.d"
	yumRepoFilePath    = yumRepoDir + "/google_osconfig_managed.repo"
	aptRepoDir         = "/etc/apt/sources.list.d"
	aptRepoFilePath    = aptRepoDir + "/google_osconfig_managed.list"

	prodEndpoint = "{zone}-osconfig.googleapis.com:443"

	osInventoryEnabledDefault      = false
	guestPoliciesEnabledDefault    = false
	taskNotificationEnabledDefault = false
	debugEnabledDefault            = false

	configDirWindows     = `C:\Program Files\Google\OSConfig`
	configDirLinux       = "/etc/osconfig"
	taskStateFileWindows = configDirWindows + `\osconfig_task.state`
	taskStateFileLinux   = configDirLinux + "/osconfig_task.state"
	restartFileWindows   = configDirWindows + `\osconfig_agent_restart_required`
	restartFileLinux     = configDirLinux + "/osconfig_agent_restart_required"

	osConfigPollIntervalDefault = 10
	osConfigMetadataPollTimeout = 60
)

var (
	endpoint = flag.String("endpoint", prodEndpoint, "osconfig endpoint override")
	debug    = flag.Bool("debug", false, "set debug log verbosity")
	stdout   = flag.Bool("stdout", false, "log to stdout")

	agentConfig   = &config{}
	agentConfigMx sync.RWMutex
	version       string
	lEtag         = &lastEtag{Etag: "0"}

	// Current supported capabilites for this agent.
	// These are matched server side to what tasks this agent can
	// perform.
	capabilities = []string{"PATCH_GA", "GUEST_POLICY_BETA", "CONFIG_V1"}

	osConfigWatchConfigTimeout = 10 * time.Minute

	defaultClient = &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
		},
	}

	freeOSMemory          = strings.ToLower(os.Getenv("OSCONFIG_FREE_OS_MEMORY"))
	disableInventoryWrite = strings.ToLower(os.Getenv("OSCONFIG_DISABLE_INVENTORY_WRITE"))
)

type config struct {
	aptRepoFilePath         string
	instanceName            string
	instanceZone            string
	projectID               string
	svcEndpoint             string
	googetRepoFilePath      string
	zypperRepoFilePath      string
	yumRepoFilePath         string
	instanceID              string
	numericProjectID        int64
	osConfigPollInterval    int
	debugEnabled            bool
	taskNotificationEnabled bool
	guestPoliciesEnabled    bool
	osInventoryEnabled      bool
}

func (c *config) parseFeatures(features string, enabled bool) {
	for _, f := range strings.Split(features, ",") {
		f = strings.ToLower(strings.TrimSpace(f))
		switch f {
		case "tasks", "ospatch": // ospatch is the legacy flag
			c.taskNotificationEnabled = enabled
		case "guestpolicies", "ospackage": // ospackage is the legacy flag
			c.guestPoliciesEnabled = enabled
		case "osinventory":
			c.osInventoryEnabled = enabled
		}
	}
}

func (c *config) asSha256() string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", c)))

	return fmt.Sprintf("%x", h.Sum(nil))
}

func getAgentConfig() config {
	agentConfigMx.RLock()
	defer agentConfigMx.RUnlock()
	return *agentConfig
}

type lastEtag struct {
	Etag string
	mu   sync.RWMutex
}

func (e *lastEtag) set(etag string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Etag = etag
}

func (e *lastEtag) get() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Etag
}

func parseBool(s string) bool {
	enabled, err := strconv.ParseBool(s)
	if err != nil {
		// Bad entry returns as not enabled.
		return false
	}
	return enabled
}

type metadataJSON struct {
	Instance instanceJSON
	Project  projectJSON
}

type instanceJSON struct {
	Attributes attributesJSON
	ID         *json.Number
	Zone       string
	Name       string
}

type projectJSON struct {
	Attributes       attributesJSON
	ProjectID        string
	NumericProjectID int64
}

type attributesJSON struct {
	PollIntervalOld       *json.Number `json:"os-config-poll-interval"`
	PollInterval          *json.Number `json:"osconfig-poll-interval"`
	InventoryEnabledOld   string       `json:"os-inventory-enabled"`
	InventoryEnabled      string       `json:"enable-os-inventory"`
	PreReleaseFeaturesOld string       `json:"os-config-enabled-prerelease-features"`
	PreReleaseFeatures    string       `json:"osconfig-enabled-prerelease-features"`
	DebugEnabledOld       string       `json:"enable-os-config-debug"`
	LogLevel              string       `json:"osconfig-log-level"`
	OSConfigEndpointOld   string       `json:"os-config-endpoint"`
	OSConfigEndpoint      string       `json:"osconfig-endpoint"`
	OSConfigEnabled       string       `json:"enable-osconfig"`
	DisabledFeatures      string       `json:"osconfig-disabled-features"`
}

func createConfigFromMetadata(md metadataJSON) *config {
	old := getAgentConfig()
	c := &config{
		osInventoryEnabled:      osInventoryEnabledDefault,
		guestPoliciesEnabled:    guestPoliciesEnabledDefault,
		taskNotificationEnabled: taskNotificationEnabledDefault,
		debugEnabled:            debugEnabledDefault,
		svcEndpoint:             prodEndpoint,
		osConfigPollInterval:    osConfigPollIntervalDefault,

		googetRepoFilePath: googetRepoFilePath,
		zypperRepoFilePath: zypperRepoFilePath,
		yumRepoFilePath:    yumRepoFilePath,
		aptRepoFilePath:    aptRepoFilePath,

		projectID:        old.projectID,
		numericProjectID: old.numericProjectID,
		instanceZone:     old.instanceZone,
		instanceName:     old.instanceName,
		instanceID:       old.instanceID,
	}

	if md.Project.ProjectID != "" {
		c.projectID = md.Project.ProjectID
	}
	if md.Project.NumericProjectID != 0 {
		c.numericProjectID = md.Project.NumericProjectID
	}
	if md.Instance.Zone != "" {
		c.instanceZone = md.Instance.Zone
	}
	if md.Instance.Name != "" {
		c.instanceName = md.Instance.Name
	}
	if md.Instance.ID != nil {
		c.instanceID = md.Instance.ID.String()
	}

	// Check project first then instance as instance metadata overrides project.
	switch {
	case md.Project.Attributes.InventoryEnabled != "":
		c.osInventoryEnabled = parseBool(md.Project.Attributes.InventoryEnabled)
	case md.Project.Attributes.InventoryEnabledOld != "":
		c.osInventoryEnabled = parseBool(md.Project.Attributes.InventoryEnabledOld)
	}

	c.parseFeatures(md.Project.Attributes.PreReleaseFeaturesOld, true)
	c.parseFeatures(md.Project.Attributes.PreReleaseFeatures, true)
	if md.Project.Attributes.OSConfigEnabled != "" {
		e := parseBool(md.Project.Attributes.OSConfigEnabled)
		c.taskNotificationEnabled = e
		c.guestPoliciesEnabled = e
		c.osInventoryEnabled = e
	}
	c.parseFeatures(md.Project.Attributes.DisabledFeatures, false)

	switch {
	case md.Instance.Attributes.InventoryEnabled != "":
		c.osInventoryEnabled = parseBool(md.Instance.Attributes.InventoryEnabled)
	case md.Instance.Attributes.InventoryEnabledOld != "":
		c.osInventoryEnabled = parseBool(md.Instance.Attributes.InventoryEnabledOld)
	}

	c.parseFeatures(md.Instance.Attributes.PreReleaseFeaturesOld, true)
	c.parseFeatures(md.Instance.Attributes.PreReleaseFeatures, true)
	if md.Instance.Attributes.OSConfigEnabled != "" {
		e := parseBool(md.Instance.Attributes.OSConfigEnabled)
		c.taskNotificationEnabled = e
		c.guestPoliciesEnabled = e
		c.osInventoryEnabled = e
	}
	c.parseFeatures(md.Instance.Attributes.DisabledFeatures, false)

	switch {
	case md.Project.Attributes.PollInterval != nil:
		if val, err := md.Project.Attributes.PollInterval.Int64(); err == nil {
			c.osConfigPollInterval = int(val)
		}
	case md.Project.Attributes.PollIntervalOld != nil:
		if val, err := md.Project.Attributes.PollIntervalOld.Int64(); err == nil {
			c.osConfigPollInterval = int(val)
		}
	}

	switch {
	case md.Instance.Attributes.PollInterval != nil:
		if val, err := md.Instance.Attributes.PollInterval.Int64(); err == nil {
			c.osConfigPollInterval = int(val)
		}
	case md.Instance.Attributes.PollIntervalOld != nil:
		if val, err := md.Instance.Attributes.PollInterval.Int64(); err == nil {
			c.osConfigPollInterval = int(val)
		}
	}

	switch {
	case md.Project.Attributes.DebugEnabledOld != "":
		c.debugEnabled = parseBool(md.Project.Attributes.DebugEnabledOld)
	case md.Instance.Attributes.DebugEnabledOld != "":
		c.debugEnabled = parseBool(md.Instance.Attributes.DebugEnabledOld)
	}

	switch strings.ToLower(md.Project.Attributes.LogLevel) {
	case "debug":
		c.debugEnabled = true
	case "info":
		c.debugEnabled = false
	}

	switch strings.ToLower(md.Instance.Attributes.LogLevel) {
	case "debug":
		c.debugEnabled = true
	case "info":
		c.debugEnabled = false
	}

	// Flags take precedence over metadata.
	if *debug {
		c.debugEnabled = true
	}

	setSVCEndpoint(md, c)

	return c
}

func setSVCEndpoint(md metadataJSON, c *config) {
	switch {
	case *endpoint != prodEndpoint:
		c.svcEndpoint = *endpoint
	case md.Instance.Attributes.OSConfigEndpoint != "":
		c.svcEndpoint = md.Instance.Attributes.OSConfigEndpoint
	case md.Instance.Attributes.OSConfigEndpointOld != "":
		c.svcEndpoint = md.Instance.Attributes.OSConfigEndpointOld
	case md.Project.Attributes.OSConfigEndpoint != "":
		c.svcEndpoint = md.Project.Attributes.OSConfigEndpoint
	case md.Project.Attributes.OSConfigEndpointOld != "":
		c.svcEndpoint = md.Project.Attributes.OSConfigEndpointOld
	}

	// Example instanceZone: projects/123456/zones/us-west1-b
	parts := strings.Split(c.instanceZone, "/")
	zone := parts[len(parts)-1]
	c.svcEndpoint = strings.ReplaceAll(c.svcEndpoint, "{zone}", zone)
}

func formatMetadataError(err error) error {
	if urlErr, ok := err.(*url.Error); ok {
		if _, ok := urlErr.Err.(*net.DNSError); ok {
			return fmt.Errorf("DNS error when requesting metadata, check DNS settings and ensure metadata.google.internal is setup in your hosts file: %w", err)
		}
		if _, ok := urlErr.Err.(*net.OpError); ok {
			return fmt.Errorf("network error when requesting metadata, make sure your instance has an active network and can reach the metadata server: %w", err)
		}
	}
	return err
}

func getMetadata(suffix string) ([]byte, string, error) {
	host := os.Getenv(metadataHostEnv)
	if host == "" {
		// Using 169.254.169.254 instead of "metadata" here because Go
		// binaries built with the "netgo" tag and without cgo won't
		// know the search suffix for "metadata" is
		// ".google.internal", and this IP address is documented as
		// being stable anyway.
		host = metadataIP
	}
	computeMetadataURL := "http://" + host + "/computeMetadata/v1/" + suffix
	req, err := http.NewRequest("GET", computeMetadataURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := defaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", err
	}
	all, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return all, resp.Header.Get("Etag"), nil
}

// WatchConfig looks for changes in metadata keys. Upon receiving successful response,
// it create a new agent config.
func WatchConfig(ctx context.Context) error {
	var md []byte
	var webError error
	// Max watch time, after this WatchConfig will return.
	timeout := time.After(osConfigWatchConfigTimeout)
	// Min watch loop time.
	loopTicker := time.NewTicker(5 * time.Second)
	defer loopTicker.Stop()
	eTag := lEtag.get()
	webErrorCount := 0
	unmarshalErrorCount := 0
	for {
		md, eTag, webError = getMetadata(fmt.Sprintf("?recursive=true&alt=json&wait_for_change=true&last_etag=%s&timeout_sec=%d", lEtag.get(), osConfigMetadataPollTimeout))
		if webError == nil && eTag != lEtag.get() {
			var metadataConfig metadataJSON
			if err := json.Unmarshal(md, &metadataConfig); err != nil {
				// Try up to three times (with 5s sleep) to get and unmarshal metadata.
				// Most unmarshal errors are transient read issues with the metadata server
				// so we should retry without logging the error.
				if unmarshalErrorCount >= 3 {
					return err
				}
				unmarshalErrorCount++
				select {
				case <-timeout:
					return err
				case <-ctx.Done():
					return nil
				case <-loopTicker.C:
					continue
				}
			}
			unmarshalErrorCount = 0
			lEtag.set(eTag)

			newAgentConfig := createConfigFromMetadata(metadataConfig)

			agentConfigMx.Lock()
			if agentConfig.asSha256() != newAgentConfig.asSha256() {
				agentConfig = newAgentConfig
				agentConfigMx.Unlock()
				break
			}
			agentConfigMx.Unlock()
		}

		// Try up to 12 times (60s) to wait for slow network initialization, after
		// that resort to using defaults and returning the error.
		if webError != nil {
			if webErrorCount == 12 {
				return formatMetadataError(webError)
			}
			webErrorCount++
		}

		select {
		case <-timeout:
			return webError
		case <-ctx.Done():
			return nil
		case <-loopTicker.C:
			continue
		}
	}

	return webError
}

// LogFeatures logs the osconfig feature status.
func LogFeatures(ctx context.Context) {
	clog.Infof(ctx, "OSConfig enabled features status:{GuestPolicies: %t, OSInventory: %t, PatchManagement: %t}.", GuestPoliciesEnabled(), OSInventoryEnabled(), TaskNotificationEnabled())
}

// SvcPollInterval returns the frequency to poll the service.
func SvcPollInterval() time.Duration {
	return time.Duration(getAgentConfig().osConfigPollInterval) * time.Minute
}

// SerialLogPort is the serial port to log to.
func SerialLogPort() string {
	if runtime.GOOS == "windows" {
		return "COM1"
	}
	// Don't write directly to the serial port on Linux as syslog already writes there.
	return ""
}

// Debug sets the debug log verbosity.
func Debug() bool {
	return *debug || getAgentConfig().debugEnabled
}

// Stdout flag.
func Stdout() bool {
	return *stdout
}

// SvcEndpoint is the OS Config service endpoint.
func SvcEndpoint() string {
	return getAgentConfig().svcEndpoint
}

// ZypperRepoDir is the location of the zypper repo files.
func ZypperRepoDir() string {
	return zypperRepoDir
}

// ZypperRepoFilePath is the location where the zypper repo file will be created.
func ZypperRepoFilePath() string {
	return getAgentConfig().zypperRepoFilePath
}

// YumRepoDir is the location of the yum repo files.
func YumRepoDir() string {
	return yumRepoDir
}

// YumRepoFilePath is the location where the yum repo file will be created.
func YumRepoFilePath() string {
	return getAgentConfig().yumRepoFilePath
}

// AptRepoDir is the location of the apt repo files.
func AptRepoDir() string {
	return aptRepoDir
}

// AptRepoFilePath is the location where the apt repo file will be created.
func AptRepoFilePath() string {
	return getAgentConfig().aptRepoFilePath
}

// GooGetRepoDir is the location of the googet repo files.
func GooGetRepoDir() string {
	return googetRepoDir
}

// GooGetRepoFilePath is the location where the googet repo file will be created.
func GooGetRepoFilePath() string {
	return getAgentConfig().googetRepoFilePath
}

// OSInventoryEnabled indicates whether OSInventory should be enabled.
func OSInventoryEnabled() bool {
	return getAgentConfig().osInventoryEnabled
}

// GuestPoliciesEnabled indicates whether GuestPolicies should be enabled.
func GuestPoliciesEnabled() bool {
	return getAgentConfig().guestPoliciesEnabled
}

// TaskNotificationEnabled indicates whether TaskNotification should be enabled.
func TaskNotificationEnabled() bool {
	return getAgentConfig().taskNotificationEnabled
}

// Instance is the URI of the instance the agent is running on.
func Instance() string {
	// Zone contains 'projects/project-id/zones' as a prefix.
	return fmt.Sprintf("%s/instances/%s", Zone(), Name())
}

// NumericProjectID is the numeric project ID of the instance.
func NumericProjectID() int64 {
	return getAgentConfig().numericProjectID
}

// ProjectID is the project ID of the instance.
func ProjectID() string {
	return getAgentConfig().projectID
}

// Zone is the zone the instance is running in.
func Zone() string {
	return getAgentConfig().instanceZone
}

// Name is the instance name.
func Name() string {
	return getAgentConfig().instanceName
}

// ID is the instance id.
func ID() string {
	return getAgentConfig().instanceID
}

type idToken struct {
	exp *time.Time
	raw string
	sync.Mutex
}

func (t *idToken) get() error {
	data, err := metadata.Get(IdentityTokenPath)
	if err != nil {
		return fmt.Errorf("error getting token from metadata: %w", err)
	}

	cs, err := jws.Decode(data)
	if err != nil {
		return err
	}

	t.raw = data
	exp := time.Unix(cs.Exp, 0)
	t.exp = &exp

	return nil
}

var identity idToken

// IDToken is the instance id token.
func IDToken() (string, error) {
	identity.Lock()
	defer identity.Unlock()

	// Rerequest token if expiry is within 10 minutes.
	if identity.exp == nil || time.Now().After(identity.exp.Add(-10*time.Minute)) {
		if err := identity.get(); err != nil {
			return "", err
		}
	}

	return identity.raw, nil
}

// Version is the agent version.
func Version() string {
	return version
}

// SetVersion sets the agent version.
func SetVersion(v string) {
	version = v
}

// Capabilities returns the agents capabilities.
func Capabilities() []string {
	return capabilities
}

// TaskStateFile is the location of the task state file.
func TaskStateFile() string {
	if runtime.GOOS == "windows" {
		return taskStateFileWindows
	}

	return taskStateFileLinux
}

// RestartFile is the location of the restart required file.
func RestartFile() string {
	if runtime.GOOS == "windows" {
		return restartFileWindows
	}

	return restartFileLinux
}

// UserAgent for creating http/grpc clients.
func UserAgent() string {
	return "google-osconfig-agent/" + Version()
}

// DisableInventoryWrite returns true if the DisableInventoryWrite setting is set.
func DisableInventoryWrite() bool {
	return strings.EqualFold(disableInventoryWrite, "true") || disableInventoryWrite == "1"
}

// FreeOSMemory returns true if the FreeOSMemory setting is set.
func FreeOSMemory() bool {
	return strings.EqualFold(freeOSMemory, "true") || freeOSMemory == "1"
}
