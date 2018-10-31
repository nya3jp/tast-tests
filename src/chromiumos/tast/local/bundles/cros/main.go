// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "cros" local test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	// Underscore-imported packages register their tests via init functions.
	"chromiumos/tast/bundle"
	"chromiumos/tast/errors"
	_ "chromiumos/tast/local/bundles/cros/arc"
	_ "chromiumos/tast/local/bundles/cros/arcapp"
	_ "chromiumos/tast/local/bundles/cros/audio"
	_ "chromiumos/tast/local/bundles/cros/cryptohome"
	_ "chromiumos/tast/local/bundles/cros/debugd"
	_ "chromiumos/tast/local/bundles/cros/example"
	_ "chromiumos/tast/local/bundles/cros/graphics"
	_ "chromiumos/tast/local/bundles/cros/meta"
	_ "chromiumos/tast/local/bundles/cros/network"
	_ "chromiumos/tast/local/bundles/cros/platform"
	_ "chromiumos/tast/local/bundles/cros/power"
	_ "chromiumos/tast/local/bundles/cros/printer"
	_ "chromiumos/tast/local/bundles/cros/security"
	_ "chromiumos/tast/local/bundles/cros/session"
	_ "chromiumos/tast/local/bundles/cros/ui"
	_ "chromiumos/tast/local/bundles/cros/video"
	_ "chromiumos/tast/local/bundles/cros/vm"
	"chromiumos/tast/local/upstart"
)

func main() {
	os.Exit(bundle.Local(os.Stdin, os.Stdout, os.Stderr, waitUntilReady))
}

// waitUntilReady waits until the system is (marginally) ready for tests to run.
// Tast can sometimes be run against a freshly-booted VM, and we don't want every test that
// depends on a critical daemon to need to call upstart.WaitForJobStatus to wait for the
// corresponding job to be running. See https://crbug.com/897521 for more details.
func waitUntilReady(ctx context.Context, log func(string)) error {
	// Periodically log a message to make it clearer what we're doing.
	// Sending a periodic control message is also needed to let the main tast process
	// know that the DUT is still responsive.
	done := make(chan struct{})
	defer func() { done <- struct{}{} }()
	go func() {
		for {
			select {
			case <-time.After(30 * time.Second):
				log("Still waiting for important system services to be running")
			case <-done:
				return
			}
		}
	}()

	// If system-services doesn't enter "start/running", everything's probably broken, so give up.
	const systemServicesJob = "system-services"
	if err := upstart.WaitForJobStatus(ctx, systemServicesJob, upstart.StartGoal, upstart.RunningState,
		upstart.TolerateWrongGoal, 2*time.Minute); err != nil {
		return errors.Wrapf(err, "failed waiting for %v job", systemServicesJob)
	}

	// Make a best effort for important daemon jobs that start later, but just log errors instead of failing.
	// We don't want to abort the whole test run if there's a bug in a daemon that prevents it from starting.
	var daemonJobs = []string{
		"debugd",         // "start on started ui"
		"metrics_daemon", // "start on stopped crash-boot-collect"
		"shill",          // "start on started network-services and started wpasupplicant"
	}
	type jobError struct {
		job string
		err error
	}
	ch := make(chan *jobError)
	for _, job := range daemonJobs {
		go func(job string) {
			if err := upstart.WaitForJobStatus(ctx, job, upstart.StartGoal, upstart.RunningState,
				upstart.TolerateWrongGoal, time.Minute); err == nil {
				ch <- nil
			} else {
				ch <- &jobError{job, err}
			}
		}(job)
	}
	for range daemonJobs {
		if je := <-ch; je != nil {
			log(fmt.Sprintf("Failed waiting for job %v: %v", je.job, je.err))
		}
	}

	return nil
}
