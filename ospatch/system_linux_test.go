//  Copyright 2024 Google Inc. All Rights Reserved.
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

package ospatch

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestDisableYumCronSystemd(t *testing.T) {
	ctx := context.Background()
	stopErr := errors.New("stop failed")
	disableErr := errors.New("disable failed")

	tests := []struct {
		desc      string
		responses map[string]cmdResponse
		wantErr   error
	}{
		{
			desc: "already disabled (exit code 1)",
			responses: map[string]cmdResponse{
				"/bin/systemctl is-enabled yum-cron.service": {err: makeExitError(1)},
			},
		},
		{
			desc: "enabled then stop and disable succeed",
			responses: map[string]cmdResponse{
				"/bin/systemctl is-enabled yum-cron.service": {out: []byte("enabled\n")},
				"/bin/systemctl stop yum-cron.service":       {},
				"/bin/systemctl disable yum-cron.service":    {},
			},
		},
		{
			desc: "check fails",
			responses: map[string]cmdResponse{
				"/bin/systemctl is-enabled yum-cron.service": {err: errors.New("check failed")},
			},
			wantErr: errors.New("error checking status of yum-cron: check failed"),
		},
		{
			desc: "stop fails",
			responses: map[string]cmdResponse{
				"/bin/systemctl is-enabled yum-cron.service": {out: []byte("enabled\n")},
				"/bin/systemctl stop yum-cron.service":       {err: stopErr},
			},
			wantErr: errors.New("error stopping yum-cron: stop failed"),
		},
		{
			desc: "disable fails",
			responses: map[string]cmdResponse{
				"/bin/systemctl is-enabled yum-cron.service": {out: []byte("enabled\n")},
				"/bin/systemctl stop yum-cron.service":       {},
				"/bin/systemctl disable yum-cron.service":    {err: disableErr},
			},
			wantErr: errors.New("error disabling yum-cron: disable failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockRun(t, tt.responses)
			err := disableYumCronSystemd(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestDisableYumCronChkconfig(t *testing.T) {
	ctx := context.Background()
	checkErr := errors.New("chkconfig failed")

	tests := []struct {
		desc      string
		responses map[string]cmdResponse
		wantErr   error
	}{
		{
			desc: "already disabled",
			responses: map[string]cmdResponse{
				"/sbin/chkconfig yum-cron": {out: []byte("yum-cron disabled")},
			},
		},
		{
			desc: "enabled then disable succeeds",
			responses: map[string]cmdResponse{
				"/sbin/chkconfig yum-cron":     {out: []byte("yum-cron enabled")},
				"/sbin/chkconfig yum-cron off": {},
			},
		},
		{
			desc: "check fails",
			responses: map[string]cmdResponse{
				"/sbin/chkconfig yum-cron": {err: checkErr},
			},
			wantErr: errors.New("error checking status of yum-cron: chkconfig failed"),
		},
		{
			desc: "disable fails",
			responses: map[string]cmdResponse{
				"/sbin/chkconfig yum-cron":     {out: []byte("yum-cron enabled")},
				"/sbin/chkconfig yum-cron off": {err: errors.New("off failed")},
			},
			wantErr: errors.New("error disabling yum-cron: off failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockRun(t, tt.responses)
			err := disableYumCronChkconfig(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestDisableDnfAutomatic(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		desc      string
		responses map[string]cmdResponse
		wantErr   error
	}{
		{
			desc: "no timers listed",
			responses: map[string]cmdResponse{
				"/bin/systemctl list-timers dnf-automatic.timer": {out: []byte("0 timers listed")},
			},
		},
		{
			desc: "timer active then stop and disable succeed",
			responses: map[string]cmdResponse{
				"/bin/systemctl list-timers dnf-automatic.timer": {out: []byte("1 timers listed")},
				"/bin/systemctl stop dnf-automatic.timer":        {},
				"/bin/systemctl disable dnf-automatic.timer":     {},
			},
		},
		{
			desc: "list-timers fails",
			responses: map[string]cmdResponse{
				"/bin/systemctl list-timers dnf-automatic.timer": {err: errors.New("list failed")},
			},
			wantErr: errors.New("error checking status of dnf-automatic: list failed"),
		},
		{
			desc: "stop fails",
			responses: map[string]cmdResponse{
				"/bin/systemctl list-timers dnf-automatic.timer": {out: []byte("1 timers listed")},
				"/bin/systemctl stop dnf-automatic.timer":        {err: errors.New("stop failed")},
			},
			wantErr: errors.New("error stopping dnf-automatic: stop failed"),
		},
		{
			desc: "disable fails",
			responses: map[string]cmdResponse{
				"/bin/systemctl list-timers dnf-automatic.timer": {out: []byte("1 timers listed")},
				"/bin/systemctl stop dnf-automatic.timer":        {},
				"/bin/systemctl disable dnf-automatic.timer":     {err: errors.New("disable failed")},
			},
			wantErr: errors.New("error disabling dnf-automatic: disable failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockRun(t, tt.responses)
			err := disableDnfAutomatic(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestDisableUnattendedUpgrades(t *testing.T) {
	ctx := context.Background()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		desc    string
		err     error
		wantErr error
	}{
		{
			desc: "success",
		},
		{
			desc:    "failure",
			err:     errors.New("apt-get failed"),
			wantErr: errors.New("error running /usr/bin/apt-get with args [\"remove\" \"-y\" \"unattended-upgrades\"]: apt-get failed, stdout: \"stdout\", stderr: \"stderr\""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
			packages.SetCommandRunner(mockCommandRunner)
			mockCommandRunner.EXPECT().Run(ctx, gomock.Any()).Return([]byte("stdout"), []byte("stderr"), tt.err).Times(1)

			err := disableUnattendedUpgrades(ctx)

			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

func TestDisableAutoUpdates(t *testing.T) {
	ctx := context.Background()

	origYumCronServicePath := yumCronServicePath
	origYumCronBinPath := yumCronBinPath
	origDnfAutomaticPath := dnfAutomaticPath
	origUnattendedUpgPath := unattendedUpgPath
	t.Cleanup(func() {
		yumCronServicePath = origYumCronServicePath
		yumCronBinPath = origYumCronBinPath
		dnfAutomaticPath = origDnfAutomaticPath
		unattendedUpgPath = origUnattendedUpgPath
	})

	yumCronServicePath = "/invalid/path/yum-cron.service"
	yumCronBinPath = "/invalid/path/yum-cron"
	dnfAutomaticPath = "/invalid/path/dnf-automatic.timer"
	unattendedUpgPath = "/invalid/path/unattended-upgrades"

	// Call DisableAutoUpdates and verify it doesn't crash/panic.
	DisableAutoUpdates(ctx)
}

// mockRun replaces run for tests and records calls.
func mockRun(t *testing.T, responses map[string]cmdResponse) {
	t.Helper()
	original := run
	t.Cleanup(func() { run = original })

	run = func(ctx context.Context, name string, args []string) ([]byte, error) {
		key := name
		for _, a := range args {
			key += " " + a
		}
		if resp, ok := responses[key]; ok {
			return resp.out, resp.err
		}
		t.Fatalf("unexpected command: %s", key)
		return nil, nil
	}
}

type cmdResponse struct {
	out []byte
	err error
}

func makeExitError(code int) error {
	return exec.Command("/bin/sh", "-c", "exit "+string(rune('0'+code))).Run()
}
