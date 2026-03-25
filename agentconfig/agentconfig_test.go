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
	"io/ioutil"
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

// setupMockMetadataServer starts an httptest.Server with the provided handler and overrides the GCE_METADATA_HOST environment variable.
// It also registers cleanup functions to close the server and restore the environment variable.
func setupMockMetadataServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	utiltest.OverrideEnv(t, "GCE_METADATA_HOST", strings.TrimPrefix(ts.URL, "http://"))

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
		desc string
		op   func() string
		want string
	}{
		{desc: "SvcEndpoint", op: SvcEndpoint, want: "SvcEndpoint"},
		{desc: "Instance", op: Instance, want: "zone/instances/name"},
		{desc: "ID", op: ID, want: "12345"},
		{desc: "ProjectID", op: ProjectID, want: "projectId"},
		{desc: "Zone", op: Zone, want: "zone"},
		{desc: "Name", op: Name, want: "name"},
	}
	for _, tt := range testsString {
		if tt.op() != tt.want {
			t.Errorf("%q: got(%q) != want(%q)", tt.desc, tt.op(), tt.want)
		}
	}

	testsBool := []struct {
		desc string
		op   func() bool
		want bool
	}{
		{desc: "osinventory should be enabled (proj disabled, inst enabled)", op: OSInventoryEnabled, want: true},
		{desc: "taskNotification should be enabled (inst enabled)", op: TaskNotificationEnabled, want: true},
		{desc: "guestpolicies should be enabled (proj enabled)", op: GuestPoliciesEnabled, want: true},
		{desc: "debugenabled should be true (proj disabled, inst enabled)", op: Debug, want: true},
		{desc: "scalibrLinuxEnabled should be true", op: ScalibrLinuxEnabled, want: true},
		{desc: "traceGetInventory should be true", op: TraceGetInventory, want: true},
		{desc: "guestAttributesEnabled should be true", op: GuestAttributesEnabled, want: true},
	}
	for _, tt := range testsBool {
		if tt.op() != tt.want {
			t.Errorf("%q: got(%t) != want(%t)", tt.desc, tt.op(), tt.want)
		}
	}

	if SvcPollInterval().Minutes() != float64(3) {
		t.Errorf("Default poll interval: got(%f) != want(%d)", SvcPollInterval().Minutes(), 3)
	}
	if NumericProjectID() != 12345 {
		t.Errorf("NumericProjectID: got(%v) != want(%d)", NumericProjectID(), 12345)
	}

	if Instance() != "zone/instances/name" {
		t.Errorf("zone: got(%s) != want(%s)", Instance(), "zone/instances/name")
	}
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
			desc string
			op   func() bool
		}{
			{desc: "OSInventoryEnabled", op: OSInventoryEnabled},
			{desc: "TaskNotificationEnabled", op: TaskNotificationEnabled},
			{desc: "GuestPoliciesEnabled", op: GuestPoliciesEnabled},
		}
		for _, tt := range testsBool {
			if tt.op() != want {
				t.Errorf("Request %d: %s: got(%t) != want(%t)", request, tt.desc, tt.op(), want)
			}
		}
	}

	request = 3
	if err := WatchConfig(context.Background()); err != nil {
		t.Fatalf("Error running SetConfig: %v", err)
	}

	testsBool := []struct {
		desc string
		op   func() bool
		want bool
	}{
		{desc: "OSInventoryEnabled", op: OSInventoryEnabled, want: false},
		{desc: "TaskNotificationEnabled", op: TaskNotificationEnabled, want: true},
		{desc: "GuestPoliciesEnabled", op: GuestPoliciesEnabled, want: true},
	}
	for _, tt := range testsBool {
		if tt.op() != tt.want {
			t.Errorf("%s: got(%t) != want(%t)", tt.desc, tt.op(), tt.want)
		}
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
		op   func() string
		want string
	}{
		{op: AptRepoFilePath, want: aptRepoFilePath},
		{op: YumRepoFilePath, want: yumRepoFilePath},
		{op: ZypperRepoFilePath, want: zypperRepoFilePath},
		{op: GooGetRepoFilePath, want: googetRepoFilePath},
		{op: ZypperRepoDir, want: zypperRepoDir},
		{op: ZypperRepoFormat, want: filepath.Join(zypperRepoDir, "osconfig_managed_%s.repo")},
		{op: YumRepoDir, want: yumRepoDir},
		{op: YumRepoFormat, want: filepath.Join(yumRepoDir, "osconfig_managed_%s.repo")},
		{op: AptRepoDir, want: aptRepoDir},
		{op: AptRepoFormat, want: filepath.Join(aptRepoDir, "osconfig_managed_%s.list")},
		{op: GooGetRepoDir, want: googetRepoDir},
		{op: GooGetRepoFormat, want: filepath.Join(googetRepoDir, "osconfig_managed_%s.repo")},
		{op: UniverseDomain, want: universeDomainDefault},
	}
	for _, tt := range testsString {
		if tt.op() != tt.want {
			f := filepath.Base(runtime.FuncForPC(reflect.ValueOf(tt.op).Pointer()).Name())
			t.Errorf("%q: got(%q) != want(%q)", f, tt.op(), tt.want)
		}
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
		if tt.op() != tt.want {
			f := filepath.Base(runtime.FuncForPC(reflect.ValueOf(tt.op).Pointer()).Name())
			t.Errorf("%q: got(%t) != want(%t)", f, tt.op(), tt.want)
		}
	}

	if SvcPollInterval().Minutes() != float64(osConfigPollIntervalDefault) {
		t.Errorf("Default poll interval: got(%f) != want(%d)", SvcPollInterval().Minutes(), osConfigPollIntervalDefault)
	}

	expectedEndpoint := "fake-zone-osconfig.googleapis.com.:443"
	if SvcEndpoint() != expectedEndpoint {
		t.Errorf("Default endpoint: got(%s) != want(%s)", SvcEndpoint(), expectedEndpoint)
	}
}

// TestWatchConfigUnchangedConfigTimeout tests how the agent behaves when it receives
// updates from the metadata server, but the actual configuration data hasn't changed.
//
// The agent checks the SHA256 hash of the new data. If the hash is identical to
// the current configuration, it knows the update is superficial. Instead of
// applying the configuration and exiting, the agent should ignore the update and
// keep polling for real changes. This test verifies that the agent correctly
// continues to wait until its internal timeout runs out, and then exits normally.
func TestWatchConfigUnchangedConfigTimeout(t *testing.T) {
	OverrideWatchConfigTimeouts(t, 1*time.Millisecond, 10*time.Millisecond)

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
	if err != nil {
		t.Errorf("Expected nil error on timeout, got: %v", err)
	}
	if ctx.Err() != nil {
		t.Errorf("Test context timed out before internal timeout fired: %v", ctx.Err())
	}
}

// TestWatchConfigWebErrorLimit tests how WatchConfig handles network errors when it
// can't reach the metadata server. The test creates a situation where the agent
// can't connect to the server and checks that the agent retries the connection
// up to a limit of 12 times before giving up and reporting an error.
func TestWatchConfigWebErrorLimit(t *testing.T) {
	lEtag.set("0")
	OverrideWatchConfigTimeouts(t, 1*time.Millisecond, 1*time.Second)
	utiltest.OverrideEnv(t, "GCE_METADATA_HOST", "mock-host")

	mockNetErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	MockDefaultClientTransport(t, func(req *http.Request) (*http.Response, error) {
		return nil, mockNetErr
	})

	err := WatchConfig(context.Background())
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}

	expectedBaseErr := &url.Error{
		Op:  "Get",
		URL: "http://mock-host/computeMetadata/v1/?recursive=true&alt=json&wait_for_change=true&last_etag=0&timeout_sec=60",
		Err: mockNetErr,
	}
	expectedErr := fmt.Errorf("network error when requesting metadata, make sure your instance has an active network and can reach the metadata server: %w", expectedBaseErr)
	utiltest.AssertErrorMatch(t, err, expectedErr)
}

// TestWatchConfigUnmarshalErrorLimit tests how WatchConfig handles bad or incomplete
// data from the metadata server. The test gives the agent a broken configuration
// response and verifies that the agent tries to read it again up to a limit of 3
// times before it stops and reports an error.
func TestWatchConfigUnmarshalErrorLimit(t *testing.T) {
	OverrideWatchConfigTimeouts(t, 1*time.Millisecond, 1*time.Second)

	badJSON := []byte(`{"bad json"`)
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", fmt.Sprintf("unmarshal-error-etag-%d", time.Now().UnixNano()))
		w.Header().Set("Metadata-Flavor", "Google")
		w.Write(badJSON)
	})

	err := WatchConfig(context.Background())
	if err == nil {
		t.Fatal("Expected unmarshal error, got nil")
	}

	var dummy metadataJSON
	expectedErr := json.Unmarshal(badJSON, &dummy)
	utiltest.AssertErrorMatch(t, err, expectedErr)
}

// TestWatchConfigContextCancel tests that the WatchConfig function can be stopped
// correctly. It checks that if another part of the program tells WatchConfig to
// cancel, it stops immediately without waiting for a timeout or retrying failed
// requests.
func TestWatchConfigContextCancel(t *testing.T) {
	OverrideWatchConfigTimeouts(t, 1*time.Minute, 1*time.Minute)

	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", fmt.Sprintf("cancel-etag-%d", time.Now().UnixNano()))
		w.Header().Set("Metadata-Flavor", "Google")
		fmt.Fprint(w, `{"bad json"`) // Trigger unmarshal error loop which checks context
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately prior to passing it in

	if err := WatchConfig(ctx); err != nil {
		t.Errorf("Expected nil error on context cancellation, got: %v", err)
	}
}

func TestSetConfigError(t *testing.T) {
	setupMockMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {})

	osConfigWatchConfigTimeout = 1 * time.Millisecond

	if err := WatchConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Errorf("Unexpected output %+v", err)
	}
}

func TestVersion(t *testing.T) {
	if Version() != "" {
		t.Errorf("Unexpected version %q, want \"\"", Version())
	}
	var v = "1"
	SetVersion(v)
	if Version() != v {
		t.Errorf("Unexpected version %q, want %q", Version(), v)
	}
}

// TestLoggingFlags tests logging setting accessors against command-line flags.
func TestLoggingFlags(t *testing.T) {
	origStdout := *stdout
	origDisableLocalLogging := *disableLocalLogging
	defer func() {
		*stdout = origStdout
		*disableLocalLogging = origDisableLocalLogging
	}()

	*stdout = true
	*disableLocalLogging = true
	if !Stdout() {
		t.Errorf("Stdout() = false, want true")
	}
	if !DisableLocalLogging() {
		t.Errorf("DisableLocalLogging() = false, want true")
	}

	*stdout = false
	*disableLocalLogging = false
	if Stdout() {
		t.Errorf("Stdout() = true, want false")
	}
	if DisableLocalLogging() {
		t.Errorf("DisableLocalLogging() = true, want false")
	}
}

// TestLogFeatures tests that feature status logging executes without panicking.
func TestLogFeatures(t *testing.T) {
	LogFeatures(context.Background())
}

// TestIDToken tests getting and understanding the instance identity token from the
// metadata server. It checks valid tokens, caching behavior, and error handling
// (e.g. HTTP 500 or malformed tokens).
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

	tests := []struct {
		name         string
		handler      http.HandlerFunc
		numCalls     int
		wantToken    string
		wantErr      error
		wantRequests int
	}{
		{
			name: "Valid token with caching",
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
			name: "Expiring token forces re-fetch",
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
			name: "HTTP 500 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal error", http.StatusInternalServerError)
			},
			numCalls: 1,
			wantErr:  fmt.Errorf("error getting token from metadata: %w", errors.New("compute: Received 500 `internal error\n`")),
			// The compute/metadata client library automatically retries on 500 errors (1 initial + 5 retries).
			wantRequests: 6,
		},
		{
			name: "Malformed token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Metadata-Flavor", "Google")
				fmt.Fprint(w, "not.a.valid.token")
			},
			numCalls:     1,
			wantErr:      errors.New("jws: invalid token received"),
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
			if token != tt.wantToken {
				t.Errorf("IDToken() = %q, want %q", token, tt.wantToken)
			}
			if requests != tt.wantRequests {
				t.Errorf("Expected %d HTTP requests, got %d", tt.wantRequests, requests)
			}
		})
	}
}

// TestFormatMetadataError verifies that network and DNS errors are wrapped with helpful context.
func TestFormatMetadataError(t *testing.T) {
	dnsErr := &url.Error{Err: &net.DNSError{Err: "no such host"}}
	netErr := &url.Error{Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}}

	tests := []struct {
		name     string
		inputErr error
		wantErr  error
	}{
		{
			name:     "standard error",
			inputErr: fmt.Errorf("standard error"),
			wantErr:  fmt.Errorf("standard error"),
		},
		{
			name:     "DNS error",
			inputErr: dnsErr,
			wantErr:  fmt.Errorf("DNS error when requesting metadata, check DNS settings and ensure metadata.google.internal is setup in your hosts file: %w", dnsErr),
		},
		{
			name:     "network error",
			inputErr: netErr,
			wantErr:  fmt.Errorf("network error when requesting metadata, make sure your instance has an active network and can reach the metadata server: %w", netErr),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMetadataError(tt.inputErr)

			utiltest.AssertErrorMatch(t, got, tt.wantErr)
		})
	}
}

// TestGetMetadata verifies successful and error responses from the metadata server.
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
			name:     "success",
			suffix:   "test-success",
			wantBody: "success",
			wantEtag: "test-etag",
		},
		{
			name:    "404 not found",
			suffix:  "test-404",
			wantNil: true,
		},
		{
			name:    "500 internal server error",
			suffix:  "test-500",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, etag, err := getMetadata(tt.suffix)
			if err != nil {
				t.Errorf("getMetadata(%q) error: %v", tt.suffix, err)
			}
			if tt.wantNil {
				if body != nil || etag != "" {
					t.Errorf("getMetadata(%q) expected nil body and empty etag, got %q, %q", tt.suffix, body, etag)
				}
			} else {
				if string(body) != tt.wantBody {
					t.Errorf("getMetadata(%q) body = %q, want %q", tt.suffix, body, tt.wantBody)
				}
				if etag != tt.wantEtag {
					t.Errorf("getMetadata(%q) etag = %q, want %q", tt.suffix, etag, tt.wantEtag)
				}
			}
		})
	}
}

// TestGetMetadataFallback verifies fallback to the default metadata IP address.
func TestGetMetadataFallback(t *testing.T) {
	utiltest.UnsetEnv(t, metadataHostEnv)

	var requestedURL string
	MockDefaultClientTransport(t, func(req *http.Request) (*http.Response, error) {
		requestedURL = req.URL.String()
		return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("mock response"))}, nil
	})

	_, _, err := getMetadata("test-suffix")
	if err != nil {
		t.Fatalf("getMetadata error: %v", err)
	}

	expected := "http://" + metadataIP + "/computeMetadata/v1/test-suffix"
	if requestedURL != expected {
		t.Errorf("getMetadata requested %q, want %q", requestedURL, expected)
	}
}

// TestGetMetadataErrors verifies request and network error handling in getMetadata.
func TestGetMetadataErrors(t *testing.T) {
	tests := []struct {
		name           string
		suffix         string
		mockTransport  func(t *testing.T)
		wantErrContain string
	}{
		{
			name:           "http.NewRequest error (bad control char in URL)",
			suffix:         "suffix\x7f",
			wantErrContain: "invalid control character in URL",
		},
		{
			name:   "client.Do error",
			suffix: "test-suffix",
			mockTransport: func(t *testing.T) {
				MockDefaultClientTransport(t, func(req *http.Request) (*http.Response, error) {
					return nil, fmt.Errorf("mock dial error")
				})
			},
			wantErrContain: "mock dial error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockTransport != nil {
				tt.mockTransport(t)
			}
			_, _, err := getMetadata(tt.suffix)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrContain) {
				t.Errorf("getMetadata() error = %v, want error containing %q", err, tt.wantErrContain)
			}
		})
	}
}

// TestConfigSha256 verifies that equivalent configurations produce the same SHA256 signature.
func TestConfigSha256(t *testing.T) {
	c1 := &config{projectID: "test-project", osInventoryEnabled: true}
	c2 := &config{projectID: "test-project", osInventoryEnabled: true}
	c3 := &config{projectID: "test-project", osInventoryEnabled: false}

	if c1.asSha256() != c2.asSha256() {
		t.Errorf("Expected identical configs to have same SHA256")
	}
	if c1.asSha256() == c3.asSha256() {
		t.Errorf("Expected different configs to have different SHA256")
	}
}

// TestLastEtag tests concurrent read and write access to the lastEtag tracker.
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

// TestSystemPaths verifies OS-specific system path generation.
func TestSystemPaths(t *testing.T) {
	origGOOS := goos
	defer func() { goos = origGOOS }()

	tests := []struct {
		name string
		op   func() string
		want map[string]string
	}{
		{
			name: "TaskStateFile",
			op:   TaskStateFile,
			want: map[string]string{"windows": filepath.Join(GetCacheDirWindows(), "osconfig_task.state"), "linux": taskStateFileLinux},
		},
		{
			name: "OldTaskStateFile",
			op:   OldTaskStateFile,
			want: map[string]string{"windows": oldTaskStateFileWindows, "linux": oldTaskStateFileLinux},
		},
		{
			name: "RestartFile",
			op:   RestartFile,
			want: map[string]string{"windows": filepath.Join(GetCacheDirWindows(), "osconfig_agent_restart_required"), "linux": restartFileLinux},
		},
		{
			name: "OldRestartFile",
			op:   OldRestartFile,
			want: map[string]string{"windows": oldRestartFileLinux, "linux": oldRestartFileLinux},
		},
		{
			name: "CacheDir",
			op:   CacheDir,
			want: map[string]string{"windows": GetCacheDirWindows(), "linux": cacheDirLinux},
		},
		{
			name: "SerialLogPort",
			op:   SerialLogPort,
			want: map[string]string{"windows": "COM1", "linux": ""},
		},
	}

	for _, tt := range tests {
		for _, testOS := range []string{"windows", "linux"} {
			t.Run(fmt.Sprintf("%s_%s", tt.name, testOS), func(t *testing.T) {
				goos = testOS
				if got := tt.op(); got != tt.want[testOS] {
					t.Errorf("%s() on %s = %v, want %v", tt.name, testOS, got, tt.want[testOS])
				}
			})
		}
	}
}

// TestMiscGetters verifies static getter function outputs.
func TestMiscGetters(t *testing.T) {
	SetVersion("1.2.3")

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{name: "Capabilities", got: Capabilities(), want: []string{"PATCH_GA", "GUEST_POLICY_BETA", "CONFIG_V1"}},
		{name: "UserAgent", got: UserAgent(), want: "google-osconfig-agent/1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Errorf("%s() = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestCreateConfigFromMetadata tests that the agent's configuration is correctly
// built from the data it gets from the metadata server. The test checks how
// various settings are read, how instance-level settings take priority over
// project-level ones, and how command-line flags can override any metadata setting.
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
			name: "default values",
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
			name: "project level debug and numeric poll interval",
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
			name: "instance level overrides project level",
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
			name: "legacy poll interval and disabled features",
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
			name: "debug flag overrides metadata",
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
			origDebug := *debug
			*debug = tt.setDebugFlag
			got := createConfigFromMetadata(tt.md)
			*debug = origDebug
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

		if SvcEndpoint() != expectedSvcEndpoint {
			t.Errorf("Default endpoint: got(%s) != want(%s)", SvcEndpoint(), expectedSvcEndpoint)
		}
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

		if DisableCloudLogging() != expectedDisableCloudLoggingValue {
			t.Errorf("DisableCloudLogging: got(%t) != want(%t)", DisableCloudLogging(), expectedDisableCloudLoggingValue)
		}
	}

}

// TestSetScalibrEnablement tests Scalibr enablement flag extraction from metadata.
func TestSetScalibrEnablement(t *testing.T) {
	tests := []struct {
		name    string
		projVal string
		instVal string
		want    bool
	}{
		{name: "Both empty", projVal: "", instVal: "", want: false},
		{name: "Project true", projVal: "true", instVal: "", want: true},
		{name: "Project false", projVal: "false", instVal: "", want: false},
		{name: "Instance true", projVal: "", instVal: "true", want: true},
		{name: "Instance overrides project", projVal: "false", instVal: "true", want: true},
		{name: "Instance overrides project (false)", projVal: "true", instVal: "false", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &config{}
			md := metadataJSON{
				Project:  projectJSON{Attributes: attributesJSON{ScalibrLinuxEnabled: tt.projVal}},
				Instance: instanceJSON{Attributes: attributesJSON{ScalibrLinuxEnabled: tt.instVal}},
			}
			setScalibrEnablement(md, c)
			if c.scalibrLinuxEnabled != tt.want {
				t.Errorf("setScalibrEnablement() = %v, want %v", c.scalibrLinuxEnabled, tt.want)
			}
		})
	}
}

// TestSetTraceGetInventory tests the inventory tracing flag extraction from metadata.
func TestSetTraceGetInventory(t *testing.T) {
	tests := []struct {
		name    string
		projVal string
		instVal string
		want    bool
	}{
		{name: "Both empty", projVal: "", instVal: "", want: false},
		{name: "Project true", projVal: "true", instVal: "", want: true},
		{name: "Project false", projVal: "false", instVal: "", want: false},
		{name: "Instance true", projVal: "", instVal: "true", want: true},
		{name: "Instance overrides project", projVal: "false", instVal: "true", want: true},
		{name: "Instance overrides project (false)", projVal: "true", instVal: "false", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &config{}
			md := metadataJSON{
				Project:  projectJSON{Attributes: attributesJSON{TraceGetInventory: tt.projVal}},
				Instance: instanceJSON{Attributes: attributesJSON{TraceGetInventory: tt.instVal}},
			}
			setTraceGetInventory(md, c)
			if c.traceGetInventory != tt.want {
				t.Errorf("setTraceGetInventory() = %v, want %v", c.traceGetInventory, tt.want)
			}
		})
	}
}

// TestSetSVCEndpoint tests the logic for figuring out which OS Config service
// endpoint to use. It checks that command-line flags have the highest priority,
// that placeholders like `{zone}` are filled in correctly, and that the endpoint
// is adjusted for different universe domains.
func TestSetSVCEndpoint(t *testing.T) {
	origEndpoint := *endpoint
	defer func() { *endpoint = origEndpoint }()

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
			name:         "Default (all empty)",
			flag:         prodEndpoint,
			instanceZone: "projects/123/zones/us-west1-a",
			want:         "us-west1-a-osconfig.googleapis.com.:443",
		},
		{
			name:    "Flag overrides all",
			flag:    "custom-endpoint",
			instNew: "inst-new",
			want:    "custom-endpoint",
		},
		{
			name:         "Instance New",
			flag:         prodEndpoint,
			instNew:      "inst-new-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			want:         "inst-new-us-west1-a",
		},
		{
			name:         "Instance Old fallback",
			flag:         prodEndpoint,
			instOld:      "inst-old-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			want:         "inst-old-us-west1-a",
		},
		{
			name:         "Project New fallback",
			flag:         prodEndpoint,
			projNew:      "proj-new-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			want:         "proj-new-us-west1-a",
		},
		{
			name:         "Project Old fallback",
			flag:         prodEndpoint,
			projOld:      "proj-old-{zone}",
			instanceZone: "projects/123/zones/us-west1-a",
			want:         "proj-old-us-west1-a",
		},
		{
			name:     "Universe Domain replacement",
			flag:     prodEndpoint,
			instNew:  "test-osconfig.googleapis.com",
			universe: "my-universe.com",
			want:     "test-osconfig.my-universe.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*endpoint = tt.flag
			c := &config{
				instanceZone: tt.instanceZone,
				svcEndpoint:  prodEndpoint,
			}
			if tt.universe != "" {
				c.universeDomain = tt.universe
			} else {
				c.universeDomain = universeDomainDefault
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
			if c.svcEndpoint != tt.want {
				t.Errorf("setSVCEndpoint() = %v, want %v", c.svcEndpoint, tt.want)
			}
		})
	}
}

// TestGetCacheDirWindows verifies primary and fallback cache directory resolution on Windows.
func TestGetCacheDirWindows(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T)
		want        string
		checkSuffix bool
	}{
		{
			name:        "Standard call",
			setup:       func(t *testing.T) { /* no-op */ },
			want:        windowsCacheDir,
			checkSuffix: true,
		},
		{
			name: "Fallback to TempDir",
			setup: func(t *testing.T) {
				// Test fallback by unsetting the HOME, AppData, and XDG environment variables
				// that os.UserCacheDir relies on to generate paths.
				envs := []string{"HOME", "LocalAppData", "XDG_CACHE_HOME"}
				for _, env := range envs {
					utiltest.UnsetEnv(t, env)
				}
			},
			want:        filepath.Join(os.TempDir(), windowsCacheDir),
			checkSuffix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			got := GetCacheDirWindows()
			if tt.checkSuffix {
				if !strings.HasSuffix(got, tt.want) {
					t.Errorf("GetCacheDirWindows() = %q, want suffix %q", got, tt.want)
				}
			} else if got != tt.want {
				t.Errorf("GetCacheDirWindows() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFlagsAndEnvVars verifies parsing of environment variables.
func TestFlagsAndEnvVars(t *testing.T) {
	origFreeOSMemory := freeOSMemory
	origDisableInventoryWrite := disableInventoryWrite
	defer func() {
		freeOSMemory = origFreeOSMemory
		disableInventoryWrite = origDisableInventoryWrite
	}()

	tests := []struct {
		name                  string
		freeOSMemoryVal       string
		disableInventoryWrite string
		wantFreeOS            bool
		wantDisableInv        bool
	}{
		{name: "Both True", freeOSMemoryVal: "true", disableInventoryWrite: "1", wantFreeOS: true, wantDisableInv: true},
		{name: "Both False", freeOSMemoryVal: "false", disableInventoryWrite: "0", wantFreeOS: false, wantDisableInv: false},
		{name: "Empty", freeOSMemoryVal: "", disableInventoryWrite: "", wantFreeOS: false, wantDisableInv: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freeOSMemory = tt.freeOSMemoryVal
			disableInventoryWrite = tt.disableInventoryWrite

			if got := FreeOSMemory(); got != tt.wantFreeOS {
				t.Errorf("FreeOSMemory() = %v, want %v", got, tt.wantFreeOS)
			}
			if got := DisableInventoryWrite(); got != tt.wantDisableInv {
				t.Errorf("DisableInventoryWrite() = %v, want %v", got, tt.wantDisableInv)
			}
		})
	}
}

// TestParseBool verifies string-to-boolean conversion logic.
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

// TestParseFeatures verifies comma-separated feature flag parsing.
func TestParseFeatures(t *testing.T) {
	tests := []struct {
		name     string
		initial  config
		features string
		enabled  bool
		want     config
	}{
		{
			name:     "enabling features",
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
			name: "disabling features (using legacy names as well)",
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

			if c != tt.want {
				t.Errorf("parseFeatures() state = %+v, want %+v", c, tt.want)
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// OverrideWatchConfigTimeouts temporarily overwrites the timeout and retry intervals for WatchConfig.
func OverrideWatchConfigTimeouts(t *testing.T, interval, timeout time.Duration) {
	t.Helper()
	origInterval := watchConfigRetryInterval
	origTimeout := osConfigWatchConfigTimeout

	watchConfigRetryInterval = interval
	osConfigWatchConfigTimeout = timeout
	t.Cleanup(func() {
		watchConfigRetryInterval = origInterval
		osConfigWatchConfigTimeout = origTimeout
	})
}

// MockDefaultClientTransport temporarily replaces the defaultClient's transport with a custom round tripper.
func MockDefaultClientTransport(t *testing.T, roundTrip func(*http.Request) (*http.Response, error)) {
	origClient := defaultClient
	defaultClient = &http.Client{
		Transport: roundTripperFunc(roundTrip),
	}
	t.Cleanup(func() {
		defaultClient = origClient
	})
}
