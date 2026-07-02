package pretty

import (
	"encoding/json"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

// TestMarshalOptions tests the MarshalOptions function.
func TestMarshalOptions(t *testing.T) {
	opts := MarshalOptions()

	utiltest.AssertEquals(t, opts.Indent, "  ")
	utiltest.AssertEquals(t, opts.AllowPartial, true)
	utiltest.AssertEquals(t, opts.UseProtoNames, true)
	utiltest.AssertEquals(t, opts.EmitUnpopulated, true)
	utiltest.AssertEquals(t, opts.UseEnumNumbers, false)
}

// TestFormat tests the Format function.
func TestFormat(t *testing.T) {
	var dummyMap map[string]any
	jsonUnmarshalErr := json.Unmarshal([]byte("<nil>"), &dummyMap)
	wantFormat := `(?s)^\{.+\}$`
	var gotMap map[string]any

	tests := []struct {
		name    string
		input   *agentendpointpb.Inventory_OsInfo
		wantMap map[string]any
		wantErr error
	}{
		{
			name: "populated fields, want formatted JSON with matching values",
			input: &agentendpointpb.Inventory_OsInfo{
				Hostname:             "Hostname",
				LongName:             "LongName",
				ShortName:            "ShortName",
				Version:              "Version",
				Architecture:         "Architecture",
				KernelVersion:        "KernelVersion",
				KernelRelease:        "KernelRelease",
				OsconfigAgentVersion: "OSConfigAgentVersion",
			},
			wantMap: map[string]any{
				"hostname":               "Hostname",
				"long_name":              "LongName",
				"short_name":             "ShortName",
				"version":                "Version",
				"architecture":           "Architecture",
				"kernel_version":         "KernelVersion",
				"kernel_release":         "KernelRelease",
				"osconfig_agent_version": "OSConfigAgentVersion",
			},
			wantErr: nil,
		},
		{
			name:  "empty message, want formatted JSON with unpopulated fields",
			input: &agentendpointpb.Inventory_OsInfo{},
			wantMap: map[string]any{
				"hostname":               "",
				"long_name":              "",
				"short_name":             "",
				"version":                "",
				"architecture":           "",
				"kernel_version":         "",
				"kernel_release":         "",
				"osconfig_agent_version": "",
			},
			wantErr: nil,
		},
		{
			name:    "nil input, want empty output and json unmarshal error",
			input:   nil,
			wantMap: nil,
			wantErr: jsonUnmarshalErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotStr := Format(tc.input)
			gotErr := json.Unmarshal([]byte(gotStr), &gotMap)

			utiltest.AssertErrorMatchAndSkip(t, gotErr, tc.wantErr)
			utiltest.AssertFormatMatch(t, gotStr, wantFormat)
			utiltest.AssertEquals(t, gotMap, tc.wantMap)
		})
	}
}
