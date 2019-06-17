package recipes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

const (
	RECIPE_BASE_PATH = "/tmp/osconfig_software_recipes"
)

type RecipeDB struct{}

func newRecipeDB() RecipeDB {
	return RecipeDB{}
}

type Recipe struct {
	version []int
}

func convertVersion(version string) ([]int, error) {
	if version == "" {
		return []int{0}, nil
	}
	var ret []int
	for idx, element := range strings.Split(version, ".") {
		if idx > 3 {
			return nil, fmt.Errorf("Invalid version string")
		}
		val, err := strconv.ParseUint(element, 10, 0)
		if err != nil {
			return nil, fmt.Errorf("Invalid version string")
		}
		ret = append(ret, int(val))
	}
	return ret, nil
}

func (r *Recipe) SetVersion(version string) error {
	var err error
	r.version, err = convertVersion(version)
	return err
}

// Greater returns true if the provided version parameter is greater than the
// recipe's version, false otherwise.
func (r *Recipe) Greater(version string) bool {
	if version == "" {
		return false
	}
	cVersion, err := convertVersion(version)
	if err != nil {
		return false
	}
	if len(r.version) > len(cVersion) {
		topad := len(r.version) - len(cVersion)
		for i := 0; i < topad; i++ {
			cVersion = append(cVersion, 0)
		}
	} else {
		topad := len(cVersion) - len(r.version)
		for i := 0; i < topad; i++ {
			r.version = append(r.version, 0)
		}
	}
	for i := 0; i < len(r.version); i++ {
		if r.version[i] != cVersion[i] {
			return cVersion[i] > r.version[i]
		}
	}
	return false
}

func (db *RecipeDB) GetRecipe(name string) (Recipe, bool) {
	return Recipe{}, false
}

// anything implementing isSoftwareRecipe_Step_Step() can be used as type isSoftware
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

func InstallRecipe(ctx context.Context, recipe osconfigpb.SoftwareRecipe) error {
	steps := recipe.InstallSteps
	recipeDB := newRecipeDB()
	installedRecipe, ok := recipeDB.GetRecipe(recipe.Name)
	if ok {
		if (!installedRecipe.Greater(recipe.Version)) &&
			(recipe.DesiredState == osconfigpb.DesiredState_UPDATED) {
			steps = recipe.UpdateSteps
		} else {
			// log skipping recipe
			return nil
		}
	}
	artifacts, err := FetchArtifacts(recipe.Artifacts)
	if err != nil {
		return err
	}
	for _, step := range steps {
		cmd, err := BuildCommand(step, artifacts)
		if err != nil {
			return err
		}
		err = Exec(cmd, recipe, "runId", "stepName", artifacts)
		if err != nil {
			return err
		}
	}
	return nil
}

// Each step handler function parses the step object, crafts the final command
// with arguments and hands it to the executor. The executor sets up working
// dirs, environment variables, and captures output of command.
func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy,
	artifacts map[string]string) ([]string, error) {
	fmt.Println("StepFileCopy")
	return nil, nil
}

func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction,
	artifacts map[string]string) ([]string, error) {
	fmt.Println("StepArchiveExtraction")
	return nil, nil
}

func StepMsiInstallation(step *osconfigpb.SoftwareRecipe_Step_MsiInstallation,
	artifacts map[string]string) ([]string, error) {
	fmt.Println("StepMsiInstallation")
	return nil, nil
}

func StepDpkgInstallation(step *osconfigpb.SoftwareRecipe_Step_DpkgInstallation,
	artifacts map[string]string) ([]string, error) {
	fmt.Println("StepDpkgInstallation")
	return nil, nil
}

func StepRpmInstallation(step *osconfigpb.SoftwareRecipe_Step_RpmInstallation,
	artifacts map[string]string) ([]string, error) {
	fmt.Println("StepRpmInstallation")
	return nil, nil
}

func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec,
	artifacts map[string]string) ([]string, error) {
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

func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun,
	artifacts map[string]string) ([]string, error) {
	// TODO: should be putting this in stepN_type/ dir
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

func FetchArtifacts(recipeArtifacts []*osconfigpb.SoftwareRecipe_Artifact) (map[string]string, error) {
	artifacts := make(map[string]string)
	for _, artifact := range recipeArtifacts {
		artifacts[artifact.Id] = artifact.Uri
	}
	return artifacts, nil
}

func Exec(cmd []string, recipe osconfigpb.SoftwareRecipe, runId, stepName string, artifacts map[string]string) error {
	// ${ROOT}/recipe[_ver]/runId/recipe.yaml  // recipe at time of application
	// ${ROOT}/recipe[_ver]/runId/artifacts/*
	// ${ROOT}/recipe[_ver]/runId/stepN_type/
	cmdObj := exec.Command(cmd[0], cmd[1:]...)
	//cmdObj := exec.Command("echo", cmd...)
	name := recipe.Name
	if recipe.Version != "" {
		name = fmt.Sprintf("%s_%s", name, recipe.Version)
	}
	cmdObj.Dir = path.Join(RECIPE_BASE_PATH, name, runId, stepName)
	if err := os.MkdirAll(cmdObj.Dir, os.ModeDir|0755); err != nil {
		return fmt.Errorf("Failed to create step working dir: %s", err)
	}
	cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("RECIPE_NAME=%s", recipe.Name))
	cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("RECIPE_VERSION=%s", recipe.Version))
	cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("RUNID=%s", runId))
	cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("PWD=%s", cmdObj.Dir))
	for artifactId, artifactPath := range artifacts {
		cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("%s=%s", artifactId, artifactPath))
	}
	stdout, err := cmdObj.Output()
	fmt.Println(string(stdout))
	return err
}
