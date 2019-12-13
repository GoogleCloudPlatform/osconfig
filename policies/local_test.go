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
//
package policies

import (
	"testing"

	"github.com/golang/protobuf/proto"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

const (
	sampleConfig string = `{
		       "packages": [
			 {
			   "name": "my-package",
			   "desiredState": "INSTALLED",
			   "manager": "APT"
			 },
			 {
			   "name": "my-other-package",
			   "desired_state": "INSTALLED",
			   "manager": "APT"
			 }
		       ],
		       "packageRepositories": [
			 {
			   "apt": {
			     "uri": "http://packages.cloud.google.com/apt",
			     "archiveType": "DEB",
			     "distribution": "google-cloud-monitoring-stretch",
			     "components": [
			       "main"
			     ],
			     "gpgKey": "https://packages.cloud.google.com/apt/doc/apt-key.gpg"
			   },
                           "yum": {
			     "id": "my-yum",
			     "display_name": "my-yum-name",
			     "base_url": "http://my-base-url",
			     "gpg_keys": ["https://packages.cloud.google.com/apt/doc/apt-key.gpg"]
			   }

			 }
		       ],
		       "softwareRecipes": [
			 {
                            "name": "install-envoy",
                            "desired_state": "INSTALLED",
			    "installSteps": [
			      {
				"scriptRun": {
				  "script": ""
				}
			      }
			    ]
			  },
                          {
                            "name": "install-something",
                            "desired_state": "INSTALLED",
			    "installSteps": [
			      {
				"scriptRun": {
				  "script": ""
				}
			      }
			    ]
			  }
		       ]
		     }`
)

func TestJsonParsing(t *testing.T) {
	a := []byte(sampleConfig)
	pr, err := parseLocalConfig(a)
	if err != nil {
		t.Errorf("Got error: %v", err)
		return
	}

	if got, want := pr.SoftwareRecipes[0].DesiredState, agentendpointpb.DesiredState_INSTALLED; got != want {
		t.Errorf("Software recipe desired state incorrectly parsed, got: %d(%s), want: %d(%s).", got, got.String(), want, want.String())
	}
}

func TestMerging(t *testing.T) {
	a := []byte(sampleConfig)
	lc, err := parseLocalConfig(a)
	if err != nil {
		t.Errorf("Got error: %v", err)
		return
	}
	var pr agentendpointpb.EffectiveGuestPolicy
	var sr agentendpointpb.EffectiveGuestPolicy_SourcedSoftwareRecipe
	sr.Source = "policy1"
	sr.SoftwareRecipe = new(agentendpointpb.SoftwareRecipe)
	sr.SoftwareRecipe.Name = "install-something"
	sr.SoftwareRecipe.DesiredState = agentendpointpb.DesiredState_REMOVED
	pr.SoftwareRecipes = append(pr.SoftwareRecipes, &sr)
	pr2 := mergeConfigs(lc, &pr)

	var wantmap = map[string]agentendpointpb.DesiredState{
		"install-something": agentendpointpb.DesiredState_REMOVED,
		"install-envoy":     agentendpointpb.DesiredState_INSTALLED,
	}
	for _, ssr := range pr2.SoftwareRecipes {
		gotState := ssr.SoftwareRecipe.DesiredState
		wantState, ok := wantmap[ssr.SoftwareRecipe.Name]
		if !ok {
			t.Errorf("Recipe ame: %s unexpected.", ssr.SoftwareRecipe.Name)
			continue
		}
		if gotState != wantState {
			t.Errorf("Recipe: %s got state: %d want state: %d", ssr.SoftwareRecipe.Name, gotState, wantState)
		}
	}
	rs := pr2.SoftwareRecipes[0].SoftwareRecipe.DesiredState
	want := agentendpointpb.DesiredState_REMOVED
	if rs != want {
		t.Logf("Merged: %s", proto.MarshalTextString(&pr2))
		t.Errorf("Wrong recipe state. Got: %d(%s), want: %d(%s).", rs, rs.String(), want, want.String())
	}

}
