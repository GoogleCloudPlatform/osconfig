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

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	agentEndpoint          = flag.String("agent_endpoint", "", "API endpoint to use for the agent to use for the tests")
	endpoint               = flag.String("endpoint", "osconfig.googleapis.com:443", "API endpoint to use for the tests")
	oauthDefault           = flag.String("local_oauth", "", "path to service creds file")
	agentRepo              = flag.String("agent_repo", "", "repo to pull agent from (unstable, staging, or stable, leave blank for no agent install)")
	bucketDefault          = "osconfig-agent-end2end-tests"
	logPushIntervalDefault = 3 * time.Second
	logsPath               = fmt.Sprintf("logs-%s", time.Now().Format("2006-01-02-15:04:05"))
	testSuiteRegex         *regexp.Regexp
	testSuiteFilter        = flag.String("test_suite_filter", "", "test suite filter")
	testCaseRegex          *regexp.Regexp
	testCaseFilter         = flag.String("test_case_filter", "", "test case filter")
	zones                  map[string]int
	testZone               = flag.String("test_zone", "", "test zone")
	testZones              = flag.String("test_zones", "{}", "test zones")
	projects               []string
	testProjectIDs         = flag.String("test_project_ids", "", "test project ids")

	// OutDir is the out directory to use.
	OutDir = flag.String("out_dir", "/tmp", "artifact directory")
)

func init() {
	flag.Parse()

	if *testSuiteFilter != "" {
		var err error
		testSuiteRegex, err = regexp.Compile(*testSuiteFilter)
		if err != nil {
			fmt.Println("-test_suite_filter flag not valid:", err)
			os.Exit(1)
		}
	}

	if *testCaseFilter != "" {
		var err error
		testCaseRegex, err = regexp.Compile(*testCaseFilter)
		if err != nil {
			fmt.Println("-test_case_filter flag not valid:", err)
			os.Exit(1)
		}
	}

	if len(strings.TrimSpace(*testProjectIDs)) == 0 {
		fmt.Println("-test_project_ids must be specified")
		os.Exit(1)
	}

	projects = strings.Split(*testProjectIDs, ",")

	zones = make(map[string]int)
	if len(strings.TrimSpace(*testZone)) != 0 {
		zones[*testZone] = math.MaxInt32
	} else {
		err := json.Unmarshal([]byte(*testZones), &zones)
		if err != nil {
			fmt.Printf("Error parsing zones `%s`\n", *testZones)
			os.Exit(1)
		}
	}

	if len(zones) == 0 {
		fmt.Println("Error, no zones specified")
		os.Exit(1)
	}
}

// Projects are the projects to use.
func Projects() []string {
	return projects
}

// Zones are the zones and associated quota to use.
func Zones() map[string]int {
	return zones
}

// TestSuiteFilter is the test suite filter regex.
func TestSuiteFilter() *regexp.Regexp {
	return testSuiteRegex
}

// TestCaseFilter is the test case filter regex.
func TestCaseFilter() *regexp.Regexp {
	return testCaseRegex
}

// AgentRepo returns the agentRepo
func AgentRepo() string {
	return *agentRepo
}

// AgentSvcEndpoint returns the agentEndpoint
func AgentSvcEndpoint() string {
	return *agentEndpoint
}

// SvcEndpoint returns the endpoint
func SvcEndpoint() string {
	return *endpoint
}

// OauthPath returns the oauthPath file path
func OauthPath() string {
	return *oauthDefault
}

// LogBucket returns the oauthPath file path
func LogBucket() string {
	return bucketDefault
}

// LogsPath returns the oauthPath file path
func LogsPath() string {
	return logsPath
}

// LogPushInterval returns the interval at which the serial console logs are written to GCS
func LogPushInterval() time.Duration {
	return logPushIntervalDefault
}
