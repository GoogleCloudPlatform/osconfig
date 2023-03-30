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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWatchConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"project":{"numericProjectID":12345,"projectId":"projectId","attributes":{"osconfig-endpoint":"bad!!1","enable-os-inventory":"false"}},"instance":{"id":12345,"name":"name","zone":"zone","attributes":{"osconfig-endpoint":"SvcEndpoint","enable-os-inventory":"1","enable-os-config-debug":"true","osconfig-enabled-prerelease-features":"ospackage,ospatch", "osconfig-poll-interval":"3"}}}`)
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

	expectedEndpoint := "fake-zone-osconfig.googleapis.com:443"
	if SvcEndpoint() != expectedEndpoint {
		t.Errorf("Default endpoint: got(%s) != want(%s)", SvcEndpoint(), expectedEndpoint)
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

func TestSvcEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Etag", "sametag")
		// we always get zone value in instance metadata.
		fmt.Fprintln(w, `{"instance": {"id": 12345,"name": "name","zone": "fakezone","attributes": {"osconfig-endpoint": "{zone}-dev.osconfig.googleapis.com"}}}`)
	}))
	defer ts.Close()

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		t.Fatalf("Error running os.Setenv: %v", err)
	}

	if err := WatchConfig(context.Background()); err != nil {
		t.Fatalf("Error running SetConfig: %v", err)
	}

	expectedSvcEndpoint := "fakezone-dev.osconfig.googleapis.com"
	if SvcEndpoint() != expectedSvcEndpoint {
		t.Errorf("Default endpoint: got(%s) != want(%s)", SvcEndpoint(), expectedSvcEndpoint)
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
