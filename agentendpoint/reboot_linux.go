//  Copyright 2018 Google Inc. All Rights Reserved.
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
	"os/exec"
	"syscall"

	"github.com/GoogleCloudPlatform/osconfig/util"
)

const (
	systemctl = "/bin/systemctl"
	reboot    = "/bin/reboot"
	shutdown  = "/bin/shutdown"
)

func rebootSystem() error {
	// Start with systemctl and work down a list of reboot methods.
	if e := util.Exists(systemctl); e {
		return exec.Command(systemctl, "reboot").Start()
	}
	if e := util.Exists(reboot); e {
		return exec.Command(reboot).Run()
	}
	if e := util.Exists(shutdown); e {
		return exec.Command(shutdown, "-r", "-t", "0").Run()
	}

	// Fall back to reboot(2) system call
	syscall.Sync()
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}
