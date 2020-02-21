// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ready provides functions to be passed as a "ready" function to the
// bundle main function.
package ready

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
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

	killOrphanAutotestd(log)

	// Disable the periodic log cleanup job to make sure system logs generated during tests are preserved.
	// We never resume the job so as to make it easier for users to inspect system logs later.
	if err := disableLogCleanup(); err != nil {
		return err
	}

	// If system-services doesn't enter "start/running", everything's probably broken, so give up.
	const systemServicesJob = "system-services"
	if err := upstart.WaitForJobStatus(ctx, systemServicesJob, upstart.StartGoal, upstart.RunningState,
		upstart.TolerateWrongGoal, 2*time.Minute); err != nil {
		return errors.Wrapf(err, "failed waiting for %v job", systemServicesJob)
	}

	// Make a best effort for important daemon jobs that start later, but just log errors instead of failing.
	// We don't want to abort the whole test run if there's a bug in a daemon that prevents it from starting.
	var daemonJobs = []string{
		"cryptohomed",
		"debugd",
		"metrics_daemon",
		"shill",
	}
	type jobError struct {
		job string
		err error
	}
	ch := make(chan *jobError)
	for _, job := range daemonJobs {
		go func(job string) {
			// Some Chrome-OS-derived systems may not have all of these jobs.
			if !upstart.JobExists(ctx, job) {
				ch <- nil
			} else if err := upstart.WaitForJobStatus(ctx, job, upstart.StartGoal, upstart.RunningState,
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

	if upstart.JobExists(ctx, "cryptohomed") {
		if err := waitForCryptohomeService(ctx, log); err != nil {
			log(fmt.Sprintf("Failed waiting for cryptohome D-Bus service: %v", err))
		} else {
			if hasTPM(log) {
				if err := ensureTPMInitialized(ctx, log); err != nil {
					log(fmt.Sprintf("Failed ensuring that TPM is initialized: %v", err))
				}
			} else {
				log("TPM not available, not waiting for readiness")
			}
		}
	}

	return nil
}

// isAutotestd returns whether p is an autotestd process.
func isAutotestd(p *process.Process) bool {
	cmd, err := p.CmdlineSlice()
	// Process is killed or arguments are unavailable (e.g. system processes).
	if err != nil || len(cmd) == 0 {
		return false
	}
	if filepath.Base(cmd[0]) == "autotestd" {
		return true
	}
	if filepath.Base(cmd[0]) == "python2.7" {
		for _, s := range cmd[1:] {
			if strings.HasPrefix(s, "-") {
				// Skip flags (if any).
				continue
			}
			return filepath.Base(s) == "autotestd"
		}
	}
	return false
}

// killOrphanAutotestd sends SIGKILL to running autotestd processes and their
// subprocesses. This works around the known issue that autotestd from timed out
// jobs interferes with Tast tests (crbug.com/874333, crbug.com/977035).
func killOrphanAutotestd(log func(string)) {
	ps, err := process.Processes()
	if err != nil {
		log(fmt.Sprint("Failed to enumerate processes: ", err))
		return
	}

	for _, p := range ps {
		if !isAutotestd(p) {
			continue
		}

		// Extract the process group ID of autotestd from ps output.
		// Unfortunately gopsutil does not support getting it.
		out, err := exec.Command("ps", "-o", "pgid=", strconv.Itoa(int(p.Pid))).Output()
		if err != nil {
			log(fmt.Sprint("ps command failed: ", err))
		}
		pgid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			log(fmt.Sprint("Failed to parse ps command output: ", err))
		}

		log(fmt.Sprintf("Killing orphan autotestd (pid=%d, pgid=%d)", p.Pid, pgid))
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			log(fmt.Sprint("Failed to kill autotestd: ", err))
		}
	}
}

// disableLogCleanup stops the periodic log cleanup job permanently.
func disableLogCleanup() error {
	if err := ioutil.WriteFile("/var/lib/cleanup_logs_paused", nil, 0666); err != nil {
		return errors.Wrap(err, "failed to disable the log cleanup job")
	}
	return nil
}

// hasTPM checks whether the DUT has a TPM.
func hasTPM(log func(string)) bool {
	const noTPMError = "Communication failure"

	out, err := exec.Command("tpm_version").CombinedOutput()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if !strings.Contains(string(out), noTPMError) {
				// Only log unexpected errors. Communication failure is expected on devices without TPMs.
				log(fmt.Sprintf("tpm_version exited with code %d: %s", exitError.ExitCode(), string(out)))
			}
		} else {
			log(fmt.Sprintf("failed to run tpm_version: %v", err))
		}

		return false
	}

	return true
}

// waitForCryptohomeService waits for cryptohomed's D-Bus service to become available.
func waitForCryptohomeService(ctx context.Context, log func(string)) error {
	const (
		svc        = "org.chromium.Cryptohome"
		svcTimeout = 15 * time.Second
		minUptime  = 10 * time.Second
	)

	bus, err := dbus.SystemBus()
	if err != nil {
		return errors.Wrap(err, "failed to connect to system bus")
	}

	wctx, wcancel := context.WithTimeout(ctx, svcTimeout)
	defer wcancel()
	if err = dbusutil.WaitForService(wctx, bus, svc); err != nil {
		return errors.Wrapf(err, "%s D-Bus service unavailable", svc)
	}

	// Make sure that we don't start using cryptohomed immediately after it registers its D-Bus service,
	// as that seems to sometimes cause it to hang: https://crbug.com/901363#c3, https://crbug.com/902199
	// In practice, this only matters on freshly-started VMs -- cryptohomed has typically been running for
	// a while on real hardware in the labs.
	uptime, err := getCryptohomedUptime()
	if err != nil {
		return errors.Wrap(err, "failed to get process uptime")
	}
	if uptime < minUptime {
		d := minUptime - uptime
		log(fmt.Sprintf("Waiting %v for cryptohomed to stabilize", d.Round(time.Millisecond)))
		if err := testing.Sleep(ctx, d); err != nil {
			return err
		}
	}

	return nil
}

// getCryptohomedUptime finds the cryptohomed process and returns how long it's been running.
func getCryptohomedUptime() (time.Duration, error) {
	procs, err := process.Processes()
	if err != nil {
		return 0, err
	}

	var proc *process.Process
	for _, p := range procs {
		if exe, err := p.Exe(); err == nil && filepath.Base(exe) == "cryptohomed" {
			proc = p
			break
		}
	}
	if proc == nil {
		return 0, errors.New("didn't find process")
	}

	ms, err := proc.CreateTime() // milliseconds since epoch
	if err != nil {
		return 0, errors.Wrap(err, "failed to get start time")
	}
	d := time.Duration(ms) * time.Millisecond
	ct := time.Unix(int64(d/time.Second), int64((d%time.Second)/time.Nanosecond))
	return time.Since(ct), nil
}

// These match lines in the output from "cryptohome --action=tpm_more_status".
var tpmEnabledRegexp = regexp.MustCompile(`(?m)^\s*enabled:\s*true\s*$`)
var tpmInitializedRegexp = regexp.MustCompile(`(?m)^\s*attestation_prepared:\s*true\s*$`)

// ensureTPMInitialized checks if the TPM is already initialized and tries to take ownership if not.
// nil is returned if the TPM is not enabled (as is the case on VMs).
func ensureTPMInitialized(ctx context.Context, log func(string)) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	tpmStatus := func(ctx context.Context) (enabled, initialized bool, err error) {
		out, err := testexec.CommandContext(ctx, "cryptohome", "--action=tpm_more_status").Output()
		if err != nil {
			return false, false, err
		}
		return tpmEnabledRegexp.Match(out), tpmInitializedRegexp.Match(out), nil
	}

	// Check if the TPM is disabled or already initialized.
	if enabled, initialized, err := tpmStatus(ctx); err != nil {
		return err
	} else if !enabled || initialized {
		return nil
	}

	log("TPM not initialized; taking ownership now to ensure that tests aren't blocked during login")
	if err := testexec.CommandContext(ctx, "cryptohome", "--action=tpm_take_ownership").Run(); err != nil {
		return err
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, initialized, err := tpmStatus(ctx); err != nil {
			// cryptohome error encountered while polling.
			return testing.PollBreak(err)
		} else if !initialized {
			return errors.New("TPM not initialized yet")
		}
		return nil
	}, nil)
}
