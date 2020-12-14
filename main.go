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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/agentendpoint"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/policies"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/tarm/serial"

	_ "net/http/pprof"

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
	agentconfig.SetVersion(version)

	os.MkdirAll(filepath.Dir(agentconfig.RestartFile()), 0755)
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
	// Setup logging.
	opts := logger.LogOpts{LoggerName: "OSConfigAgent"}
	if agentconfig.Stdout() {
		opts.Writers = []io.Writer{os.Stdout}
	}
	if runtime.GOOS == "windows" {
		opts.Writers = append(opts.Writers, &serialPort{"COM1"})
	}

	// If this call to WatchConfig fails (like a metadata error) we can't continue.
	if err := agentconfig.WatchConfig(ctx); err != nil {
		logger.Init(ctx, opts)
		logger.Fatalf(err.Error())
	}
	opts.Debug = agentconfig.Debug()
	opts.ProjectName = agentconfig.ProjectID()

	if err := logger.Init(ctx, opts); err != nil {
		fmt.Printf("Error initializing logger: %v", err)
		os.Exit(1)
	}
	ctx = clog.WithLabels(ctx, map[string]string{"instance_name": agentconfig.Name()})

	// Remove any existing restart file.
	if err := os.Remove(agentconfig.RestartFile()); err != nil && !os.IsNotExist(err) {
		clog.Errorf(ctx, "Error removing restart signal file: %v", err)
	}

	deferredFuncs = append(deferredFuncs, logger.Close, func() { clog.Infof(ctx, "OSConfig Agent (version %s) shutting down.", agentconfig.Version()) })

	obtainLock()

	// obtainLock adds functions to clear the lock at close.
	logger.DeferredFatalFuncs = append(logger.DeferredFatalFuncs, deferredFuncs...)

	clog.Infof(ctx, "OSConfig Agent (version %s) started.", agentconfig.Version())

	// Call RegisterAgent on start then at least once every day.
	go func() {
		for {
			if agentconfig.TaskNotificationEnabled() || agentconfig.GuestPoliciesEnabled() {
				if client, err := agentendpoint.NewClient(ctx); err != nil {
					logger.Errorf(err.Error())
				} else if err := client.RegisterAgent(ctx); err != nil {
					logger.Errorf(err.Error())
				}
			}
			time.Sleep(24 * time.Hour)
		}
	}()

	switch action := flag.Arg(0); action {
	case "", "run", "noservice":
		runServiceLoop(ctx)
	case "inventory", "osinventory":
		client, err := agentendpoint.NewClient(ctx)
		if err != nil {
			logger.Fatalf(err.Error())
		}
		tasker.Enqueue(ctx, "Report OSInventory", func() {
			client.ReportInventory(ctx)
		})
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

func runTaskLoop(ctx context.Context, c chan struct{}) {
	var taskNotificationClient *agentendpoint.Client
	var err error
	for {
		if agentconfig.TaskNotificationEnabled() && (taskNotificationClient == nil || taskNotificationClient.Closed()) {
			// Start WaitForTaskNotification if we need to.
			taskNotificationClient, err = agentendpoint.NewClient(ctx)
			if err != nil {
				clog.Errorf(ctx, err.Error())
			} else {
				taskNotificationClient.WaitForTaskNotification(ctx)
			}
		} else if !agentconfig.TaskNotificationEnabled() && taskNotificationClient != nil && !taskNotificationClient.Closed() {
			// Cancel WaitForTaskNotification if we need to, this will block if there is
			// an existing current task running.
			if err := taskNotificationClient.Close(); err != nil {
				clog.Errorf(ctx, err.Error())
			}
		}

		// This is just to signal WaitForTaskNotification has run if needed.
		select {
		case c <- struct{}{}:
		default:
		}

		if err := agentconfig.WatchConfig(ctx); err != nil {
			clog.Errorf(ctx, err.Error())
		}
		select {
		case <-ctx.Done():
			return
		default:
			continue
		}
	}
}

func runServiceLoop(ctx context.Context) {
	// This is just to ensure WaitForTaskNotification runs before any periodocs.
	c := make(chan struct{})
	// Configures WaitForTaskNotification, waits for config changes with WatchConfig.
	go runTaskLoop(ctx, c)
	<-c

	// Runs functions that need to run on a set interval.
	ticker := time.NewTicker(agentconfig.SvcPollInterval())
	defer ticker.Stop()
	for {
		if _, err := os.Stat(agentconfig.RestartFile()); err == nil {
			clog.Infof(ctx, "Restart required marker file exists, beginning agent shutdown, waiting for tasks to complete.")
			tasker.Close()
			clog.Infof(ctx, "All tasks completed, stopping agent.")
			for _, f := range deferredFuncs {
				f()
			}
			os.Exit(2)
		}

		if agentconfig.GuestPoliciesEnabled() {
			policies.Run(ctx)
		}

		if agentconfig.OSInventoryEnabled() {
			// This should always run after ospackage.SetConfig.
			tasker.Enqueue(ctx, "Report OSInventory", func() {
				client, err := agentendpoint.NewClient(ctx)
				if err != nil {
					logger.Errorf(err.Error())
				}
				client.ReportInventory(ctx)
				client.Close()
			})
		}

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
	ctx = clog.WithLabels(ctx, map[string]string{"agent_version": agentconfig.Version()})
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
	// wuaupdates just runs the packages.WUAUpdates function and returns it's output
	// as JSON on stdout. This avoids memory issues with the WUA api since this is
	// called often for Windows inventory runs.
	case "wuaupdates":
		if err := wuaUpdates(flag.Arg(1)); err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	case "", "run":
		runService(ctx)
	default:
		run(ctx)
	}

	for _, f := range deferredFuncs {
		f()
	}
}
