package pretty

import (
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
)

func TestFormat(t *testing.T) {
	//flaky test
	t.Skip()

	input := &agentendpointpb.Inventory_OsInfo{
		Hostname:             "Hostname",
		LongName:             "LongName",
		ShortName:            "ShortName",
		Version:              "Version",
		Architecture:         "Architecture",
		KernelVersion:        "KernelVersion",
		KernelRelease:        "KernelRelease",
		OsconfigAgentVersion: "OSConfigAgentVersion",
	}

	expected := "{\n  \"hostname\":  \"Hostname\",\n  \"long_name\":  \"LongName\",\n  \"short_name\":  \"ShortName\",\n  \"version\":  \"Version\",\n  \"architecture\":  \"Architecture\",\n  \"kernel_version\":  \"KernelVersion\",\n  \"kernel_release\":  \"KernelRelease\",\n  \"osconfig_agent_version\":  \"OSConfigAgentVersion\"\n}"

	output := Format(input)
	if expected != output {
		t.Errorf("unexpected output\nexpected:\n%q\ngot:\n%q", expected, output)
	}
}
