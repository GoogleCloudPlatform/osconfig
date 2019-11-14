package main

import (
	"fmt"

	"github.com/GoogleCloudPlatform/osconfig/e2etester/executor"
	"github.com/GoogleCloudPlatform/osconfig/integration_test/test_classes"
)

func main() {
	xcutor := executor.GetITestExecutor()
	xcutor.AddTestOptions(executor.NewTestOptions("zypper",
		[]string{"projects/suse-cloud/global/images/family/sles-12",
			"projects/suse-cloud/global/images/family/sles-15"},
		test_classes.ZypperTest{}))
	for _, t := range xcutor.GetTestOptions() {
		fmt.Printf("running test_suite: %s\n", t.Name)
		xcutor.Execute(t)
		fmt.Printf("finished running test_suite\n")
	}
	fmt.Printf("tests completed\n")
}
