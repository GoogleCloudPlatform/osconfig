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
	"io/ioutil"
	"log"
	"os/exec"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/osinfo"
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
	// GemExists indicates whether gem is installed.
	GemExists bool
	// PipExists indicates whether pip is installed.
	PipExists bool
	// GooGetExists indicates whether googet is installed.
	GooGetExists bool

	noarch = osinfo.Architecture("noarch")

	// DebugLogger is the debug logger to use.
	DebugLogger = log.New(ioutil.Discard, "", 0)

<<<<<<< HEAD
<<<<<<< HEAD
	// runner is the CommandRunner used for running exec commands.
	runner util.CommandRunner
=======
	// Runner is the CommandRunner used for running exec commands.
<<<<<<< HEAD
<<<<<<< HEAD
	Runner CommandRunner
>>>>>>> ca5d879... fix linter
=======
	Runner util.CommandRunner
>>>>>>> bae3585... implement feedback from reviewer
=======
	Runner CommandRunner
>>>>>>> d06ae33... Revert "implement feedback from reviewer"
)

func init() {
	runner = &packageCommandRunner{}
=======
	// runner is the CommandRunner used for running exec commands.
	runner CommandRunner
)

func init() {
	runner = &PackageCommandRunner{}
>>>>>>> 5345774... make runner non-public
}

// Packages is a selection of packages based on their manager.
type Packages struct {
	Yum           []PkgInfo     `json:"yum,omitempty"`
	Rpm           []PkgInfo     `json:"rpm,omitempty"`
	Apt           []PkgInfo     `json:"apt,omitempty"`
	Deb           []PkgInfo     `json:"deb,omitempty"`
	Zypper        []PkgInfo     `json:"zypper,omitempty"`
	ZypperPatches []ZypperPatch `json:"zypperPatches,omitempty"`
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
	SupportURL               string
	UpdateID                 string
	RevisionNumber           int32
	LastDeploymentChangeTime time.Time
}

// QFEPackage describes a Windows Quick Fix Engineering package.
type QFEPackage struct {
	Caption, Description, HotFixID, InstalledOn string
}

<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
// packageCommandRunner is the CommandRunner implementation used for running
// package manager commands.
type packageCommandRunner struct {
	runner util.CommandRunner
}

// RunCommand takes precreated exec.Cmd and returns the results of execution.
func (runner *packageCommandRunner) RunCommand(cmd *exec.Cmd) ([]byte, error) {
=======
=======
>>>>>>> d06ae33... Revert "implement feedback from reviewer"
// CommandRunner will execute the commands and return the results of that
// execution.
type CommandRunner interface {

	// RunCommand takes precreated exec.Cmd and returns the results of execution.
	RunCommand(command *exec.Cmd) ([]byte, error)

	// Run takes string arguments of command to be executed
	// and returns the results of execution.
	Run(command string, args ...string) ([]byte, error)

	// RunWithPty is a special case for RunCommand, except it runs with
	// pty instead of tty.
	RunWithPty(command *exec.Cmd) ([]byte, error)
}

<<<<<<< HEAD
=======
>>>>>>> bae3585... implement feedback from reviewer
=======
>>>>>>> d06ae33... Revert "implement feedback from reviewer"
// PackageCommandRunner is the CommandRunner implementation used for running
// package manager commands.
type PackageCommandRunner struct {
	runner CommandRunner
}

// RunCommand takes precreated exec.Cmd and returns the results of execution.
func (runner *PackageCommandRunner) RunCommand(cmd *exec.Cmd) ([]byte, error) {
>>>>>>> ca5d879... fix linter
	DebugLogger.Printf("Running %q with args %q\n", cmd.Path, cmd.Args[1:])
	return cmd.CombinedOutput()
}

// Run takes string arguments of command to be executed
// and returns the results of execution.
<<<<<<< HEAD
func (runner *packageCommandRunner) Run(arg string, args ...string) ([]byte, error) {
=======
func (runner *PackageCommandRunner) Run(arg string, args ...string) ([]byte, error) {
>>>>>>> ca5d879... fix linter
	return runner.RunCommand(exec.Command(arg, args...))
}

// RunWithPty is a special case for RunCommand, except it runs with
// pty instead of tty.
<<<<<<< HEAD
func (runner *packageCommandRunner) RunWithPty(cmd *exec.Cmd) ([]byte, error) {
=======
func (runner *PackageCommandRunner) RunWithPty(cmd *exec.Cmd) ([]byte, error) {
>>>>>>> ca5d879... fix linter
	return runWithPty(cmd)
}

var run = func(cmd *exec.Cmd) ([]byte, error) {
	DebugLogger.Printf("Running %q with args %q\n", cmd.Path, cmd.Args[1:])
	return cmd.CombinedOutput()
}
