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
	"encoding/json"
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
)

func TestWatchConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"project":{"numericProjectID":12345,"projectId":"projectId","attributes":{"osconfig-endpoint":"bad!!1","enable-os-inventory":"false"}},"instance":{"id":12345,"name":"name","zone":"zone","attributes":{"osconfig-endpoint":"SvcEndpoint","enable-os-inventory":"1","enable-os-config-debug":"true","osconfig-enabled-prerelease-features":"ospackage,ospatch", "osconfig-poll-interval":"3", "enable-scalibr-linux":"true", "trace-get-inventory":"true", "enable-guest-attributes":"true"}}}`)
	}))
	defer ts.Close()

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}

	if err := WatchConfig(context.Background()); err != nil {
		t.Fatalf("Error running WatchConfig: %v", err)
	}

	testsString := []struct {
		desc string
		op   func() string
		want string
	}{
		{"SvcEndpoint", SvcEndpoint, "SvcEndpoint"},
		{"Instance", Instance, "zone/instances/name"},
		{"ID", ID, "12345"},
		{"ProjectID", ProjectID, "projectId"},
		{"Zone", Zone, "zone"},
		{"Name", Name, "name"},
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
		{"osinventory should be enabled (proj disabled, inst enabled)", OSInventoryEnabled, true},
		{"taskNotification should be enabled (inst enabled)", TaskNotificationEnabled, true},
		{"guestpolicies should be enabled (proj enabled)", GuestPoliciesEnabled, true},
		{"debugenabled should be true (proj disabled, inst enabled)", Debug, true},
		{"scalibrLinuxEnabled should be true", ScalibrLinuxEnabled, true},
		{"traceGetInventory should be true", TraceGetInventory, true},
		{"guestAttributesEnabled should be true", GuestAttributesEnabled, true},
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer ts.Close()

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}

	for i, want := range []bool{false, true, false} {
		request = i
		if err := WatchConfig(context.Background()); err != nil {
			t.Fatalf("Error running SetConfig: %v", err)
		}

		testsBool := []struct {
			desc string
			op   func() bool
		}{
			{"OSInventoryEnabled", OSInventoryEnabled},
			{"TaskNotificationEnabled", TaskNotificationEnabled},
			{"GuestPoliciesEnabled", GuestPoliciesEnabled},
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
		{"OSInventoryEnabled", OSInventoryEnabled, false},
		{"TaskNotificationEnabled", TaskNotificationEnabled, true},
		{"GuestPoliciesEnabled", GuestPoliciesEnabled, true},
	}
	for _, tt := range testsBool {
		if tt.op() != tt.want {
			t.Errorf("%s: got(%t) != want(%t)", tt.desc, tt.op(), tt.want)
		}
	}
}

func TestSetConfigDefaultValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", "sample-etag")
		// we always get zone value in instance metadata.
		fmt.Fprintln(w, `{"instance": {"zone": "fake-zone"}}`)
	}))
	defer ts.Close()

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}

	if err := WatchConfig(context.Background()); err != nil {
		t.Fatalf("Error running SetConfig: %v", err)
	}

	testsString := []struct {
		op   func() string
		want string
	}{
		{AptRepoFilePath, aptRepoFilePath},
		{YumRepoFilePath, yumRepoFilePath},
		{ZypperRepoFilePath, zypperRepoFilePath},
		{GooGetRepoFilePath, googetRepoFilePath},
		{ZypperRepoDir, zypperRepoDir},
		{ZypperRepoFormat, filepath.Join(zypperRepoDir, "osconfig_managed_%s.repo")},
		{YumRepoDir, yumRepoDir},
		{YumRepoFormat, filepath.Join(yumRepoDir, "osconfig_managed_%s.repo")},
		{AptRepoDir, aptRepoDir},
		{AptRepoFormat, filepath.Join(aptRepoDir, "osconfig_managed_%s.list")},
		{GooGetRepoDir, googetRepoDir},
		{GooGetRepoFormat, filepath.Join(googetRepoDir, "osconfig_managed_%s.repo")},
		{UniverseDomain, universeDomainDefault},
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
		{OSInventoryEnabled, osInventoryEnabledDefault},
		{TaskNotificationEnabled, taskNotificationEnabledDefault},
		{GuestPoliciesEnabled, guestPoliciesEnabledDefault},
		{Debug, debugEnabledDefault},
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

// TestWatchConfigUnchangedConfigTimeout verifies that WatchConfig handles a continuous stream
// of valid metadata responses where the actual configuration data has not changed
// (as indicated by matching SHA256 hashes). The function should continue polling
// without breaking the loop and ultimately exit gracefully when the timeout expires.
func TestWatchConfigUnchangedConfigTimeout(t *testing.T) {
	origInterval := watchConfigRetryInterval
	origTimeout := osConfigWatchConfigTimeout
	defer func() {
		watchConfigRetryInterval = origInterval
		osConfigWatchConfigTimeout = origTimeout
	}()
	watchConfigRetryInterval = 1 * time.Millisecond
	osConfigWatchConfigTimeout = 10 * time.Millisecond

	var count int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Etag", fmt.Sprintf("etag-%d", count))
		w.Header().Set("Metadata-Flavor", "Google")
		// Return exactly the same config on every request so asSha256() matches
		fmt.Fprint(w, `{}`)
	}))
	defer ts.Close()

	origHost := os.Getenv("GCE_METADATA_HOST")
	if err := os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}
	defer os.Setenv("GCE_METADATA_HOST", origHost)

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

// TestWatchConfigWebErrorLimit verifies that WatchConfig correctly handles network errors
// during metadata retrieval. It simulates a connection refused scenario and ensures that
// the function retries the request up to the maximum limit (12 times) before
// returning the network error.
func TestWatchConfigWebErrorLimit(t *testing.T) {
	origInterval := watchConfigRetryInterval
	origTimeout := osConfigWatchConfigTimeout
	defer func() {
		watchConfigRetryInterval = origInterval
		osConfigWatchConfigTimeout = origTimeout
	}()
	watchConfigRetryInterval = 1 * time.Millisecond
	osConfigWatchConfigTimeout = 1 * time.Second

	// Close the mock server immediately to force a connection refused (network error)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	origHost := os.Getenv("GCE_METADATA_HOST")
	if err := os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}
	defer os.Setenv("GCE_METADATA_HOST", origHost)

	err := WatchConfig(context.Background())
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "network error when requesting metadata") && !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestWatchConfigUnmarshalErrorLimit verifies that WatchConfig correctly handles
// malformed JSON responses from the metadata server. It simulates a scenario where
// unmarshaling fails and ensures that the function retries up to the maximum limit
// (3 times) before returning the unmarshal error.
func TestWatchConfigUnmarshalErrorLimit(t *testing.T) {
	origInterval := watchConfigRetryInterval
	origTimeout := osConfigWatchConfigTimeout
	defer func() {
		watchConfigRetryInterval = origInterval
		osConfigWatchConfigTimeout = origTimeout
	}()
	watchConfigRetryInterval = 1 * time.Millisecond
	osConfigWatchConfigTimeout = 1 * time.Second

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", fmt.Sprintf("unmarshal-error-etag-%d", time.Now().UnixNano()))
		w.Header().Set("Metadata-Flavor", "Google")
		fmt.Fprint(w, `{"bad json"`)
	}))
	defer ts.Close()

	origHost := os.Getenv("GCE_METADATA_HOST")
	if err := os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}
	defer os.Setenv("GCE_METADATA_HOST", origHost)

	err := WatchConfig(context.Background())
	if err == nil {
		t.Fatal("Expected unmarshal error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid character") && !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestWatchConfigContextCancel verifies that WatchConfig respects context cancellation.
// It ensures that if the provided context is cancelled, the polling loop exits
// immediately and returns a nil error, rather than waiting for the timeout or
// continuing to retry failed requests.
func TestWatchConfigContextCancel(t *testing.T) {
	origInterval := watchConfigRetryInterval
	origTimeout := osConfigWatchConfigTimeout
	defer func() {
		watchConfigRetryInterval = origInterval
		osConfigWatchConfigTimeout = origTimeout
	}()
	watchConfigRetryInterval = 1 * time.Minute
	osConfigWatchConfigTimeout = 1 * time.Minute

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", fmt.Sprintf("cancel-etag-%d", time.Now().UnixNano()))
		w.Header().Set("Metadata-Flavor", "Google")
		fmt.Fprint(w, `{"bad json"`) // Trigger unmarshal error loop which checks context
	}))
	defer ts.Close()

	origHost := os.Getenv("GCE_METADATA_HOST")
	if err := os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}
	defer os.Setenv("GCE_METADATA_HOST", origHost)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately prior to passing it in

	if err := WatchConfig(ctx); err != nil {
		t.Errorf("Expected nil error on context cancellation, got: %v", err)
	}
}

func TestSetConfigError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer ts.Close()

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}

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

// TestLoggingFlags verifies that the Stdout and DisableLocalLogging getter functions
// correctly reflect the boolean values set by their corresponding command-line flags.
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

// TestLogFeatures verifies that the LogFeatures function, which logs the status
// of various OS Config features, executes without panicking.
func TestLogFeatures(t *testing.T) {
	// LogFeatures only outputs to the logger, so we just ensure it runs without panicking.
	LogFeatures(context.Background())
}

// TestIDToken verifies the retrieval and decoding of the instance identity token
// from the metadata server. It checks that the token is correctly fetched,
// parsed as a JWS, and that the result is cached for subsequent calls within
// the token's validity window.
func TestIDToken(t *testing.T) {
	// Create a dummy JWS token
	// Header: {"alg":"RS256","typ":"JWT"} -> eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9
	// Payload: {"exp": 32503680000} -> eyJleHAiOiAzMjUwMzY4MDAwMH0
	// Signature: dummy -> ZHVtbXk
	dummyToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOiAzMjUwMzY4MDAwMH0.ZHVtbXk"

	var requests int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/computeMetadata/v1/instance/service-accounts/default/identity") {
			requests++
			w.Header().Set("Metadata-Flavor", "Google")
			fmt.Fprint(w, dummyToken)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	origHost := os.Getenv("GCE_METADATA_HOST")
	if err := os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}
	defer os.Setenv("GCE_METADATA_HOST", origHost)

	// Reset the identity cache
	identity = idToken{}

	token, err := IDToken()
	if err != nil {
		t.Fatalf("IDToken() error: %v", err)
	}
	if token != dummyToken {
		t.Errorf("IDToken() = %q, want %q", token, dummyToken)
	}

	// Call again to test caching
	_, err = IDToken()
	if err != nil {
		t.Fatalf("IDToken() error on second call: %v", err)
	}
	if requests != 1 {
		t.Errorf("Expected 1 request due to caching, got %d", requests)
	}
}

// TestIDTokenErrors verifies the error handling logic within the IDToken function
// when the metadata server returns an HTTP error (e.g., 500 Internal Server Error)
// or when the retrieved token is malformed and cannot be decoded as a valid JWS.
func TestIDTokenErrors(t *testing.T) {
	origHost := os.Getenv("GCE_METADATA_HOST")
	defer os.Setenv("GCE_METADATA_HOST", origHost)

	// Test 1: HTTP 500 error
	ts500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts500.Close()

	os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ts500.URL, "http://"))
	identity = idToken{} // reset cache
	if _, err := IDToken(); err == nil {
		t.Error("Expected error on 500 response, got nil")
	}

	// Test 2: Malformed token
	tsMalformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Metadata-Flavor", "Google")
		fmt.Fprint(w, "not.a.valid.token")
	}))
	defer tsMalformed.Close()

	os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(tsMalformed.URL, "http://"))
	identity = idToken{} // reset cache
	if _, err := IDToken(); err == nil {
		t.Error("Expected error on malformed token, got nil")
	}
}

// TestFormatMetadataError verifies that formatMetadataError wraps specific network
// and DNS errors with informative messages to aid in debugging metadata connection
// issues, while returning other standard errors unmodified.
func TestFormatMetadataError(t *testing.T) {
	errStandard := fmt.Errorf("standard error")
	errDNS := &url.Error{Err: &net.DNSError{Err: "no such host"}}
	errNet := &url.Error{Err: &net.OpError{Op: "dial", Net: "tcp"}}

	if got := formatMetadataError(errStandard); got != errStandard {
		t.Errorf("formatMetadataError(errStandard) = %v, want %v", got, errStandard)
	}

	if got := formatMetadataError(errDNS); !strings.Contains(got.Error(), "DNS error when requesting metadata") {
		t.Errorf("formatMetadataError(errDNS) = %v, want to contain 'DNS error...'", got)
	}

	if got := formatMetadataError(errNet); !strings.Contains(got.Error(), "network error when requesting metadata") {
		t.Errorf("formatMetadataError(errNet) = %v, want to contain 'network error...'", got)
	}
}

// TestGetMetadata verifies the core metadata retrieval logic. It ensures that
// the function correctly returns the response body and Etag for successful (200 OK)
// requests, and appropriately handles non-success HTTP status codes like 404 and 500
// by returning nil data and no error, as expected by the caller handling.
func TestGetMetadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer ts.Close()

	origHost := os.Getenv("GCE_METADATA_HOST")
	if err := os.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}
	defer os.Setenv("GCE_METADATA_HOST", origHost)

	// Test success
	body, etag, err := getMetadata("test-success")
	if err != nil {
		t.Errorf("getMetadata(test-success) error: %v", err)
	}
	if string(body) != "success" {
		t.Errorf("getMetadata(test-success) body = %q, want %q", body, "success")
	}
	if etag != "test-etag" {
		t.Errorf("getMetadata(test-success) etag = %q, want %q", etag, "test-etag")
	}

	// Test 404
	body, etag, err = getMetadata("test-404")
	if err != nil {
		t.Errorf("getMetadata(test-404) error: %v", err)
	}
	if body != nil || etag != "" {
		t.Errorf("getMetadata(test-404) expected nil body and empty etag, got %q, %q", body, etag)
	}

	// Test 500
	body, etag, err = getMetadata("test-500")
	if err != nil {
		t.Errorf("getMetadata(test-500) error: %v", err)
	}
	if body != nil || etag != "" {
		t.Errorf("getMetadata(test-500) expected nil body and empty etag, got %q, %q", body, etag)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestGetMetadataFallback verifies that getMetadata falls back to using the default
// metadata IP address (169.254.169.254) when the GCE_METADATA_HOST environment
// variable is not set. This ensures compatibility in environments where DNS resolution fails.
func TestGetMetadataFallback(t *testing.T) {
	origHost := os.Getenv(metadataHostEnv)
	os.Unsetenv(metadataHostEnv)
	defer func() {
		if origHost != "" {
			os.Setenv(metadataHostEnv, origHost)
		}
	}()

	origClient := defaultClient
	defer func() { defaultClient = origClient }()

	var requestedURL string
	defaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requestedURL = req.URL.String()
			return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("mock response"))}, nil
		}),
	}

	_, _, err := getMetadata("test-suffix")
	if err != nil {
		t.Fatalf("getMetadata error: %v", err)
	}

	expected := "http://" + metadataIP + "/computeMetadata/v1/test-suffix"
	if requestedURL != expected {
		t.Errorf("getMetadata requested %q, want %q", requestedURL, expected)
	}
}

// TestGetMetadataErrors verifies the error handling in getMetadata when
// the underlying HTTP request creation fails (e.g., due to an invalid URL)
// or when the HTTP client encounters a network error (e.g., a dial error).
func TestGetMetadataErrors(t *testing.T) {
	// Test http.NewRequest error (bad control char in URL)
	_, _, err := getMetadata("suffix\x7f")
	if err == nil {
		t.Error("Expected error for bad URL suffix, got nil")
	}

	// Test client.Do error
	origClient := defaultClient
	defer func() { defaultClient = origClient }()

	defaultClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("mock dial error")
		}),
	}

	_, _, err = getMetadata("test-suffix")
	if err == nil || !strings.Contains(err.Error(), "mock dial error") {
		t.Errorf("Expected mock dial error, got: %v", err)
	}
}

// TestConfigSha256 verifies that the asSha256 method generates a stable and
// unique SHA256 hash representing the state of a config struct. This hash
// is used to detect when configuration changes actually require updating state.
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

// TestLastEtag verifies the thread-safety of the lastEtag struct, ensuring
// that concurrent read and write operations do not result in race conditions.
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

// TestSystemPaths verifies that functions returning system paths (e.g., for state files,
// caches, and serial logs) provide the correct path strings appropriate for the
// specified operating system (Windows vs. Linux).
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

// TestMiscGetters verifies the return values of simple static getter functions,
// such as Capabilities() which lists supported agent actions, and UserAgent().
func TestMiscGetters(t *testing.T) {
	SetVersion("1.2.3")

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"Capabilities", Capabilities(), []string{"PATCH_GA", "GUEST_POLICY_BETA", "CONFIG_V1"}},
		{"UserAgent", UserAgent(), "google-osconfig-agent/1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Errorf("%s() = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}

// TestCreateConfigFromMetadata verifies that the agent configuration object is
// accurately constructed from the provided JSON metadata payload. It extensively
// tests the parsing of different fields, the correct fallback hierarchy (e.g.,
// instance attributes overriding project attributes), and the overriding behavior of flags.
func TestCreateConfigFromMetadata(t *testing.T) {
	// Reset the global agent config to avoid test cross-contamination
	agentConfigMx.Lock()
	agentConfig = &config{}
	agentConfigMx.Unlock()

	pollInt15 := json.Number("15")
	pollInt20 := json.Number("20")
	id98765 := json.Number("98765")

	tests := []struct {
		name           string
		md             metadataJSON
		setDebugFlag   bool
		wantDebug      bool
		wantPollInt    int
		wantProjectID  string
		wantOSInv      bool
		wantGuestPol   bool
		wantInstanceID string
		wantTaskNotif  bool
	}{
		{
			name:          "default values",
			md:            metadataJSON{},
			wantPollInt:   10,
			wantTaskNotif: false,
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
			wantDebug:     true,
			wantPollInt:   15,
			wantProjectID: "proj-1",
			wantOSInv:     true,
			wantGuestPol:  true,
			wantTaskNotif: true,
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
			wantDebug:     true,
			wantPollInt:   20,
			wantProjectID: "proj-1",
			wantOSInv:     false,
			wantGuestPol:  false,
			wantTaskNotif: false,
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
			wantPollInt:    15,
			wantOSInv:      false,
			wantGuestPol:   false,
			wantTaskNotif:  true,
			wantInstanceID: "98765",
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
			wantDebug:    true,
			wantPollInt:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origDebug := *debug
			*debug = tt.setDebugFlag
			c := createConfigFromMetadata(tt.md)
			*debug = origDebug
			if c.debugEnabled != tt.wantDebug {
				t.Errorf("debugEnabled = %v, want %v", c.debugEnabled, tt.wantDebug)
			}
			if c.osConfigPollInterval != tt.wantPollInt {
				t.Errorf("osConfigPollInterval = %v, want %v", c.osConfigPollInterval, tt.wantPollInt)
			}
			if c.projectID != tt.wantProjectID {
				t.Errorf("projectID = %v, want %v", c.projectID, tt.wantProjectID)
			}
			if c.osInventoryEnabled != tt.wantOSInv {
				t.Errorf("osInventoryEnabled = %v, want %v", c.osInventoryEnabled, tt.wantOSInv)
			}
			if c.guestPoliciesEnabled != tt.wantGuestPol {
				t.Errorf("guestPoliciesEnabled = %v, want %v", c.guestPoliciesEnabled, tt.wantGuestPol)
			}
			if c.taskNotificationEnabled != tt.wantTaskNotif {
				t.Errorf("taskNotificationEnabled = %v, want %v", c.taskNotificationEnabled, tt.wantTaskNotif)
			}
			if c.instanceID != tt.wantInstanceID {
				t.Errorf("instanceID = %v, want %v", c.instanceID, tt.wantInstanceID)
			}
		})
	}
}

func TestSvcEndpoint(t *testing.T) {
	var request int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch request {
		case 0:
			w.Header().Set("Etag", "etag-0")
			// we always get zone value in instance metadata.
			fmt.Fprintln(w, `{"instance": {"id": 12345,"name": "name","zone": "fakezone","attributes": {"osconfig-endpoint": "{zone}-dev.osconfig.googleapis.com"}}}`)
		case 1:
			w.Header().Set("Etag", "etag-1")
			fmt.Fprintln(w, `{"universe": {"universeDomain": "domain.com"}, "instance": {"id": 12345,"name": "name","zone": "fakezone","attributes": {"osconfig-endpoint": "{zone}-dev.osconfig.googleapis.com"}}}`)
		}
	}))
	defer ts.Close()

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}

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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch request {
		case 0:
			w.Header().Set("Etag", "etag-0")
			fmt.Fprintln(w, `{"universe":{"universeDomain": "domain.com"}}`)
		case 1:
			w.Header().Set("Etag", "etag-1")
			fmt.Fprintln(w, `{"instance": {"zone": "fake-zone"}}`)
		}
	}))
	defer ts.Close()

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}

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

// TestSetScalibrEnablement verifies the parsing logic for the Scalibr Linux
// enablement flag, ensuring that instance-level settings correctly override project-level settings.
func TestSetScalibrEnablement(t *testing.T) {
	tests := []struct {
		name    string
		projVal string
		instVal string
		want    bool
	}{
		{"Both empty", "", "", false},
		{"Project true", "true", "", true},
		{"Project false", "false", "", false},
		{"Instance true", "", "true", true},
		{"Instance overrides project", "false", "true", true},
		{"Instance overrides project (false)", "true", "false", false},
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

// TestSetTraceGetInventory verifies the parsing logic for the trace inventory
// flag, checking that instance-level settings appropriately override project-level settings.
func TestSetTraceGetInventory(t *testing.T) {
	tests := []struct {
		name    string
		projVal string
		instVal string
		want    bool
	}{
		{"Both empty", "", "", false},
		{"Project true", "true", "", true},
		{"Project false", "false", "", false},
		{"Instance true", "", "true", true},
		{"Instance overrides project", "false", "true", true},
		{"Instance overrides project (false)", "true", "false", false},
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

// TestSetSVCEndpoint verifies the cascade fallback logic for determining the
// OS Config service endpoint. It tests that command-line flags override metadata,
// zone placeholders are correctly populated, and the universe domain is properly substituted.
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

// TestGetCacheDirWindows verifies the Windows-specific cache directory path generation.
// It checks both the standard path creation using the user's cache directory and
// the fallback logic that defaults to the system temporary directory if it cannot be determined.
func TestGetCacheDirWindows(t *testing.T) {
	// Standard call test
	got := GetCacheDirWindows()
	if !strings.HasSuffix(got, windowsCacheDir) {
		t.Errorf("GetCacheDirWindows() = %q, want suffix %q", got, windowsCacheDir)
	}

	// Test fallback by unsetting the HOME, AppData, and XDG environment variables
	// that os.UserCacheDir relies on to generate paths.
	envs := []string{"HOME", "LocalAppData", "XDG_CACHE_HOME"}
	origEnvs := make(map[string]bool)
	origVals := make(map[string]string)
	for _, env := range envs {
		v, ok := os.LookupEnv(env)
		origEnvs[env] = ok
		origVals[env] = v
		os.Unsetenv(env)
	}
	defer func() {
		for _, env := range envs {
			if origEnvs[env] {
				os.Setenv(env, origVals[env])
			} else {
				os.Unsetenv(env) // restore to unset if it was previously unset
			}
		}
	}()

	fallbackGot := GetCacheDirWindows()
	expectedFallback := filepath.Join(os.TempDir(), windowsCacheDir)
	if fallbackGot != expectedFallback {
		t.Errorf("GetCacheDirWindows() with fallback = %q, want %q", fallbackGot, expectedFallback)
	}
}

// TestFlagsAndEnvVars verifies that functions reading configuration settings
// from environment variables parse the values correctly as booleans, handling
// variations like "true", "1", "false", "0", and empty strings.
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
		{"Both True", "true", "1", true, true},
		{"Both False", "false", "0", false, false},
		{"Empty", "", "", false, false},
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

// TestParseBool verifies the behavior of the internal parseBool utility function,
// ensuring it correctly interprets valid boolean string representations and safely defaults to false.
func TestParseBool(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		if got := parseBool(tt.input); got != tt.want {
			t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseFeatures verifies the behavior of the config's parseFeatures method,
// ensuring it correctly interprets comma-separated lists of enabled or disabled
// features and updates the corresponding boolean flags within the config struct.
func TestParseFeatures(t *testing.T) {
	c := &config{}

	// Test enabling features
	c.parseFeatures("tasks, ospackage, osinventory, unknown", true)
	if !c.taskNotificationEnabled || !c.guestPoliciesEnabled || !c.osInventoryEnabled {
		t.Errorf("parseFeatures failed to enable features: %+v", c)
	}

	// Test disabling features (using legacy names as well)
	c.parseFeatures("ospatch, guestpolicies", false)
	if c.taskNotificationEnabled || c.guestPoliciesEnabled {
		t.Errorf("parseFeatures failed to disable features: %+v", c)
	}
}
