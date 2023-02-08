// Copyright 2023 Canonical Ltd. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utilmocks

import (
	"fmt"
	exec "os/exec"

	gomock "github.com/golang/mock/gomock"
)

type eqCmdMatcher struct {
	x *exec.Cmd
}

func (e eqCmdMatcher) Matches(x interface{}) bool {
	xCmd, ok := x.(*exec.Cmd)
	if !ok {
		return false
	}
	if fmt.Sprintf("%s", e.x.Env) != fmt.Sprintf("%s", xCmd.Env) {
		return false
	}
	return e.x.String() == xCmd.String()
}

func (e eqCmdMatcher) String() string {
	return fmt.Sprintf("is equal to %v (env: %s)", e.x, e.x.Env)
}

// EqCmd returns a matcher that matches on equality of exec.Cmd
func EqCmd(x *exec.Cmd) gomock.Matcher {
	return eqCmdMatcher{x}
}
