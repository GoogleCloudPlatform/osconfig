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

package agentconfig

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

// setupMockMetadataServer starts an httptest.Server and points metadata requests at it.
func setupMockMetadataServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Setenv(metadataHostEnv, strings.TrimPrefix(ts.URL, "http://"))
	t.Cleanup(ts.Close)
	return ts
}

func TestWatchConfig(t *testing.T) {
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"project":{"numericProjectID":12345,"projectId":"projectId","attributes":{"osconfig-endpoint":"bad!!1","enable-os-inventory":"false"}},"instance":{"id":12345,"name":"name","zone":"zone","attributes":{"osconfig-endpoint":"SvcEndpoint","enable-os-inventory":"1","enable-os-config-debug":"true","osconfig-enabled-prerelease-features":"ospackage,ospatch", "osconfig-poll-interval":"3", "enable-scalibr-linux":"true", "trace-get-inventory":"true", "enable-guest-attributes":"true"}}}`)
	})

	if err := WatchConfig(context.Background()); err != nil {
		t.Fatalf("Error running WatchConfig: %v", err)
	}

	testsString := []struct {
		name string
		op   func() string
		want string
	}{
		{name: "metadata endpoint is SvcEndpoint, returns configured service endpoint", op: SvcEndpoint, want: "SvcEndpoint"},
		{name: "metadata zone and name are populated, returns instance resource path", op: Instance, want: "zone/instances/name"},
		{name: "metadata instance id is 12345, returns instance id string", op: ID, want: "12345"},
		{name: "metadata project id is projectId, returns project id", op: ProjectID, want: "projectId"},
		{name: "metadata zone is zone, returns zone", op: Zone, want: "zone"},
		{name: "metadata instance name is name, returns instance name", op: Name, want: "name"},
	}
	for _, tt := range testsString {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, tt.op(), tt.want)
		})
	}

	testsBool := []struct {
		name string
		op   func() bool
		want bool
	}{
		{name: "project disables inventory and instance enables it, returns inventory enabled", op: OSInventoryEnabled, want: true},
		{name: "instance enables prerelease tasks, returns task notifications enabled", op: TaskNotificationEnabled, want: true},
		{name: "instance enables prerelease ospatch, returns guest policies enabled", op: GuestPoliciesEnabled, want: true},
		{name: "project disables debug and instance enables it, returns debug enabled", op: Debug, want: true},
		{name: "instance enables scalibr linux, returns scalibr linux enabled", op: ScalibrLinuxEnabled, want: true},
		{name: "instance enables trace get inventory, returns inventory tracing enabled", op: TraceGetInventory, want: true},
		{name: "instance enables guest attributes, returns guest attributes enabled", op: GuestAttributesEnabled, want: true},
	}
	for _, tt := range testsBool {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, tt.op(), tt.want)
		})
	}

	utiltest.AssertEquals(t, SvcPollInterval().Minutes(), float64(3))
	utiltest.AssertEquals(t, NumericProjectID(), int64(12345))
	utiltest.AssertEquals(t, Instance(), "zone/instances/name")
}

func TestSetConfigEnabled(t *testing.T) {
	var request int
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch request {
		case 0:
			w.Header().Set("Etag", "etag-0")
			fmt.Fprintln(w, `{"project":{"attributes":{"enable-osconfig":"false"}},"instance":{"attributes":{"enable-osconfig":"false"}}}`)
		case 1:
			w.Header().Set("Etag", "etag-1")
			fmt.Fprintln(w, `{"project":{"attributes":{"enable-osconfig":"false"}},"instance":{"attributes":{"enable-osconfig":"true"}}}`)
		case 2:
			w.Header().Set("Etag", "etag-2")
			fmt.Fprintln(w, `{"project":{"attributes":{"enable-osconfig":"false"}},"instance":{"attributes":{"enable-osconfig":"false"}}}`)
		case 3:
			w.Header().Set("Etag", "etag-3")
			fmt.Fprintln(w, `{"project":{"attributes":{"enable-osconfig":"true","osconfig-disabled-features":"osinventory"}}}`)
		}
	})

	for i, want := range []bool{false, true, false} {
		request = i
		if err := WatchConfig(context.Background()); err != nil {
			t.Fatalf("Error running SetConfig: %v", err)
		}

		testsBool := []struct {
			name string
			op   func() bool
		}{
			{name: "enable-osconfig metadata is applied to inventory state, returns expected inventory flag", op: OSInventoryEnabled},
			{name: "enable-osconfig metadata is applied to task state, returns expected task notification flag", op: TaskNotificationEnabled},
			{name: "enable-osconfig metadata is applied to guest policy state, returns expected guest policy flag", op: GuestPoliciesEnabled},
		}
		for _, tt := range testsBool {
			t.Run(fmt.Sprintf("request %d: %s", request, tt.name), func(t *testing.T) {
				utiltest.AssertEquals(t, tt.op(), want)
			})
		}
	}

	request = 3
	if err := WatchConfig(context.Background()); err != nil {
		t.Fatalf("Error running SetConfig: %v", err)
	}

	testsBool := []struct {
		name string
		op   func() bool
		want bool
	}{
		{name: "disabled features contains osinventory, returns inventory disabled", op: OSInventoryEnabled, want: false},
		{name: "osconfig remains enabled for tasks, returns task notifications enabled", op: TaskNotificationEnabled, want: true},
		{name: "osconfig remains enabled for guest policies, returns guest policies enabled", op: GuestPoliciesEnabled, want: true},
	}
	for _, tt := range testsBool {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, tt.op(), tt.want)
		})
	}
}

func TestSetConfigDefaultValues(t *testing.T) {
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", "sample-etag")
		// we always get zone value in instance metadata.
		fmt.Fprintln(w, `{"instance": {"zone": "fake-zone"}}`)
	})

	if err := WatchConfig(context.Background()); err != nil {
		t.Fatalf("Error running SetConfig: %v", err)
	}

	testsString := []struct {
		name string
		op   func() string
		want string
	}{
		{name: "apt repo file path is requested, returns default apt repo file path", op: AptRepoFilePath, want: aptRepoFilePath},
		{name: "yum repo file path is requested, returns default yum repo file path", op: YumRepoFilePath, want: yumRepoFilePath},
		{name: "zypper repo file path is requested, returns default zypper repo file path", op: ZypperRepoFilePath, want: zypperRepoFilePath},
		{name: "googet repo file path is requested, returns default googet repo file path", op: GooGetRepoFilePath, want: googetRepoFilePath},
		{name: "zypper repo dir is requested, returns default zypper repo dir", op: ZypperRepoDir, want: zypperRepoDir},
		{name: "zypper repo format is requested, returns default zypper repo format", op: ZypperRepoFormat, want: filepath.Join(zypperRepoDir, "osconfig_managed_%s.repo")},
		{name: "yum repo dir is requested, returns default yum repo dir", op: YumRepoDir, want: yumRepoDir},
		{name: "yum repo format is requested, returns default yum repo format", op: YumRepoFormat, want: filepath.Join(yumRepoDir, "osconfig_managed_%s.repo")},
		{name: "apt repo dir is requested, returns default apt repo dir", op: AptRepoDir, want: aptRepoDir},
		{name: "apt repo format is requested, returns default apt repo format", op: AptRepoFormat, want: filepath.Join(aptRepoDir, "osconfig_managed_%s.list")},
		{name: "googet repo dir is requested, returns default googet repo dir", op: GooGetRepoDir, want: googetRepoDir},
		{name: "googet repo format is requested, returns default googet repo format", op: GooGetRepoFormat, want: filepath.Join(googetRepoDir, "osconfig_managed_%s.repo")},
		{name: "universe domain is requested, returns default universe domain", op: UniverseDomain, want: universeDomainDefault},
	}
	for _, tt := range testsString {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, tt.op(), tt.want)
		})
	}

	testsBool := []struct {
		op   func() bool
		want bool
	}{
		{op: OSInventoryEnabled, want: osInventoryEnabledDefault},
		{op: TaskNotificationEnabled, want: taskNotificationEnabledDefault},
		{op: GuestPoliciesEnabled, want: guestPoliciesEnabledDefault},
		{op: Debug, want: debugEnabledDefault},
	}
	for _, tt := range testsBool {
		f := filepath.Base(runtime.FuncForPC(reflect.ValueOf(tt.op).Pointer()).Name())
		t.Run(fmt.Sprintf("%s is requested, returns default boolean", f), func(t *testing.T) {
			utiltest.AssertEquals(t, tt.op(), tt.want)
		})
	}

	utiltest.AssertEquals(t, SvcPollInterval().Minutes(), float64(osConfigPollIntervalDefault))

	expectedEndpoint := "fake-zone-osconfig.googleapis.com.:443"
	utiltest.AssertEquals(t, SvcEndpoint(), expectedEndpoint)
}

// TestWatchConfigUnchangedConfigTimeout ignores unchanged metadata until timeout.
func TestWatchConfigUnchangedConfigTimeout(t *testing.T) {
	utiltest.OverrideVariable(t, &watchConfigRetryInterval, 1*time.Millisecond)
	utiltest.OverrideVariable(t, &osConfigWatchConfigTimeout, 10*time.Millisecond)

	var count int
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Etag", fmt.Sprintf("etag-%d", count))
		w.Header().Set("Metadata-Flavor", "Google")
		// Return exactly the same config on every request so asSha256() matches
		fmt.Fprint(w, `{}`)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := WatchConfig(ctx)
	utiltest.AssertErrorMatch(t, err, nil)
	utiltest.AssertErrorMatch(t, ctx.Err(), nil)
}

// TestWatchConfigWebErrorLimit returns a wrapped network error after retry exhaustion.
func TestWatchConfigWebErrorLimit(t *testing.T) {
	lEtag.set("0")
	utiltest.OverrideVariable(t, &watchConfigRetryInterval, 1*time.Millisecond)
	utiltest.OverrideVariable(t, &osConfigWatchConfigTimeout, 1*time.Second)
	t.Setenv(metadataHostEnv, "mock-host")

	mockNetErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	utiltest.OverrideVariable(t, &defaultClient, &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, mockNetErr
	})})

	err := WatchConfig(context.Background())

	expectedBaseErr := &url.Error{
		Op:  "Get",
		URL: "http://mock-host/computeMetadata/v1/?recursive=true&alt=json&wait_for_change=true&last_etag=0&timeout_sec=60",
		Err: mockNetErr,
	}
	expectedErr := fmt.Errorf("network error when requesting metadata, make sure your instance has an active network and can reach the metadata server: %w", expectedBaseErr)
	utiltest.AssertErrorMatch(t, err, expectedErr)
}

// TestWatchConfigUnmarshalErrorLimit returns the unmarshal error after retry exhaustion.
func TestWatchConfigUnmarshalErrorLimit(t *testing.T) {
	utiltest.OverrideVariable(t, &watchConfigRetryInterval, 1*time.Millisecond)
	utiltest.OverrideVariable(t, &osConfigWatchConfigTimeout, 1*time.Second)

	badJSON := []byte(`{"bad json"`)
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", fmt.Sprintf("unmarshal-error-etag-%d", time.Now().UnixNano()))
		w.Header().Set("Metadata-Flavor", "Google")
		w.Write(badJSON)
	})

	err := WatchConfig(context.Background())

	var dummy metadataJSON
	expectedErr := json.Unmarshal(badJSON, &dummy)
	utiltest.AssertErrorMatch(t, err, expectedErr)
}

// TestWatchConfigContextCancel returns nil when the context is canceled.
func TestWatchConfigContextCancel(t *testing.T) {
	utiltest.OverrideVariable(t, &watchConfigRetryInterval, 1*time.Minute)
	utiltest.OverrideVariable(t, &osConfigWatchConfigTimeout, 1*time.Minute)

	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", fmt.Sprintf("cancel-etag-%d", time.Now().UnixNano()))
		w.Header().Set("Metadata-Flavor", "Google")
		fmt.Fprint(w, `{"bad json"`) // Trigger unmarshal error loop which checks context
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately prior to passing it in

	utiltest.AssertErrorMatch(t, WatchConfig(ctx), nil)
}

func TestSetConfigError(t *testing.T) {
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {})
	utiltest.OverrideVariable(t, &osConfigWatchConfigTimeout, 1*time.Millisecond)

	err := WatchConfig(context.Background())
	var dummy metadataJSON
	expectedErr := json.Unmarshal([]byte{}, &dummy)
	utiltest.AssertErrorMatch(t, err, expectedErr)
}

func TestVersion(t *testing.T) {
	utiltest.AssertEquals(t, Version(), "")
	var v = "1"
	SetVersion(v)
	utiltest.AssertEquals(t, Version(), v)
}

// TestLoggingFlags reflects the current logging flag values.
func TestLoggingFlags(t *testing.T) {
	utiltest.OverrideVariable(t, stdout, true)
	utiltest.OverrideVariable(t, disableLocalLogging, true)

	utiltest.AssertEquals(t, Stdout(), true)
	utiltest.AssertEquals(t, DisableLocalLogging(), true)

	utiltest.OverrideVariable(t, stdout, false)
	utiltest.OverrideVariable(t, disableLocalLogging, false)
	utiltest.AssertEquals(t, Stdout(), false)
	utiltest.AssertEquals(t, DisableLocalLogging(), false)
}

// TestLogFeatures logs feature status without panicking.
func TestLogFeatures(t *testing.T) {
	LogFeatures(context.Background())
}

// TestIDToken validates token caching and token parsing errors.
func TestIDToken(t *testing.T) {
	// Create a valid dummy JWS token
	// Header: {"alg":"RS256","typ":"JWT"} -> eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9
	// Payload: {"exp": 4102444800} (January 1, 2100 00:00:00 UTC) -> eyJleHAiOiA0MTAyNDQ0ODAwfQ
	// Signature: dummy -> ZHVtbXk
	validToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOiA0MTAyNDQ0ODAwfQ.ZHVtbXk"

	// Create a token that expires in 5 minutes to test caching fallback.
	// The agent re-requests the token if the expiry is within 10 minutes.
	expTime := time.Now().Add(5 * time.Minute).Unix()
	payload := fmt.Sprintf(`{"exp": %d}`, expTime)
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	expiringToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9." + payloadB64 + ".ZHVtbXk"
	malformedToken := "not.a.valid.token"
	malformedTokenErr := errors.New("jws: invalid token received")

	tests := []struct {
		name         string
		handler      http.HandlerFunc
		numCalls     int
		wantToken    string
		wantErr      error
		wantRequests int
	}{
		{
			name: "token stays valid across two calls, reuses cached token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/computeMetadata/v1/instance/service-accounts/default/identity") {
					w.Header().Set("Metadata-Flavor", "Google")
					fmt.Fprint(w, validToken)
					return
				}
				http.NotFound(w, r)
			},
			numCalls:     2,
			wantToken:    validToken,
			wantErr:      nil,
			wantRequests: 1, // Only 1 request should be made due to caching
		},
		{
			name: "token expires within ten minutes, fetches a fresh token on each call",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/computeMetadata/v1/instance/service-accounts/default/identity") {
					w.Header().Set("Metadata-Flavor", "Google")
					fmt.Fprint(w, expiringToken)
					return
				}
				http.NotFound(w, r)
			},
			numCalls:     2,
			wantToken:    expiringToken,
			wantErr:      nil,
			wantRequests: 2, // Token is within 10m of expiry, should trigger a fetch on every call
		},
		{
			name: "metadata server returns http 500, returns an error after retries",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal error", http.StatusInternalServerError)
			},
			numCalls: 1,
			wantErr:  fmt.Errorf("error getting token from metadata: %w", errors.New("compute: Received 500 `internal error\n`")),
			// The compute/metadata client library automatically retries on 500 errors (1 initial + 5 retries).
			wantRequests: 6,
		},
		{
			name: "metadata server returns malformed token, returns an error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Metadata-Flavor", "Google")
				fmt.Fprint(w, malformedToken)
			},
			numCalls:     1,
			wantErr:      malformedTokenErr,
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				tt.handler(w, r)
			})

			identity = idToken{}

			var token string
			var err error
			for i := 0; i < tt.numCalls; i++ {
				token, err = IDToken()
			}

			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			utiltest.AssertEquals(t, token, tt.wantToken)
			utiltest.AssertEquals(t, requests, tt.wantRequests)
		})
	}
}

// TestFormatMetadataError wraps DNS and network metadata errors.
func TestFormatMetadataError(t *testing.T) {
	errStandard := fmt.Errorf("standard error")
	errDNS := &url.Error{Err: &net.DNSError{Err: "no such host"}}
	errNet := &url.Error{Err: &net.OpError{Op: "dial", Net: "tcp"}}

	tests := []struct {
		name     string
		inputErr error
		wantErr  error
	}{
		{
			name:     "input is a standard error, returns the original error",
			inputErr: errStandard,
			wantErr:  errStandard,
		},
		{
			name:     "input is a dns error, returns a wrapped dns error",
			inputErr: errDNS,
			wantErr:  fmt.Errorf("DNS error when requesting metadata, check DNS settings and ensure metadata.google.internal is setup in your hosts file: %w", errDNS),
		},
		{
			name:     "input is a network error, returns a wrapped network error",
			inputErr: errNet,
			wantErr:  fmt.Errorf("network error when requesting metadata, make sure your instance has an active network and can reach the metadata server: %w", errNet),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertErrorMatch(t, formatMetadataError(tt.inputErr), tt.wantErr)
		})
	}
}

// TestGetMetadata returns metadata bodies and etags for known responses.
func TestGetMetadata(t *testing.T) {
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/computeMetadata/v1/test-success" {
			w.Header().Set("Etag", "test-etag")
			fmt.Fprint(w, "success")
			return
		}
		if r.URL.Path == "/computeMetadata/v1/test-404" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	tests := []struct {
		name     string
		suffix   string
		wantBody string
		wantEtag string
		wantNil  bool
	}{
		{
			name:     "metadata suffix maps to a 200 response, returns body and etag",
			suffix:   "test-success",
			wantBody: "success",
			wantEtag: "test-etag",
		},
		{
			name:    "metadata suffix maps to a 404 response, returns nil body and empty etag",
			suffix:  "test-404",
			wantNil: true,
		},
		{
			name:    "metadata suffix maps to a 500 response, returns nil body and empty etag",
			suffix:  "test-500",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, etag, err := getMetadata(tt.suffix)
			utiltest.AssertErrorMatch(t, err, nil)
			if tt.wantNil {
				utiltest.AssertEquals(t, body, []byte(nil))
				utiltest.AssertEquals(t, etag, "")
			} else {
				utiltest.AssertEquals(t, string(body), tt.wantBody)
				utiltest.AssertEquals(t, etag, tt.wantEtag)
			}
		})
	}
}

// TestGetMetadataFallback uses the metadata IP when the host env var is empty.
func TestGetMetadataFallback(t *testing.T) {
	t.Setenv(metadataHostEnv, "")

	var requestedURL string
	utiltest.OverrideVariable(t, &defaultClient, &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestedURL = req.URL.String()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("mock response"))}, nil
	})})

	_, _, err := getMetadata("test-suffix")
	utiltest.AssertErrorMatch(t, err, nil)

	expected := "http://" + metadataIP + "/computeMetadata/v1/test-suffix"
	utiltest.AssertEquals(t, requestedURL, expected)
}

// TestGetMetadataErrors returns request construction and transport errors.
func TestGetMetadataErrors(t *testing.T) {
	invalidSuffix := "suffix\x7f"
	invalidURLErr := func() error {
		_, err := http.NewRequest("GET", "http://"+metadataIP+"/computeMetadata/v1/"+invalidSuffix, nil)
		return err
	}()
	transportErr := errors.New("mock dial error")

	tests := []struct {
		name          string
		suffix        string
		mockTransport http.RoundTripper
		wantErr       error
	}{
		{
			name:    "metadata suffix contains invalid control character, returns request creation error",
			suffix:  invalidSuffix,
			wantErr: invalidURLErr,
		},
		{
			name:          "metadata client transport returns an error, propagates transport error",
			suffix:        "test-suffix",
			mockTransport: roundTripperFunc(func(req *http.Request) (*http.Response, error) { return nil, transportErr }),
			wantErr:       &url.Error{Op: "Get", URL: "http://" + metadataIP + "/computeMetadata/v1/test-suffix", Err: transportErr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockTransport != nil {
				utiltest.OverrideVariable(t, &defaultClient, &http.Client{Transport: tt.mockTransport})
			}

			_, _, err := getMetadata(tt.suffix)

			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// TestConfigSha256 changes when config content changes.
func TestConfigSha256(t *testing.T) {
	c1 := &config{projectID: "test-project", osInventoryEnabled: true}
	c2 := &config{projectID: "test-project", osInventoryEnabled: true}
	c3 := &config{projectID: "test-project", osInventoryEnabled: false}

	utiltest.AssertEquals(t, c1.asSha256(), c2.asSha256())
	if c1.asSha256() == c3.asSha256() {
		t.Errorf("Expected different configs to have different SHA256")
	}
}

// TestLastEtag supports concurrent reads and writes.
func TestLastEtag(t *testing.T) {
	le := &lastEtag{Etag: "initial"}
	var wg sync.WaitGroup

	// Run concurrent gets and sets to ensure no race conditions
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			le.set(fmt.Sprintf("etag-%d", val))
			_ = le.get()
		}(i)
	}
	wg.Wait()

	if le.get() == "" {
		t.Errorf("Expected non-empty etag")
	}
}

// TestSystemPaths returns OS-specific system paths.
func TestSystemPaths(t *testing.T) {
	utiltest.OverrideVariable(t, &goos, runtime.GOOS)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "system-cache"))

	tests := []struct {
		name string
		op   func() string
		want map[string]string
	}{
		{
			name: "task state file is requested",
			op:   TaskStateFile,
			want: map[string]string{"windows": filepath.Join(GetCacheDirWindows(), "osconfig_task.state"), "linux": taskStateFileLinux},
		},
		{
			name: "old task state file is requested",
			op:   OldTaskStateFile,
			want: map[string]string{"windows": oldTaskStateFileWindows, "linux": oldTaskStateFileLinux},
		},
		{
			name: "restart file is requested",
			op:   RestartFile,
			want: map[string]string{"windows": filepath.Join(GetCacheDirWindows(), "osconfig_agent_restart_required"), "linux": restartFileLinux},
		},
		{
			name: "old restart file is requested",
			op:   OldRestartFile,
			want: map[string]string{"windows": oldRestartFileLinux, "linux": oldRestartFileLinux},
		},
		{
			name: "cache directory is requested",
			op:   CacheDir,
			want: map[string]string{"windows": GetCacheDirWindows(), "linux": cacheDirLinux},
		},
		{
			name: "serial log port is requested",
			op:   SerialLogPort,
			want: map[string]string{"windows": "COM1", "linux": ""},
		},
	}

	for _, tt := range tests {
		for _, testOS := range []string{"windows", "linux"} {
			t.Run(fmt.Sprintf("%s, returns %s path", tt.name, testOS), func(t *testing.T) {
				utiltest.OverrideVariable(t, &goos, testOS)
				utiltest.AssertEquals(t, tt.op(), tt.want[testOS])
			})
		}
	}
}

// TestMiscGetters returns static getter values.
func TestMiscGetters(t *testing.T) {
	SetVersion("1.2.3")

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{name: "agent capabilities are requested, returns supported capability list", got: Capabilities(), want: []string{"PATCH_GA", "GUEST_POLICY_BETA", "CONFIG_V1"}},
		{name: "user agent is requested after version is set, returns versioned user agent", got: UserAgent(), want: "google-osconfig-agent/1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.AssertEquals(t, tt.got, tt.want)
		})
	}
}

// TestCreateConfigFromMetadata applies metadata precedence to config values.
func TestCreateConfigFromMetadata(t *testing.T) {
	// Reset the global agent config to avoid test cross-contamination
	agentConfigMx.Lock()
	agentConfig = &config{}
	agentConfigMx.Unlock()

	pollInt15 := json.Number("15")
	pollInt20 := json.Number("20")
	id98765 := json.Number("98765")

	tests := []struct {
		name         string
		md           metadataJSON
		setDebugFlag bool
		want         *config
	}{
		{
			name: "metadata is empty, returns config defaults",
			md:   metadataJSON{},
			want: &config{
				osInventoryEnabled:      osInventoryEnabledDefault,
				guestPoliciesEnabled:    guestPoliciesEnabledDefault,
				taskNotificationEnabled: taskNotificationEnabledDefault,
				debugEnabled:            debugEnabledDefault,
				svcEndpoint:             strings.ReplaceAll(prodEndpoint, "{zone}", ""),
				osConfigPollInterval:    osConfigPollIntervalDefault,
				googetRepoFilePath:      googetRepoFilePath,
				zypperRepoFilePath:      zypperRepoFilePath,
				yumRepoFilePath:         yumRepoFilePath,
				aptRepoFilePath:         aptRepoFilePath,
				universeDomain:          universeDomainDefault,
			},
		},
		{
			name: "project metadata sets debug and poll interval, returns project derived config",
			md: metadataJSON{
				Project: projectJSON{
					ProjectID: "proj-1",
					Attributes: attributesJSON{
						LogLevel:        "debug",
						PollInterval:    &pollInt15,
						OSConfigEnabled: "true",
					},
				},
			},
			want: &config{
				projectID:               "proj-1",
				osInventoryEnabled:      true,
				guestPoliciesEnabled:    true,
				taskNotificationEnabled: true,
				debugEnabled:            true,
				svcEndpoint:             strings.ReplaceAll(prodEndpoint, "{zone}", ""),
				osConfigPollInterval:    15,
				googetRepoFilePath:      googetRepoFilePath,
				zypperRepoFilePath:      zypperRepoFilePath,
				yumRepoFilePath:         yumRepoFilePath,
				aptRepoFilePath:         aptRepoFilePath,
				universeDomain:          universeDomainDefault,
			},
		},
		{
			name: "instance metadata conflicts with project metadata, returns instance overrides",
			md: metadataJSON{
				Project: projectJSON{
					ProjectID: "proj-1",
					Attributes: attributesJSON{
						LogLevel:        "info",
						PollInterval:    &pollInt15,
						OSConfigEnabled: "true",
					},
				},
				Instance: instanceJSON{
					Attributes: attributesJSON{
						LogLevel:        "debug",
						PollInterval:    &pollInt20,
						OSConfigEnabled: "false",
					},
				},
			},
			want: &config{
				projectID:               "proj-1",
				osInventoryEnabled:      false,
				guestPoliciesEnabled:    false,
				taskNotificationEnabled: false,
				debugEnabled:            true,
				svcEndpoint:             strings.ReplaceAll(prodEndpoint, "{zone}", ""),
				osConfigPollInterval:    20,
				googetRepoFilePath:      googetRepoFilePath,
				zypperRepoFilePath:      zypperRepoFilePath,
				yumRepoFilePath:         yumRepoFilePath,
				aptRepoFilePath:         aptRepoFilePath,
				universeDomain:          universeDomainDefault,
			},
		},
		{
			name: "legacy poll interval and disabled features are set, returns config with legacy values applied",
			md: metadataJSON{
				Project: projectJSON{
					Attributes: attributesJSON{
						PollIntervalOld: &pollInt15,
					},
				},
				Instance: instanceJSON{
					ID: &id98765,
					Attributes: attributesJSON{
						OSConfigEnabled:  "true",
						DisabledFeatures: "osinventory, guestpolicies",
					},
				},
			},
			want: &config{
				instanceID:              "98765",
				osInventoryEnabled:      false,
				guestPoliciesEnabled:    false,
				taskNotificationEnabled: true,
				debugEnabled:            debugEnabledDefault,
				svcEndpoint:             strings.ReplaceAll(prodEndpoint, "{zone}", ""),
				osConfigPollInterval:    15,
				googetRepoFilePath:      googetRepoFilePath,
				zypperRepoFilePath:      zypperRepoFilePath,
				yumRepoFilePath:         yumRepoFilePath,
				aptRepoFilePath:         aptRepoFilePath,
				universeDomain:          universeDomainDefault,
			},
		},
		{
			name: "debug flag is enabled with non-debug metadata, returns config with debug enabled",
			md: metadataJSON{
				Project: projectJSON{
					Attributes: attributesJSON{
						LogLevel: "info",
					},
				},
			},
			setDebugFlag: true,
			want: &config{
				osInventoryEnabled:      osInventoryEnabledDefault,
				guestPoliciesEnabled:    guestPoliciesEnabledDefault,
				taskNotificationEnabled: taskNotificationEnabledDefault,
				debugEnabled:            true,
				svcEndpoint:             strings.ReplaceAll(prodEndpoint, "{zone}", ""),
				osConfigPollInterval:    osConfigPollIntervalDefault,
				googetRepoFilePath:      googetRepoFilePath,
				zypperRepoFilePath:      zypperRepoFilePath,
				yumRepoFilePath:         yumRepoFilePath,
				aptRepoFilePath:         aptRepoFilePath,
				universeDomain:          universeDomainDefault,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.OverrideVariable(t, debug, tt.setDebugFlag)

			got := createConfigFromMetadata(tt.md)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createConfigFromMetadata() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSvcEndpoint(t *testing.T) {
	var request int
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch request {
		case 0:
			w.Header().Set("Etag", "etag-0")
			// we always get zone value in instance metadata.
			fmt.Fprintln(w, `{"instance": {"id": 12345,"name": "name","zone": "fakezone","attributes": {"osconfig-endpoint": "{zone}-dev.osconfig.googleapis.com"}}}`)
		case 1:
			w.Header().Set("Etag", "etag-1")
			fmt.Fprintln(w, `{"universe": {"universeDomain": "domain.com"}, "instance": {"id": 12345,"name": "name","zone": "fakezone","attributes": {"osconfig-endpoint": "{zone}-dev.osconfig.googleapis.com"}}}`)
		}
	})

	for i, expectedSvcEndpoint := range []string{"fakezone-dev.osconfig.googleapis.com", "fakezone-dev.osconfig.domain.com"} {
		request = i
		if err := WatchConfig(context.Background()); err != nil {
			t.Fatalf("Error running SetConfig: %v", err)
		}

		utiltest.AssertEquals(t, SvcEndpoint(), expectedSvcEndpoint)
	}

}

func TestDisableCloudLogging(t *testing.T) {
	var request int
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch request {
		case 0:
			w.Header().Set("Etag", "etag-0")
			fmt.Fprintln(w, `{"universe":{"universeDomain": "domain.com"}}`)
		case 1:
			w.Header().Set("Etag", "etag-1")
			fmt.Fprintln(w, `{"instance": {"zone": "fake-zone"}}`)
		}
	})

	for i, expectedDisableCloudLoggingValue := range []bool{true, false} {
		request = i
		if err := WatchConfig(context.Background()); err != nil {
			t.Fatalf("Error running SetConfig: %v", err)
		}

		utiltest.AssertEquals(t, DisableCloudLogging(), expectedDisableCloudLoggingValue)
	}

}

// TestSetScalibrEnablement applies metadata precedence for scalibr enablement.
func TestSetScalibrEnablement(t *testing.T) {
	tests := []struct {
		name    string
		projVal string
		instVal string
		want    bool
	}{
		{name: "project and instance values are empty, returns scalibr disabled", projVal: "", instVal: "", want: false},
		{name: "project enables scalibr and instance is empty, returns scalibr enabled", projVal: "true", instVal: "", want: true},
		{name: "project disables scalibr and instance is empty, returns scalibr disabled", projVal: "false", instVal: "", want: false},
		{name: "instance enables scalibr and project is empty, returns scalibr enabled", projVal: "", instVal: "true", want: true},
		{name: "instance enables scalibr and project disables it, returns instance override", projVal: "false", instVal: "true", want: true},
		{name: "instance disables scalibr and project enables it, returns instance override", projVal: "true", instVal: "false", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &config{}
			md := metadataJSON{
				Project:  projectJSON{Attributes: attributesJSON{ScalibrLinuxEnabled: tt.projVal}},
				Instance: instanceJSON{Attributes: attributesJSON{ScalibrLinuxEnabled: tt.instVal}},
			}
			setScalibrEnablement(md, c)

			utiltest.AssertEquals(t, c.scalibrLinuxEnabled, tt.want)
		})
	}
}

// TestSetTraceGetInventory applies metadata precedence for inventory tracing.
func TestSetTraceGetInventory(t *testing.T) {
	tests := []struct {
		name    string
		projVal string
		instVal string
		want    bool
	}{
		{name: "project and instance values are empty, returns trace get inventory disabled", projVal: "", instVal: "", want: false},
		{name: "project enables trace get inventory and instance is empty, returns tracing enabled", projVal: "true", instVal: "", want: true},
		{name: "project disables trace get inventory and instance is empty, returns tracing disabled", projVal: "false", instVal: "", want: false},
		{name: "instance enables trace get inventory and project is empty, returns tracing enabled", projVal: "", instVal: "true", want: true},
		{name: "instance enables trace get inventory and project disables it, returns instance override", projVal: "false", instVal: "true", want: true},
		{name: "instance disables trace get inventory and project enables it, returns instance override", projVal: "true", instVal: "false", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &config{}
			md := metadataJSON{
				Project:  projectJSON{Attributes: attributesJSON{TraceGetInventory: tt.projVal}},
				Instance: instanceJSON{Attributes: attributesJSON{TraceGetInventory: tt.instVal}},
			}
			setTraceGetInventory(md, c)

			utiltest.AssertEquals(t, c.traceGetInventory, tt.want)
		})
	}
}

// TestSetSVCEndpoint applies endpoint precedence and placeholder replacement.
func TestSetSVCEndpoint(t *testing.T) {
	utiltest.OverrideVariable(t, endpoint, *endpoint)

	tests := []struct {
		name         string
		flag         string
		instNew      string
		instOld      string
		projNew      string
		projOld      string
		universe     string
		instanceZone string
		want         string
	}{
		{
			name:         "flag and metadata endpoints are empty, returns default zonal endpoint",
			flag:         prodEndpoint,
			instanceZone: "projects/123/zones/us-west1-a",
			universe:     "googleapis.com",
			want:         "us-west1-a-osconfig.googleapis.com.:443",
		},
		{
			name:     "endpoint flag is set, returns flag endpoint",
			flag:     "custom-endpoint",
			instNew:  "inst-new",
			universe: "googleapis.com",
			want:     "custom-endpoint",
		},
		{
			name:         "instance new endpoint is set, returns zonal instance endpoint",
			flag:         prodEndpoint,
			instNew:      "inst-new-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			universe:     "googleapis.com",
			want:         "inst-new-us-west1-a",
		},
		{
			name:         "instance legacy endpoint is set, returns zonal legacy instance endpoint",
			flag:         prodEndpoint,
			instOld:      "inst-old-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			universe:     "googleapis.com",
			want:         "inst-old-us-west1-a",
		},
		{
			name:         "project new endpoint is set, returns zonal project endpoint",
			flag:         prodEndpoint,
			projNew:      "proj-new-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			universe:     "googleapis.com",
			want:         "proj-new-us-west1-a",
		},
		{
			name:         "project legacy endpoint is set, returns zonal legacy project endpoint",
			flag:         prodEndpoint,
			projOld:      "proj-old-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			universe:     "googleapis.com",
			want:         "proj-old-us-west1-a",
		},
		{
			name:     "endpoint uses default domain and universe domain is custom, returns rewritten universe endpoint",
			flag:     prodEndpoint,
			instNew:  "test-osconfig.googleapis.com",
			universe: "my-universe.com",
			want:     "test-osconfig.my-universe.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.OverrideVariable(t, endpoint, tt.flag)
			c := &config{
				instanceZone:   tt.instanceZone,
				svcEndpoint:    prodEndpoint,
				universeDomain: tt.universe,
			}

			md := metadataJSON{
				Project: projectJSON{
					Attributes: attributesJSON{
						OSConfigEndpoint:    tt.projNew,
						OSConfigEndpointOld: tt.projOld,
					},
				},
				Instance: instanceJSON{
					Attributes: attributesJSON{
						OSConfigEndpoint:    tt.instNew,
						OSConfigEndpointOld: tt.instOld,
					},
				},
			}

			setSVCEndpoint(md, c)

			utiltest.AssertEquals(t, c.svcEndpoint, tt.want)
		})
	}
}

// TestGetCacheDirWindows prefers the user cache dir and falls back to TempDir.
func TestGetCacheDirWindows(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T)
		want  func(t *testing.T) string
	}{
		{
			name: "xdg cache home is set, returns cache path under xdg cache home",
			setup: func(t *testing.T) {
				t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "xdg-cache"))
			},
			want: func(t *testing.T) string {
				return filepath.Join(os.Getenv("XDG_CACHE_HOME"), windowsCacheDir)
			},
		},
		{
			name: "windows user cache directory is unavailable, returns tempdir fallback path",
			setup: func(t *testing.T) {
				envs := []string{"HOME", "LocalAppData", "XDG_CACHE_HOME"}
				for _, env := range envs {
					t.Setenv(env, "")
				}
			},
			want: func(t *testing.T) string {
				return filepath.Join(os.TempDir(), windowsCacheDir)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			utiltest.AssertEquals(t, GetCacheDirWindows(), tt.want(t))
		})
	}
}

// TestFlagsAndEnvVars parses environment-backed flags.
func TestFlagsAndEnvVars(t *testing.T) {
	tests := []struct {
		name                  string
		freeOSMemoryVal       string
		disableInventoryWrite string
		wantFreeOS            bool
		wantDisableInv        bool
	}{
		{name: "environment enables both flags, returns both flags enabled", freeOSMemoryVal: "true", disableInventoryWrite: "1", wantFreeOS: true, wantDisableInv: true},
		{name: "environment disables both flags, returns both flags disabled", freeOSMemoryVal: "false", disableInventoryWrite: "0", wantFreeOS: false, wantDisableInv: false},
		{name: "environment leaves both flags empty, returns both flags disabled", freeOSMemoryVal: "", disableInventoryWrite: "", wantFreeOS: false, wantDisableInv: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utiltest.OverrideVariable(t, &freeOSMemory, tt.freeOSMemoryVal)
			utiltest.OverrideVariable(t, &disableInventoryWrite, tt.disableInventoryWrite)

			utiltest.AssertEquals(t, FreeOSMemory(), tt.wantFreeOS)
			utiltest.AssertEquals(t, DisableInventoryWrite(), tt.wantDisableInv)
		})
	}
}

// TestParseBool parses supported boolean string forms.
func TestParseBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "true", want: true},
		{input: "1", want: true},
		{input: "false", want: false},
		{input: "0", want: false},
		{input: "invalid", want: false},
	}

	for _, tt := range tests {
		if got := parseBool(tt.input); got != tt.want {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseFeatures applies comma-separated feature flags.
func TestParseFeatures(t *testing.T) {
	tests := []struct {
		name     string
		initial  config
		features string
		enabled  bool
		want     config
	}{
		{
			name:     "feature list enables tasks ospackage and osinventory, returns enabled feature state",
			initial:  config{},
			features: "tasks, ospackage, osinventory, unknown",
			enabled:  true,
			want: config{
				taskNotificationEnabled: true,
				guestPoliciesEnabled:    true,
				osInventoryEnabled:      true,
			},
		},
		{
			name: "feature list disables ospatch and guestpolicies, returns disabled task and guest policy state",
			initial: config{
				taskNotificationEnabled: true,
				guestPoliciesEnabled:    true,
				osInventoryEnabled:      true,
			},
			features: "ospatch, guestpolicies",
			enabled:  false,
			want: config{
				taskNotificationEnabled: false,
				guestPoliciesEnabled:    false,
				osInventoryEnabled:      true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.initial
			c.parseFeatures(tt.features, tt.enabled)

			if !reflect.DeepEqual(c, tt.want) {
				t.Errorf("parseFeatures() state = %+v, want %+v", c, tt.want)
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
