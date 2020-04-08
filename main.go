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
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/agentendpoint"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/inventory"
	"github.com/GoogleCloudPlatform/osconfig/inventory/packages"
	"github.com/GoogleCloudPlatform/osconfig/ospatch"
	"github.com/GoogleCloudPlatform/osconfig/policies"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/tarm/serial"

	_ "google.golang.org/genproto/googleapis/rpc/errdetails"
)

var (
	version string
	profile = flag.Bool("profile", false, "serve profiling data at localhost:6060/debug/pprof")
)

func init() {
	if version == "" {
		version = "manual-" + time.Now().Format(time.RFC3339)
	}
	// We do this here so the -X value doesn't need the full path.
	config.SetVersion(version)

	os.MkdirAll(filepath.Dir(config.RestartFile()), 0755)
}

type logWriter struct{}

func (l *logWriter) Write(b []byte) (int, error) {
	logger.Log(logger.LogEntry{Message: string(b), Severity: logger.Debug})
	return len(b), nil
}

type serialPort struct {
	port string
}

func (s *serialPort) Write(b []byte) (int, error) {
	c := &serial.Config{Name: s.port, Baud: 115200}
	p, err := serial.OpenPort(c)
	if err != nil {
		return 0, err
	}
	defer p.Close()

	return p.Write(b)
}

var deferredFuncs []func()

func run(ctx context.Context) {
	// Remove any existing restart file.
	if err := os.Remove(config.RestartFile()); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Error removing restart signal file: %v", err)
	}

	// Setup logging.
	opts := logger.LogOpts{LoggerName: "OSConfigAgent"}
	if config.Stdout() {
		opts.Writers = []io.Writer{os.Stdout}
	}
	if runtime.GOOS == "windows" {
		opts.Writers = append(opts.Writers, &serialPort{"COM1"})
	}

	// If this call to SetConfig fails (like a metadata error) we can't continue.
	if err := config.SetConfig(ctx); err != nil {
		logger.Init(ctx, opts)
		logger.Fatalf(err.Error())
	}
	opts.Debug = config.Debug()
	opts.ProjectName = config.ProjectID()

	if err := logger.Init(ctx, opts); err != nil {
		fmt.Printf("Error initializing logger: %v", err)
		os.Exit(1)
	}
	packages.DebugLogger = log.New(&logWriter{}, "", 0)

	deferredFuncs = append(deferredFuncs, logger.Close, func() { logger.Infof("OSConfig Agent (version %s) shutting down.", config.Version()) })

	obtainLock()

	// obtainLock adds functions to clear the lock at close.
	logger.DeferredFatalFuncs = append(logger.DeferredFatalFuncs, deferredFuncs...)
	defer func() {
		for _, f := range deferredFuncs {
			f()
		}
	}()

	logger.Infof("OSConfig Agent (version %s) started.", config.Version())

	switch action := flag.Arg(0); action {
	case "", "run", "noservice":
		runLoop(ctx)
	case "inventory", "osinventory":
		inventory.Run()
		tasker.Close()
		return
	case "gp", "policies", "guestpolicies", "ospackage":
		policies.Run(ctx)
		tasker.Close()
		return
	case "w", "waitfortasknotification", "ospatch":
		client, err := agentendpoint.NewClient(ctx)
		if err != nil {
			logger.Fatalf(err.Error())
		}
		client.WaitForTaskNotification(ctx)
		select {
		case <-ctx.Done():
		}
	default:
		logger.Fatalf("Unknown arg %q", action)
	}
}

func runLoop(ctx context.Context) {
	var taskNotificationClient *agentendpoint.Client
	var err error
	ticker := time.NewTicker(config.SvcPollInterval())
	for {
		if err := config.SetConfig(ctx); err != nil {
			logger.Errorf(err.Error())
		}

		if _, err := os.Stat(config.RestartFile()); err == nil {
			logger.Infof("Restart required marker file exists, beginning agent shutdown, waiting for tasks to complete.")
			tasker.Close()
			logger.Infof("All tasks completed, stopping agent.")
			for _, f := range deferredFuncs {
				f()
			}
			os.Exit(2)
		}

		if config.TaskNotificationEnabled() && (taskNotificationClient == nil || taskNotificationClient.Closed()) {
			ospatch.DisableAutoUpdates()

			// Start WaitForTaskNotification if we need to.
			taskNotificationClient, err = agentendpoint.NewClient(ctx)
			if err != nil {
				logger.Errorf(err.Error())
			} else {
				taskNotificationClient.WaitForTaskNotification(ctx)
			}
		} else if !config.TaskNotificationEnabled() && taskNotificationClient != nil && !taskNotificationClient.Closed() {
			// Cancel WaitForTaskNotification if we need to, this will block if there is
			// an existing current task running.
			if err := taskNotificationClient.Close(); err != nil {
				logger.Errorf(err.Error())
			}
		}

		if config.GuestPoliciesEnabled() {
			policies.Run(ctx)
		}

		if config.OSInventoryEnabled() {
			// This should always run after ospackage.SetConfig.
			inventory.Run()
		}

		// Return unused memory to ensure our footprint doesn't keep increasing.
		logger.Debugf("Running debug.FreeOSMemory()")
		debug.FreeOSMemory()

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	flag.Parse()
	ctx, cncl := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-c:
			cncl()
		}
	}()

	if *profile {
		go func() {
			fmt.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	switch action := flag.Arg(0); action {
	case "", "run":
		runService(ctx)
	default:
		run(ctx)
	}
}
