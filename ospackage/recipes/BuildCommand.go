package recipes

import (
	"fmt"
	"os"
	"strings"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

func BuildCommand(step *osconfigpb.SoftwareRecipe_Step, artifacts map[string]string) ([]string, error) {
	switch v := step.Step.(type) {
	case *osconfigpb.SoftwareRecipe_Step_FileCopy:
		return StepFileCopy(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction:
		return StepArchiveExtraction(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_MsiInstallation:
		return StepMsiInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_DpkgInstallation:
		return StepDpkgInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_RpmInstallation:
		return StepRpmInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_FileExec:
		return StepFileExec(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ScriptRun:
		return StepScriptRun(v, artifacts)
	default:
		return nil, fmt.Errorf("I don't know about step type %T!\n", v)
	}
}

func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepFileCopy")
	return nil, nil
}

func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepArchiveExtraction")
	return nil, nil
}

func StepMsiInstallation(step *osconfigpb.SoftwareRecipe_Step_MsiInstallation, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepMsiInstallation")
	return nil, nil
}

func StepDpkgInstallation(step *osconfigpb.SoftwareRecipe_Step_DpkgInstallation, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepDpkgInstallation")
	return nil, nil
}

func StepRpmInstallation(step *osconfigpb.SoftwareRecipe_Step_RpmInstallation, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepRpmInstallation")
	return nil, nil
}

func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec, artifacts map[string]string) ([]string, error) {
	var path string
	switch v := step.FileExec.LocationType.(type) {
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_LocalPath:
		path = v.LocalPath
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId:
		var ok bool
		path, ok = artifacts[v.ArtifactId]
		if !ok {
			return nil, fmt.Errorf("%q not found in artifact map", v.ArtifactId)
		}
	default:
		return nil, fmt.Errorf("Can't determine location type")
	}

	res := []string{path}
	res = append(res, step.FileExec.Args...)
	return res, nil
}

func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, artifacts map[string]string) ([]string, error) {
	// TODO: should be putting this in stepN_type/ dir, but that needs me to know the dir way in advance..
	// actually this is an artifact. we should have made them referenced as artifacts.. we'll have to repeat file creation logic here.
	f, err := os.Create("/tmp/scriptrun")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	f.WriteString(step.ScriptRun.Script)
	f.Sync()
	if err := os.Chmod("/tmp/scriptrun", 0755); err != nil {
		return nil, err
	}

	res := []string{"/bin/sh", "-c"}
	//if step.ScriptRun.Interpreter == osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL {
	var qargs []string
	for _, arg := range step.ScriptRun.Args {
		qargs = append(qargs, fmt.Sprintf("%q", arg))
	}
	res = append(res, "/tmp/scriptrun"+" "+strings.Join(qargs, " "))
	return res, nil
}
