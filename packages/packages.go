/*
Copyright 2017 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package packages provides package management functions for Windows and Linux
// systems.
package packages

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

var (
	// AptExists indicates whether apt is installed.
	AptExists bool
	// DpkgExists indicates whether dpkg is installed.
	DpkgExists bool
	// DpkgQueryExists indicates whether dpkg-query is installed.
	DpkgQueryExists bool
	// YumExists indicates whether yum is installed.
	YumExists bool
	// ZypperExists indicates whether zypper is installed.
	ZypperExists bool
	// RPMExists indicates whether rpm is installed.
	RPMExists bool
	// RPMQueryExists indicates whether rpmquery is installed.
	RPMQueryExists bool
	// COSPkgInfoExists indicates whether COS package information is available.
	COSPkgInfoExists bool
	// GemExists indicates whether gem is installed.
	GemExists bool
	// PipExists indicates whether pip is installed.
	PipExists bool
	// GooGetExists indicates whether googet is installed.
	GooGetExists bool
	// MSIExists indicates whether MSIs can be installed.
	MSIExists bool

	noarch = osinfo.Architecture("noarch")

	runner = util.CommandRunner(&util.DefaultRunner{})

	ptyrunner = util.CommandRunner(&ptyRunner{})
)

// Packages is a selection of packages based on their manager.
type Packages struct {
	Yum           []PkgInfo     `json:"yum,omitempty"`
	Rpm           []PkgInfo     `json:"rpm,omitempty"`
	Apt           []PkgInfo     `json:"apt,omitempty"`
	Deb           []PkgInfo     `json:"deb,omitempty"`
	Zypper        []PkgInfo     `json:"zypper,omitempty"`
	ZypperPatches []ZypperPatch `json:"zypperPatches,omitempty"`
	COS           []PkgInfo     `json:"cos,omitempty"`
	Gem           []PkgInfo     `json:"gem,omitempty"`
	Pip           []PkgInfo     `json:"pip,omitempty"`
	GooGet        []PkgInfo     `json:"googet,omitempty"`
	WUA           []WUAPackage  `json:"wua,omitempty"`
	QFE           []QFEPackage  `json:"qfe,omitempty"`
}

// PkgInfo describes a package.
type PkgInfo struct {
	Name, Arch, Version string
}

// ZypperPatch describes a Zypper patch.
type ZypperPatch struct {
	Name, Category, Severity, Summary string
}

// WUAPackage describes a Windows Update Agent package.
type WUAPackage struct {
	Title                    string
	Description              string
	Categories               []string
	CategoryIDs              []string
	KBArticleIDs             []string
	MoreInfoURLs             []string
	SupportURL               string
	UpdateID                 string
	RevisionNumber           int32
	LastDeploymentChangeTime time.Time
}

// QFEPackage describes a Windows Quick Fix Engineering package.
type QFEPackage struct {
	Caption, Description, HotFixID, InstalledOn string
}

func run(ctx context.Context, cmd string, args []string) ([]byte, error) {
	stdout, stderr, err := runner.Run(ctx, exec.Command(cmd, args...))
	if err != nil {
		return nil, fmt.Errorf("error running %s with args %q: %v, stdout: %q, stderr: %q", cmd, args, err, stdout, stderr)
	}
	return stdout, nil
}

type ptyRunner struct{}

func (p *ptyRunner) Run(ctx context.Context, cmd *exec.Cmd) ([]byte, []byte, error) {
	clog.Debugf(ctx, "Running %q with args %q\n", cmd.Path, cmd.Args[1:])
	stdout, stderr, err := runWithPty(cmd)
	clog.Debugf(ctx, "%s %q output:\n%s", cmd.Path, cmd.Args[1:], strings.ReplaceAll(string(stdout), "\n", "\n "))
	return stdout, stderr, err
}

// SetCommandRunner allows external clients to set a custom commandRunner.
func SetCommandRunner(commandRunner util.CommandRunner) {
	runner = commandRunner
}

// SetPtyCommandRunner allows external clients to set a custom
// custom commandRunner.
func SetPtyCommandRunner(commandRunner util.CommandRunner) {
	ptyrunner = commandRunner
}
