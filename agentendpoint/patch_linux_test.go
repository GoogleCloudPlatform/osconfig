//  Copyright 2022 Google Inc. All Rights Reserved.
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
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/ospatch"
)

func TestExcludeConversion(t *testing.T) {
	regex, _ := regexp.Compile("PackageName")
	emptyRegex, _ := regexp.Compile("")

	tests := []struct {
		name  string
		input []string
		want  []*ospatch.Exclude
	}{
		{name: "StrictStringConversion", input: []string{"PackageName"}, want: CreateStringExcludes("PackageName")},
		{name: "MultipleStringConversion", input: []string{"PackageName1", "PackageName2"}, want: CreateStringExcludes("PackageName1", "PackageName2")},
		{name: "RegexConversion", input: []string{"/PackageName/"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(regex)}},
		{name: "CornerCaseRegex", input: []string{"//"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(emptyRegex)}},
		{name: "CornerCaseStrictString", input: []string{"/"}, want: CreateStringExcludes("/")},
		{name: "CornerCaseEmptyString", input: []string{""}, want: CreateStringExcludes("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludes, err := convertInputToExcludes(tt.input)
			if err != nil {
				t.Errorf("err = %v, want %v", err, nil)
			}
			if !reflect.DeepEqual(excludes, tt.want) {
				t.Errorf("convertInputToExcludes() = %s, want = %s", toString(excludes), toString(tt.want))
			}
		})
	}
}

func toString(excludes []*ospatch.Exclude) string {
	results := make([]string, len(excludes))
	for i, exc := range excludes {
		results[i] = exc.String()
	}

	return strings.Join(results, ",")
}

func CreateStringExcludes(pkgs ...string) []*ospatch.Exclude {
	excludes := make([]*ospatch.Exclude, len(pkgs))
	for i := 0; i < len(pkgs); i++ {
		pkg := pkgs[i]
		excludes[i] = ospatch.CreateStringExclude(&pkg)
	}

	return excludes
}
