package workflow

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"google.golang.org/api/compute/v1"
)

func createVMWorkflow(name, image string) (*daisy.Workflow, error) {
	wf := daisy.New()
	wf.Name = name
	wf.Sources = map[string]string{"startup": os.Args[0]}

	diskStepName := fmt.Sprintf("create-disk-%s", name)
	bootdisk := &daisy.Disk{
		Disk: compute.Disk{
			SourceImage: image,
			Name:        diskStepName,
		},
	}

	step, err := wf.NewStep(diskStepName)
	if err != nil {
		return nil, err
	}
	step.CreateDisks = &daisy.CreateDisks{bootdisk}

	var md map[string]string
	if md == nil {
		md = make(map[string]string)
	}
	md["TestName"] = name
	instance := &daisy.Instance{
		Instance:      compute.Instance{Name: name},
		StartupScript: "startup",
		Metadata:      md,
		Resource:      daisy.Resource{RealName: fmt.Sprintf("test-instance-%s", wf.ID())},
		Scopes: []string{
			"https://www.googleapis.com/auth/devstorage.read_only",
			"https://www.googleapis.com/auth/compute",
		},
	}
	instance.Disks = append(instance.Disks, &compute.AttachedDisk{Source: diskStepName})
	step, err = wf.NewStep(name)
	if err != nil {
		return nil, err
	}
	step.CreateInstances = &daisy.CreateInstances{instance}

	wf.AddDependency(wf.Steps[name], wf.Steps[diskStepName])
	return wf, nil
}
