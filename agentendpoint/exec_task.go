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

package agentendpoint

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
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/external"
	"github.com/GoogleCloudPlatform/osconfig/util"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
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

func getGCSObject(ctx context.Context, bkt, obj string, gen int64) (string, error) {
	cl, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("error creating gcs client: %v", err)
	}
	reader, err := external.FetchGCSObject(ctx, cl, bkt, obj, gen)
	if err != nil {
		return "", fmt.Errorf("error fetching GCS object: %v", err)
	}
	defer reader.Close()
	clog.Debugf(ctx, "Fetched GCS object bucket %s object %s generation number %d", bkt, obj, gen)

	localPath := filepath.Join(os.TempDir(), path.Base(obj))
	if _, err := util.AtomicWriteFileStream(reader, "", localPath, 0755); err != nil {
		return "", fmt.Errorf("error downloading GCS object: %s", err)
	}

	clog.Debugf(ctx, "Downloaded to local path %s", localPath)
	return localPath, nil
}

func executeCommand(ctx context.Context, path string, args []string) (int32, error) {
	clog.Debugf(ctx, "Running command %s with args %s", path, args)

	cmd := exec.Command(path, args...)
	out, err := run(cmd)
	var exitCode int32
	if cmd.ProcessState != nil {
		exitCode = int32(cmd.ProcessState.ExitCode())
		clog.Infof(ctx, "Command exit code: %d, out:\n%s", exitCode, out)
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}

	return exitCode, nil
}

type execTask struct {
	StartedAt time.Time `json:",omitempty"`
	client    *Client
	Task      *execStepTask
	TaskID    string
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
	clog.Infof(ctx, "Beginning ExecStepTask")
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

	if stepConfig == nil {
		// The given ExecTask does not apply to this OS.
		return e.reportCompletedState(ctx, "", &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
			ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
				State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
				ExitCode: 0,
			},
		})
	}

	localPath := stepConfig.GetLocalPath()
	if gcsObject := stepConfig.GetGcsObject(); gcsObject != nil {
		var err error
		localPath, err = getGCSObject(ctx, gcsObject.GetBucket(), gcsObject.GetObject(), gcsObject.GetGenerationNumber())
		if err != nil {
			msg := fmt.Sprintf("Error downloading GCS object: %v", err)
			clog.Errorf(ctx, msg)
			return e.reportCompletedState(ctx, msg, &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
				ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
					State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
					ExitCode: -1,
				},
			})
		}
		defer func() {
			if err := os.Remove(localPath); err != nil {
				clog.Errorf(ctx, "error removing downloaded file %s", err)
			}
		}()
	}

	exitCode := int32(-1)
	switch stepConfig.GetInterpreter() {
	case agentendpointpb.ExecStepConfig_INTERPRETER_UNSPECIFIED:
		if goos == "windows" {
			err = errWinNoInt
		} else {
			exitCode, err = executeCommand(ctx, localPath, nil)
		}
	case agentendpointpb.ExecStepConfig_SHELL:
		if goos == "windows" {
			exitCode, err = executeCommand(ctx, winCmd, []string{"/c", localPath})
		} else {
			exitCode, err = executeCommand(ctx, sh, []string{localPath})
		}
	case agentendpointpb.ExecStepConfig_POWERSHELL:
		if goos == "windows" {
			exitCode, err = executeCommand(ctx, winPowershell, append(winPowershellArgs, "-File", localPath))
		} else {
			err = errLinuxPowerShell
		}
	default:
		err = fmt.Errorf("invalid interpreter %q", stepConfig.GetInterpreter())
	}
	if err != nil {
		msg := fmt.Sprintf("Error running ExecStepTask: %v", err)
		clog.Errorf(ctx, msg)
		return e.reportCompletedState(ctx, msg, &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
			ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
				State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
				ExitCode: exitCode,
			},
		})
	}

	if err := e.reportCompletedState(ctx, "", &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
		ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
			State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
			ExitCode: exitCode,
		},
	}); err != nil {
		return err
	}
	clog.Infof(ctx, "Successfully completed ApplyConfigTask")
	return nil
}

// RunExecStep runs an ExecStepTask.
func (c *Client) RunExecStep(ctx context.Context, task *agentendpointpb.Task) error {
	ctx = clog.WithLabels(ctx, task.GetServiceLabels())
	e := &execTask{
		TaskID: task.GetTaskId(),
		client: c,
		Task:   &execStepTask{task.GetExecStepTask()},
	}

	return e.run(ctx)
}
