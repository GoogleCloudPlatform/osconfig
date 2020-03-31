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

package agentendpointbeta

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/external"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

var (
	winRoot = os.Getenv("SystemRoot")
	sh      = "/bin/sh"

	winPowershell string
	winCmd        string

	winPowershellArgs = []string{"-NonInteractive", "-NoProfile", "-ExecutionPolicy", "Bypass"}

	goos = runtime.GOOS

	errLinuxPowerShell = errors.New("interpreter POWERSHELL cannot be used on non-Windows system")
	errWinNoInt        = fmt.Errorf("interpreter must be specified for a Windows system")
)

func init() {
	if winRoot == "" {
		winRoot = `C:\Windows`
	}
	winPowershell = filepath.Join(winRoot, `System32\WindowsPowerShell\v1.0\PowerShell.exe`)
	winCmd = filepath.Join(winRoot, `System32\cmd.exe`)
}

var run = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.CombinedOutput()
}

func getGCSObject(ctx context.Context, gcsObject *agentendpointpb.GcsObject, loggingLabels map[string]string) (string, error) {
	if gcsObject == nil {
		return "", errors.New("gcsObject cannot be nil")
	}

	cl, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("error creating gcs client: %v", err)
	}
	reader, err := external.FetchGCSObject(ctx, cl, gcsObject.Object, gcsObject.Bucket, gcsObject.GenerationNumber)
	if err != nil {
		return "", fmt.Errorf("error fetching GCS object: %v", err)
	}
	defer reader.Close()
	logger.Debugf("Fetched GCS object bucket %s object %s generation number %d", gcsObject.GetBucket(), gcsObject.GetObject(), gcsObject.GetGenerationNumber())

	localPath := filepath.Join(os.TempDir(), path.Base(gcsObject.GetObject()))
	if err := external.DownloadStream(reader, "", localPath, 0755); err != nil {
		return "", fmt.Errorf("error downloading GCS object: %s", err)
	}

	logger.Debugf("Downloaded to local path %s", localPath)
	return localPath, nil
}

func executeCommand(path string, args []string, loggingLabels map[string]string) (int32, error) {
	logger.Debugf("Running command %s with args %s", path, args)

	cmd := exec.Command(path, args...)
	out, err := run(cmd)
	var exitCode int32
	if cmd.ProcessState != nil {
		exitCode = int32(cmd.ProcessState.ExitCode())
		logger.Infof("Command exit code: %d, out:\n%s", exitCode, out)
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}

	return exitCode, nil
}

type execTask struct {
	client *Client

	TaskID    string
	Task      *execStepTask
	StartedAt time.Time         `json:",omitempty"`
	LogLabels map[string]string `json:",omitempty"`
}

type execStepTask struct {
	*agentendpointpb.ExecStepTask
}

func (e *execTask) reportCompletedState(ctx context.Context, errMsg string, output *agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput) error {
	req := &agentendpointpb.ReportTaskCompleteRequest{
		TaskId:       e.TaskID,
		TaskType:     agentendpointpb.TaskType_EXEC_STEP_TASK,
		ErrorMessage: errMsg,
		Output:       output,
	}
	if err := e.client.reportTaskComplete(ctx, req); err != nil {
		return fmt.Errorf("error reporting completed state: %v", err)
	}
	return nil
}

func (e *execTask) run(ctx context.Context) error {
	e.StartedAt = time.Now()
	req := &agentendpointpb.ReportTaskProgressRequest{
		TaskId:   e.TaskID,
		TaskType: agentendpointpb.TaskType_EXEC_STEP_TASK,
		Progress: &agentendpointpb.ReportTaskProgressRequest_ExecStepTaskProgress{
			ExecStepTaskProgress: &agentendpointpb.ExecStepTaskProgress{State: agentendpointpb.ExecStepTaskProgress_STARTED},
		},
	}
	res, err := e.client.reportTaskProgress(ctx, req)
	if err != nil {
		return fmt.Errorf("error reporting state %s: %v", agentendpointpb.ExecStepTaskProgress_STARTED, err)
	}

	if res.GetTaskDirective() == agentendpointpb.TaskDirective_STOP {
		return e.reportCompletedState(ctx, errServerCancel.Error(), &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
			ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{State: agentendpointpb.ExecStepTaskOutput_CANCELLED},
		})
	}

	stepConfig := e.Task.GetExecStep().GetLinuxExecStepConfig()
	if goos == "windows" {
		stepConfig = e.Task.GetExecStep().GetWindowsExecStepConfig()
	}

	localPath := stepConfig.GetLocalPath()
	if gcsObject := stepConfig.GetGcsObject(); gcsObject != nil {
		var err error
		localPath, err = getGCSObject(ctx, gcsObject, e.LogLabels)
		if err != nil {
			return fmt.Errorf("error getting executable path: %v", err)
		}
		defer func() {
			if err := os.Remove(localPath); err != nil {
				logger.Errorf("error removing downloaded file %s", err)
			}
		}()
	}

	exitCode := int32(-1)
	switch stepConfig.GetInterpreter() {
	case agentendpointpb.ExecStepConfig_INTERPRETER_UNSPECIFIED:
		if goos == "windows" {
			err = errWinNoInt
		} else {
			exitCode, err = executeCommand(localPath, nil, e.LogLabels)
		}
	case agentendpointpb.ExecStepConfig_SHELL:
		if goos == "windows" {
			exitCode, err = executeCommand(winCmd, []string{"/c", localPath}, e.LogLabels)
		} else {
			exitCode, err = executeCommand(sh, []string{localPath}, e.LogLabels)
		}
	case agentendpointpb.ExecStepConfig_POWERSHELL:
		if goos == "windows" {
			exitCode, err = executeCommand(winPowershell, append(winPowershellArgs, "-File", localPath), e.LogLabels)
		} else {
			err = errLinuxPowerShell
		}
	default:
		err = fmt.Errorf("invalid interpreter %q", stepConfig.GetInterpreter())
	}
	if err != nil {
		return e.reportCompletedState(ctx, err.Error(), &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
			ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
				State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
				ExitCode: exitCode,
			},
		})
	}

	return e.reportCompletedState(ctx, "", &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
		ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
			State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
			ExitCode: exitCode,
		},
	})
}

// RunExecStep runs an exec step task.
func (c *Client) RunExecStep(ctx context.Context, task *agentendpointpb.Task) error {
	e := &execTask{
		TaskID:    task.GetTaskId(),
		client:    c,
		Task:      &execStepTask{task.GetExecStepTask()},
		LogLabels: mkLabels(task),
	}

	return e.run(ctx)
}
