package workflow

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
)

func CreateTestWorkflows(tests interface{}, images []string, regex *regexp.Regexp) (map[string]*daisy.Workflow, error) {
	// Create base test workflows.
	workflows := make(map[string]*daisy.Workflow)
	typ := reflect.TypeOf(tests)
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		if regex != nil && !regex.MatchString(m.Name) {
			continue
		}
		for _, image := range images {
			fmt.Printf("creating vm workflow for %s\n", image)
			wf, err := createVMWorkflow(m.Name, image)
			if err != nil {
				return nil, fmt.Errorf("failed to create workflow for %q", m.Name)
			}
			nameAndImage := fmt.Sprintf("%s [%s]", m.Name, image)
			workflows[nameAndImage] = wf
		}
	}

	// Add wait signals to top-level WFs.
	for _, wf := range workflows {
		waitStepName := fmt.Sprintf("wait-instance-%s", wf.Name)
		step, err := wf.NewStep(waitStepName)
		if err != nil {
			return nil, fmt.Errorf("Failed to create workflow step: %v", err)
		}
		instanceSignal := &daisy.InstanceSignal{
			Name: wf.Name,
			SerialOutput: &daisy.SerialOutput{
				Port:         1,
				SuccessMatch: "TEST-SUCCESS",
				FailureMatch: daisy.FailureMatches{"TEST-FAILURE"},
			},
		}
		step.WaitForInstancesSignal = &daisy.WaitForInstancesSignal{instanceSignal}
		step.Timeout = "15m"
		wf.AddDependency(wf.Steps[waitStepName], wf.Steps[wf.Name])
	}

	return workflows, nil
}
