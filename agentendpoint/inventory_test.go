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

package agentendpoint

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/inventory"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
)

type agentEndpointServiceInventoryTestServer struct {
	lastReportInventoryRequest *agentendpointpb.ReportInventoryRequest
	reportFullInventory        bool
}

func (*agentEndpointServiceInventoryTestServer) ReceiveTaskNotification(req *agentendpointpb.ReceiveTaskNotificationRequest, srv agentendpointpb.AgentEndpointService_ReceiveTaskNotificationServer) error {
	return status.Errorf(codes.Unimplemented, "method ReceiveTaskNotification not implemented")
}

func (*agentEndpointServiceInventoryTestServer) StartNextTask(ctx context.Context, req *agentendpointpb.StartNextTaskRequest) (*agentendpointpb.StartNextTaskResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartNextTask not implemented")
}

func (*agentEndpointServiceInventoryTestServer) ReportTaskProgress(ctx context.Context, req *agentendpointpb.ReportTaskProgressRequest) (*agentendpointpb.ReportTaskProgressResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReportTaskProgress not implemented")
}

func (*agentEndpointServiceInventoryTestServer) ReportTaskComplete(ctx context.Context, req *agentendpointpb.ReportTaskCompleteRequest) (*agentendpointpb.ReportTaskCompleteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReportTaskComplete not implemented")
}

func (*agentEndpointServiceInventoryTestServer) RegisterAgent(ctx context.Context, req *agentendpointpb.RegisterAgentRequest) (*agentendpointpb.RegisterAgentResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterAgent not implemented")
}

func (s *agentEndpointServiceInventoryTestServer) ReportInventory(ctx context.Context, req *agentendpointpb.ReportInventoryRequest) (*agentendpointpb.ReportInventoryResponse, error) {
	s.lastReportInventoryRequest = req
	resp := &agentendpointpb.ReportInventoryResponse{ReportFullInventory: s.reportFullInventory}
	if s.reportFullInventory {
		s.reportFullInventory = false
	}
	return resp, nil
}

func generateInventoryState() *inventory.InstanceInventory {
	return &inventory.InstanceInventory{
		Hostname:             "Hostname",
		LongName:             "LongName",
		ShortName:            "ShortName",
		Version:              "Version",
		Architecture:         "Architecture",
		KernelVersion:        "KernelVersion",
		KernelRelease:        "KernelRelease",
		OSConfigAgentVersion: "OSConfigAgentVersion",
		InstalledPackages: &packages.Packages{
			Yum: []*packages.PkgInfo{{
				Name:    "YumInstalledPkg",
				Arch:    "Arch",
				Version: "Version",
				Source:  packages.Source{Name: "SourceName", Version: "SourceVersion"}}},

			Rpm: []*packages.PkgInfo{{
				Name:    "RpmInstalledPkg",
				Arch:    "Arch",
				Version: "Version",
				Source:  packages.Source{Name: "SourceName", Version: "SourceVersion"}}},

			Apt: []*packages.PkgInfo{{
				Name:    "AptInstalledPkg",
				Arch:    "Arch",
				Version: "Version",
				Source:  packages.Source{Name: "SourceName", Version: "SourceVersion"}}},
			Deb: []*packages.PkgInfo{{
				Name:    "DebInstalledPkg",
				Arch:    "Arch",
				Version: "Version",
				Source:  packages.Source{Name: "SourceName", Version: "SourceVersion"}}},
			Zypper:        []*packages.PkgInfo{{Name: "ZypperInstalledPkg", Arch: "Arch", Version: "Version"}},
			ZypperPatches: []*packages.ZypperPatch{{Name: "ZypperInstalledPatch", Category: "Category", Severity: "Severity", Summary: "Summary"}},
			Gem:           []*packages.PkgInfo{{Name: "GemInstalledPkg", Arch: "Arch", Version: "Version"}},
			Pip:           []*packages.PkgInfo{{Name: "PipInstalledPkg", Arch: "Arch", Version: "Version"}},
			GooGet:        []*packages.PkgInfo{{Name: "GooGetInstalledPkg", Arch: "Arch", Version: "Version"}},
			WUA: []*packages.WUAPackage{{
				Title:                    "WUAInstalled",
				Description:              "Description",
				Categories:               []string{"Category1", "Category2", "Category3", "Category4"},
				CategoryIDs:              []string{"CategoryID1", "CategoryID2", "CategoryID3", "CategoryID4"},
				KBArticleIDs:             []string{"KB1", "KB2", "KB3", "KB4"},
				MoreInfoURLs:             []string{"MoreInfoURL1", "MoreInfoURL2", "MoreInfoURL3", "MoreInfoURL4"},
				SupportURL:               "SupportURL",
				UpdateID:                 "UpdateID",
				RevisionNumber:           1,
				LastDeploymentChangeTime: time.Date(2020, time.November, 10, 23, 0, 0, 0, time.UTC)}},
			QFE: []*packages.QFEPackage{{Caption: "QFEInstalled", Description: "Description", HotFixID: "HotFixID", InstalledOn: "9/1/2020"}},
			COS: []*packages.PkgInfo{{Name: "CosInstalledPkg", Arch: "Arch", Version: "Version"}},
		},
		PackageUpdates: &packages.Packages{
			Yum:           []*packages.PkgInfo{{Name: "YumPkgUpdate", Arch: "Arch", Version: "Version"}},
			Apt:           []*packages.PkgInfo{{Name: "AptPkgUpdate", Arch: "Arch", Version: "Version"}},
			Zypper:        []*packages.PkgInfo{{Name: "ZypperPkgUpdate", Arch: "Arch", Version: "Version"}},
			ZypperPatches: []*packages.ZypperPatch{{Name: "ZypperPatchUpdate", Category: "Category", Severity: "Severity", Summary: "Summary"}},
			Gem:           []*packages.PkgInfo{{Name: "GemPkgUpdate", Arch: "Arch", Version: "Version"}},
			Pip:           []*packages.PkgInfo{{Name: "PipPkgUpdate", Arch: "Arch", Version: "Version"}},
			GooGet:        []*packages.PkgInfo{{Name: "GooGetPkgUpdate", Arch: "Arch", Version: "Version"}},
			WUA: []*packages.WUAPackage{{
				Title:       "WUAUpdate",
				Description: "Description",

				Categories:   []string{"Category1", "Category2", "Category3", "Category4"},
				CategoryIDs:  []string{"CategoryID1", "CategoryID2", "CategoryID3", "CategoryID4"},
				KBArticleIDs: []string{"KB1", "KB2", "KB3", "KB4"},
				MoreInfoURLs: []string{"MoreInfoURL1", "MoreInfoURL2", "MoreInfoURL3", "MoreInfoURL4"},

				SupportURL:               "SupportURL",
				UpdateID:                 "UpdateID",
				RevisionNumber:           1,
				LastDeploymentChangeTime: time.Time{}}},
		},
	}
}

func generateInventory() *agentendpointpb.Inventory {
	return &agentendpointpb.Inventory{
		OsInfo: &agentendpointpb.Inventory_OsInfo{
			Hostname:             "Hostname",
			LongName:             "LongName",
			ShortName:            "ShortName",
			Version:              "Version",
			Architecture:         "Architecture",
			KernelVersion:        "KernelVersion",
			KernelRelease:        "KernelRelease",
			OsconfigAgentVersion: "OSConfigAgentVersion",
		},
		InstalledPackages: []*agentendpointpb.Inventory_SoftwarePackage{
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_AptPackage{
					AptPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "AptInstalledPkg",
						Architecture: "Arch",
						Version:      "Version",
						Source: &agentendpointpb.Inventory_VersionedPackage_Source{
							Name:    "SourceName",
							Version: "SourceVersion",
						},
					}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_AptPackage{
					AptPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "DebInstalledPkg",
						Architecture: "Arch",
						Version:      "Version",
						Source: &agentendpointpb.Inventory_VersionedPackage_Source{
							Name:    "SourceName",
							Version: "SourceVersion",
						},
					}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_GoogetPackage{
					GoogetPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "GooGetInstalledPkg",
						Architecture: "Arch",
						Version:      "Version"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_YumPackage{
					YumPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "YumInstalledPkg",
						Architecture: "Arch",
						Version:      "Version",
						Source: &agentendpointpb.Inventory_VersionedPackage_Source{
							Name:    "SourceName",
							Version: "SourceVersion",
						},
					}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_ZypperPackage{
					ZypperPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "ZypperInstalledPkg",
						Architecture: "Arch",
						Version:      "Version"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_YumPackage{
					YumPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "RpmInstalledPkg",
						Architecture: "Arch",
						Version:      "Version",
						Source: &agentendpointpb.Inventory_VersionedPackage_Source{
							Name:    "SourceName",
							Version: "SourceVersion",
						},
					}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_ZypperPatch{
					ZypperPatch: &agentendpointpb.Inventory_ZypperPatch{
						PatchName: "ZypperInstalledPatch",
						Category:  "Category",
						Severity:  "Severity",
						Summary:   "Summary"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_WuaPackage{
					WuaPackage: &agentendpointpb.Inventory_WindowsUpdatePackage{
						Title:       "WUAInstalled",
						Description: "Description",
						Categories: []*agentendpointpb.Inventory_WindowsUpdatePackage_WindowsUpdateCategory{
							{Id: "CategoryID1", Name: "Category1"},
							{Id: "CategoryID2", Name: "Category2"},
							{Id: "CategoryID3", Name: "Category3"},
							{Id: "CategoryID4", Name: "Category4"},
						},
						KbArticleIds:             []string{"KB1", "KB2", "KB3", "KB4"},
						SupportUrl:               "SupportURL",
						MoreInfoUrls:             []string{"MoreInfoURL1", "MoreInfoURL2", "MoreInfoURL3", "MoreInfoURL4"},
						UpdateId:                 "UpdateID",
						RevisionNumber:           1,
						LastDeploymentChangeTime: timestamppb.New(time.Date(2020, time.November, 10, 23, 0, 0, 0, time.UTC)),
					}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_QfePackage{
					QfePackage: &agentendpointpb.Inventory_WindowsQuickFixEngineeringPackage{
						Caption:     "QFEInstalled",
						Description: "Description",
						HotFixId:    "HotFixID",
						InstallTime: timestamppb.New(time.Date(2020, time.September, 1, 0, 0, 0, 0, time.UTC))}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_CosPackage{
					CosPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "CosInstalledPkg",
						Architecture: "Arch",
						Version:      "Version"}}},
		},
		AvailablePackages: []*agentendpointpb.Inventory_SoftwarePackage{
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_AptPackage{
					AptPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "AptPkgUpdate",
						Architecture: "Arch",
						Version:      "Version"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_GoogetPackage{
					GoogetPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "GooGetPkgUpdate",
						Architecture: "Arch",
						Version:      "Version"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_YumPackage{
					YumPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "YumPkgUpdate",
						Architecture: "Arch",
						Version:      "Version"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_ZypperPackage{
					ZypperPackage: &agentendpointpb.Inventory_VersionedPackage{
						PackageName:  "ZypperPkgUpdate",
						Architecture: "Arch",
						Version:      "Version"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_ZypperPatch{
					ZypperPatch: &agentendpointpb.Inventory_ZypperPatch{
						PatchName: "ZypperPatchUpdate",
						Category:  "Category",
						Severity:  "Severity",
						Summary:   "Summary"}}},
			{
				Details: &agentendpointpb.Inventory_SoftwarePackage_WuaPackage{
					WuaPackage: &agentendpointpb.Inventory_WindowsUpdatePackage{
						Title:       "WUAUpdate",
						Description: "Description",
						Categories: []*agentendpointpb.Inventory_WindowsUpdatePackage_WindowsUpdateCategory{

							{Id: "CategoryID1", Name: "Category1"},
							{Id: "CategoryID2", Name: "Category2"},
							{Id: "CategoryID3", Name: "Category3"},
							{Id: "CategoryID4", Name: "Category4"},
						},
						KbArticleIds:             []string{"KB1", "KB2", "KB3", "KB4"},
						SupportUrl:               "SupportURL",
						MoreInfoUrls:             []string{"MoreInfoURL1", "MoreInfoURL2", "MoreInfoURL3", "MoreInfoURL4"},
						UpdateId:                 "UpdateID",
						RevisionNumber:           1,
						LastDeploymentChangeTime: timestamppb.New(time.Time{})}}},
		},
	}
}

func decodePackages(str string) *packages.Packages {
	decoded, _ := base64.StdEncoding.DecodeString(str)
	zr, _ := gzip.NewReader(bytes.NewReader(decoded))
	var buf bytes.Buffer
	io.Copy(&buf, zr)
	zr.Close()

	var pkgs packages.Packages
	json.Unmarshal(buf.Bytes(), &pkgs)
	return &pkgs
}

func TestWrite(t *testing.T) {
	inv := &inventory.InstanceInventory{
		Hostname:      "Hostname",
		LongName:      "LongName",
		ShortName:     "ShortName",
		Architecture:  "Architecture",
		KernelVersion: "KernelVersion",
		KernelRelease: "KernelRelease",
		Version:       "Version",
		InstalledPackages: &packages.Packages{
			Yum: []*packages.PkgInfo{{Name: "Name", Arch: "Arch", Version: "Version"}},
			WUA: []*packages.WUAPackage{{Title: "Title"}},
			QFE: []*packages.QFEPackage{{HotFixID: "HotFixID"}},
		},
		PackageUpdates: &packages.Packages{
			Apt: []*packages.PkgInfo{{Name: "Name", Arch: "Arch", Version: "Version"}},
		},
		OSConfigAgentVersion: "OSConfigAgentVersion",
		LastUpdated:          "LastUpdated",
	}

	want := map[string]bool{
		"Hostname":             false,
		"LongName":             false,
		"ShortName":            false,
		"Architecture":         false,
		"KernelVersion":        false,
		"Version":              false,
		"InstalledPackages":    false,
		"PackageUpdates":       false,
		"OSConfigAgentVersion": false,
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
		case "/KernelRelease":
			if buf.String() != inv.KernelRelease {
				t.Errorf("did not get expected KernelRelease, got: %q, want: %q", buf.String(), inv.KernelRelease)
			}
			want["KernelRelease"] = true
		case "/Version":
			if buf.String() != inv.Version {
				t.Errorf("did not get expected Version, got: %q, want: %q", buf.String(), inv.Version)
			}
			want["Version"] = true
		case "/InstalledPackages":
			got := decodePackages(buf.String())
			if !reflect.DeepEqual(got, inv.InstalledPackages) {
				t.Errorf("did not get expected InstalledPackages, got: %+v, want: %+v", got, inv.InstalledPackages)
			}
			want["InstalledPackages"] = true
		case "/PackageUpdates":
			got := decodePackages(buf.String())
			if !reflect.DeepEqual(got, inv.PackageUpdates) {
				t.Errorf("did not get expected PackageUpdates, got: %+v, want: %+v", got, inv.PackageUpdates)
			}
			want["PackageUpdates"] = true
		case "/OSConfigAgentVersion":
			if buf.String() != inv.OSConfigAgentVersion {
				t.Errorf("did not get expected OSConfigAgentVersion, got: %q, want: %q", buf.String(), inv.OSConfigAgentVersion)
			}
			want["OSConfigAgentVersion"] = true
		case "/LastUpdated":
			if buf.String() != inv.LastUpdated {
				t.Errorf("did not get expected LastUpdated, got: %q, want: %q", buf.String(), inv.LastUpdated)
			}
			want["LastUpdated"] = true
		default:
			w.WriteHeader(500)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, url)
		}
	}))
	defer svr.Close()

	ctx := context.Background()
	write(ctx, inv, svr.URL)

	for k, v := range want {
		if v {
			continue
		}
		t.Errorf("writeInventory call did not write %q", k)
	}
}

func TestReport(t *testing.T) {
	ctx := context.Background()
	srv := &agentEndpointServiceInventoryTestServer{}
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatal(err)
	}
	defer tc.close()

	tests := []struct {
		name                string
		reportFullInventory bool
		inventoryState      *inventory.InstanceInventory
		wantInventory       *agentendpointpb.Inventory
	}{
		{
			name:                "ReportChecksumOnly",
			reportFullInventory: false,
			inventoryState:      generateInventoryState(),
			wantInventory:       nil,
		},
		{
			name:                "ReportFullInventory",
			reportFullInventory: true,
			inventoryState:      generateInventoryState(),
			wantInventory:       generateInventory(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv.reportFullInventory = tt.reportFullInventory

			tc.client.report(ctx, tt.inventoryState)

			actualInventory := srv.lastReportInventoryRequest.Inventory
			if diff := cmp.Diff(tt.wantInventory, actualInventory, protocmp.Transform()); diff != "" {
				t.Fatalf("ReportInventoryRequest.Inventory mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_computeFingerprint_gotExpectedFingerprintFormat(t *testing.T) {
	ctx := context.Background()
	fingerprint, err := computeFingerprint(ctx, generateInventory())
	if err != nil {
		t.Fatalf("unable to generate fingerprint, err - %s", err)
	}

	if !validFingerprint(fingerprint) {
		t.Fatalf("got fingerprint %q in unexpected format", fingerprint)
	}
}

func Test_computeFingerprint_fingerprintChangedAfterReshuffle(t *testing.T) {
	ctx := context.Background()
	inventory := generateInventory()
	initialFingerprint, err := computeFingerprint(ctx, inventory)
	if err != nil {
		t.Fatalf("unable to generate initial fingerprint, err - %s", err)
	}

	inventoryPackagesMixer(time.Now().UnixNano())(inventory)

	finalFingerprint, err := computeFingerprint(ctx, inventory)
	if err != nil {
		t.Fatalf("unable to generate final fingerprint, err - %s", err)
	}

	if initialFingerprint == finalFingerprint {
		t.Errorf("unstable fingerprint is equal for different inventories")
	}
}

func Test_computeStableFingerprint_fingerprintNotChangedAfterReshuffle(t *testing.T) {
	ctx := context.Background()
	inventory := generateInventory()
	initialFingerprint, err := computeStableFingerprint(ctx, inventory)
	if err != nil {
		t.Fatalf("unable to generate initial fingerprint, err - %s", err)
	}

	inventoryPackagesMixer(time.Now().UnixNano())(inventory)

	finalFingerprint, err := computeStableFingerprint(ctx, inventory)
	if err != nil {
		t.Fatalf("unable to generate final fingerprint, err - %s", err)
	}

	if initialFingerprint != finalFingerprint {
		t.Errorf("stable fingerprint is not equal for inventory with different order")
	}
}

func Test_computeStableFingerprint_fingerprintChangedAfterChanging(t *testing.T) {
	ctx := context.Background()
	inventory := generateInventory()
	initialFingerprint, err := computeStableFingerprint(ctx, inventory)
	if err != nil {
		t.Fatalf("unable to generate initial fingerprint, err - %s", err)
	}

	inventory.AvailablePackages = nil

	finalFingerprint, err := computeStableFingerprint(ctx, inventory)
	if err != nil {
		t.Fatalf("unable to generate final fingerprint, err - %s", err)
	}

	if initialFingerprint == finalFingerprint {
		t.Errorf("stable fingerprint should not be equal if inventory is actually changed")
	}
}

func validFingerprint(fingerprint string) bool {
	if fingerprint == "" {
		return false
	}

	if len(fingerprint) != 64 {
		return false
	}

	return true
}

func Test_computeStableFingerprint_testFingerprintStability(t *testing.T) {
	ctx := context.Background()
	inventory := generateInventory()

	mixer := inventoryPackagesMixer(time.Now().UnixNano())

	n := 100
	for i := 0; i < n; i++ {
		t.Run(fmt.Sprintf("Run %d", i), func(t *testing.T) {
			// Compute initial fingerprints.
			unstableFingerprintBefore, err := computeFingerprint(ctx, inventory)
			if err != nil {
				t.Errorf("unexpected error while calculating fingerprint, err %v", err)
			}
			stableFingerprintBefore, err := computeStableFingerprint(ctx, inventory)
			if err != nil {
				t.Errorf("unexpected error while calculating fingerprint, err %v", err)
			}

			// Reorder pacakges in the inventory, that should change result of computeFingerprint
			// and do not change the result of computeStableFingerptint.
			mixer(inventory)

			// Generate new fingerprints after reshufling of the packages.
			unstableFingerprintAfter, err := computeFingerprint(ctx, inventory)
			if err != nil {
				t.Errorf("unexpected error while calculating fingerprint, err %v", err)
			}
			stableFingerprintAfter, err := computeStableFingerprint(ctx, inventory)
			if err != nil {
				t.Errorf("unexpected error while calculating fingerprint, err %v", err)
			}

			//Confirm that unstable fingerprint changed and stable fingerprint not.
			if unstableFingerprintBefore == unstableFingerprintAfter {
				t.Errorf("unstable fingerprint is equal for different inventories")
			}
			if stableFingerprintBefore != stableFingerprintAfter {
				t.Errorf("stable fingerprint is not equal for identical inventories")
			}
		})

	}
}

func inventoryPackagesMixer(source int64) func(inventory *agentendpointpb.Inventory) {
	fmt.Printf("Inventory packages mixer initialized with source %d\n", source)

	rng := rand.New(rand.NewSource(source))

	return func(inventory *agentendpointpb.Inventory) {
		rng.Shuffle(len(inventory.InstalledPackages), func(i, j int) {
			inventory.InstalledPackages[i], inventory.InstalledPackages[j] = inventory.InstalledPackages[j], inventory.InstalledPackages[i]
		})

		rng.Shuffle(len(inventory.AvailablePackages), func(i, j int) {
			inventory.AvailablePackages[i], inventory.AvailablePackages[j] = inventory.AvailablePackages[j], inventory.AvailablePackages[i]
		})

	}
}

func Benchmark_computeFingerprint(b *testing.B) {
	ctx, inventory := context.Background(), generateInventory()

	for i := 0; i < b.N; i++ {
		_, err := computeFingerprint(ctx, inventory)
		if err != nil {
			b.Fatalf("unable to generate fingerprint, err - %s", err)
		}
	}
}

func Benchmark_computeStableFingerprint(b *testing.B) {
	ctx, inventory := context.Background(), generateInventory()

	for i := 0; i < b.N; i++ {
		_, err := computeStableFingerprint(ctx, inventory)
		if err != nil {
			b.Fatalf("unable to generate fingerprint, err - %s", err)
		}
	}
}
