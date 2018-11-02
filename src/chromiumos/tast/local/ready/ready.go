// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ready provides functions to be passed as a "ready" function to the
// bundle main function.
package ready

import (
	"context"
	"fmt"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
)

// Wait waits until the system is (marginally) ready for tests to run.
// Tast can sometimes be run against a freshly-booted VM, and we don't want every test that
// depends on a critical daemon to need to call upstart.WaitForJobStatus to wait for the
// corresponding job to be running. See https://crbug.com/897521 for more details.
func Wait(ctx context.Context, log func(string)) error {
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
		"cryptohomed",    // "start on started boot-services and started tcsd and started chapsd"
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

	if err := waitForCryptohomeService(ctx); err != nil {
		log(fmt.Sprintf("Failed waiting for cryptohome D-Bus service: %v", err))
	}

	return nil
}

// waitForCryptohomeService waits for cryptohomed's D-Bus service to become available.
func waitForCryptohomeService(ctx context.Context) error {
	const (
		svc     = "org.chromium.Cryptohome"
		timeout = 15 * time.Second
	)

	bus, err := dbus.SystemBus()
	if err != nil {
		return errors.Wrap(err, "failed to connect to system bus")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err = dbusutil.WaitForService(ctx, bus, svc); err != nil {
		return errors.Wrapf(err, "%s D-Bus service unavailable", svc)
	}
	return nil
}
