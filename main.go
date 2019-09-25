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

// osconfig_agent interacts with the osconfig api.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/inventory"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
	"github.com/GoogleCloudPlatform/osconfig/ospatch"
	"github.com/GoogleCloudPlatform/osconfig/policies"
	"github.com/GoogleCloudPlatform/osconfig/service"
	"github.com/GoogleCloudPlatform/osconfig/tasker"

	_ "google.golang.org/genproto/googleapis/rpc/errdetails"
)

var version string

func init() {
	// We do this here so the -X value doesn't need the full path.
	config.SetVersion(version)
}

type logWriter struct{}

func (l *logWriter) Write(b []byte) (int, error) {
	logger.Log(logger.LogEntry{Message: string(b), Severity: logger.Debug})
	return len(b), nil
}

func run(ctx context.Context) {
	ticker := time.NewTicker(config.SvcPollInterval())
	for {
		if err := config.SetConfig(); err != nil {
			logger.Errorf(err.Error())
		}

		if _, err := os.Stat(config.RestartFile()); err == nil {
			logger.Infof("Restart required marker file exists, beginning agent shutdown, waiting for tasks to complete.")
			tasker.Close()
			logger.Infof("All tasks completed, stopping agent.")
			return
		}

		// This sets up the patching system to run in the background.
		ospatch.Configure(ctx)

		if config.OSPackageEnabled() {
			policies.Run(ctx, config.Instance())
		}

		if config.OSInventoryEnabled() {
			// This should always run after ospackage.SetConfig.
			inventory.Run()
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}

var deferredFuncs []func()

func main() {
	flag.Parse()
	ctx := context.Background()
	if err := os.Remove(config.RestartFile()); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Error removing restart signal file: %v", err)
	}

	// If this call to SetConfig fails (like a metadata error) we can't continue.
	if err := config.SetConfig(); err != nil {
		logger.Init(ctx, logger.LogOpts{LoggerName: "OSConfigAgent", ProjectName: config.ProjectID()})
		logger.Fatalf(err.Error())
	}
	logger.Init(ctx, logger.LogOpts{LoggerName: "OSConfigAgent", ProjectName: config.ProjectID(), Debug: config.Debug(), Stdout: config.Stdout()})
	defer logger.Close()

	obtainLock()

	logger.DeferredFatalFuncs = append(logger.DeferredFatalFuncs, deferredFuncs...)
	defer func() {
		for _, f := range deferredFuncs {
			f()
		}
	}()

	packages.DebugLogger = log.New(&logWriter{}, "", 0)

	logger.Infof("OSConfig Agent (version %s) Started", config.Version())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		select {
		case <-c:
			logger.Fatalf("Ctrl-C caught, shutting down.")
		}
	}()

	switch action := flag.Arg(0); action {
	case "", "run":
		if err := service.Register(ctx, "google_osconfig_agent", "Google OSConfig Agent", "", run, "run"); err != nil {
			logger.Fatalf("service.Register error: %v", err)
		}
		return
	case "noservice":
		run(ctx)
		return
	case "inventory", "osinventory":
		inventory.Run()
		tasker.Close()
		return
	case "policies", "guestpolicies", "ospackage":
		policies.Run(ctx, config.Instance())
		tasker.Close()
		return
	case "ospatch":
		ospatch.Run(ctx, make(chan struct{}))
		return
	default:
		logger.Fatalf("Unknown arg %q", action)
	}
}
