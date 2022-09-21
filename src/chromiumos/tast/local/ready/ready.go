// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ready provides functions to be passed as a "ready" function to the
// bundle main function.
package ready

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Wait waits until the system is (marginally) ready for tests to run.
// Tast can sometimes be run against a freshly-booted VM, and we don't want every test that
// depends on a critical daemon to need to call upstart.WaitForJobStatus to wait for the
// corresponding job to be running. See https://crbug.com/897521 for more details.
func Wait(ctx context.Context, systemServicesTimeout time.Duration) error {
	// Periodically log a message to make it clearer what we're doing.
	// Sending a periodic control message is also needed to let the main tast process
	// know that the DUT is still responsive.
	done := make(chan struct{})
	defer func() { done <- struct{}{} }()
	go func() {
		for {
			select {
			case <-time.After(30 * time.Second):
				testing.ContextLog(ctx, "Still waiting for important system services to be running")
			case <-done:
				return
			}
		}
	}()

	ourExe, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable name")
	}
	killMatchingProcesses(ctx, func(ctx context.Context, p *process.Process) bool {
		// Kill orphan autotestd processes and their subprocesses.
		// This works around the known issue that autotestd from timed out
		// jobs interferes with Tast tests (crbug.com/874333, crbug.com/977035).
		if isAutotestd(p) {
			return true
		}
		// Kill any existing test bundle processes. This can help prevent
		// confusing failures if multiple test jobs are incorrectly scheduled
		// on the same DUT (b/187787103).
		if isBundle(ctx, p, ourExe) {
			return true
		}
		return false
	})
	clearPolicies(ctx)

	// Delete all core dumps to free up spaces.
	if err := crash.DeleteCoreDumps(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to delete core dumps: ", err)
	}

	// If system-services doesn't enter "start/running", everything's probably broken, so give up.
	const systemServicesJob = "system-services"
	if err := upstart.WaitForJobStatus(ctx, systemServicesJob, upstartcommon.StartGoal, upstartcommon.RunningState,
		upstart.TolerateWrongGoal, systemServicesTimeout); err != nil {
		return errors.Wrapf(err, "failed waiting for %v job after %f seconds", systemServicesJob, systemServicesTimeout.Seconds())
	}

	// Start the ui job if it is not running. When the ui job is stopped, some
	// daemons (e.g. debugd) are also stopped, so it is not useful to wait for
	// them without starting the ui job.
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to start ui")
	}

	// Make a best effort for important daemon jobs that start later, but just log errors instead of failing.
	// We don't want to abort the whole test run if there's a bug in a daemon that prevents it from starting.
	var daemonJobs = []string{
		"attestationd",
		"cryptohomed",
		"debugd",
		"metrics_daemon",
		"shill",
		"tpm_managerd",
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
			} else if err := upstart.WaitForJobStatus(ctx, job, upstartcommon.StartGoal, upstartcommon.RunningState,
				upstart.TolerateWrongGoal, time.Minute); err == nil {
				ch <- nil
			} else {
				ch <- &jobError{job, err}
			}
		}(job)
	}
	for range daemonJobs {
		if je := <-ch; je != nil {
			testing.ContextLogf(ctx, "Failed waiting for job %v: %v", je.job, je.err)
		}
	}

	if upstart.JobExists(ctx, "cryptohomed") {
		if err := waitForCryptohomeService(ctx); err != nil {
			testing.ContextLog(ctx, "Failed waiting for cryptohome D-Bus service: ", err)
		} else {
			if hasTPM(ctx) {
				if err := ensureTPMInitialized(ctx); err != nil {
					testing.ContextLog(ctx, "Failed ensuring that TPM is initialized: ", err)
				}
				checkEnterpriseOwned(ctx)
			} else {
				testing.ContextLog(ctx, "TPM not available, not waiting for readiness")
			}
		}
	}
	if err := hwsec.BackupTPMManagerDataIfIntact(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to backup tpm manager local data: ", err)
	}

	return nil
}

// killMatchingProcesses sends SIGKILL to matching processes and their
// subprocesses. Current process is not killed.
func killMatchingProcesses(ctx context.Context, match func(ctx context.Context, p *process.Process) bool) {
	ps, err := process.Processes()
	if err != nil {
		testing.ContextLog(ctx, "Failed to enumerate processes: ", err)
		return
	}

	ourPID := os.Getpid()
	for _, p := range ps {
		if int(p.Pid) == ourPID {
			continue
		}
		if !match(ctx, p) {
			continue
		}

		// Extract the process group ID from ps output.
		// Unfortunately gopsutil does not support getting it.
		out, err := exec.Command("ps", "-o", "pgid=", strconv.Itoa(int(p.Pid))).Output()
		if err != nil {
			testing.ContextLog(ctx, "ps command failed: ", err)
		}
		pgid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			testing.ContextLog(ctx, "Failed to parse ps command output: ", err)
		}

		testing.ContextLogf(ctx, "Killing orphan process (pid=%d, pgid=%d)", p.Pid, pgid)
		if err := unix.Kill(-pgid, unix.SIGKILL); err != nil {
			testing.ContextLog(ctx, "Failed to kill process: ", err)
		}
	}
}

// isBundle returns whether p is a bundle process. ourExe should be current
// bundle executable's path.
func isBundle(ctx context.Context, p *process.Process, ourExe string) bool {
	exeDir := filepath.Dir(ourExe)
	if exe, err := p.Exe(); err == nil && filepath.Dir(exe) == exeDir {
		return true
	}
	return false
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

// hasTPM checks whether the DUT has a TPM.
func hasTPM(ctx context.Context) bool {
	const noTPMError = "Communication failure"

	out, err := exec.Command("tpm_version").CombinedOutput()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if !strings.Contains(string(out), noTPMError) {
				// Only log unexpected errors. Communication failure is expected on devices without TPMs.
				testing.ContextLogf(ctx, "tpm_version exited with code %d: %s", exitError.ExitCode(), string(out))
			}
		} else {
			testing.ContextLog(ctx, "Failed to run tpm_version: ", err)
		}

		return false
	}

	return true
}

// waitForCryptohomeService waits for cryptohomed's D-Bus service to become available.
func waitForCryptohomeService(ctx context.Context) error {
	const (
		svc        = "org.chromium.UserDataAuth"
		svcTimeout = 15 * time.Second
		minUptime  = 10 * time.Second
	)

	bus, err := dbusutil.SystemBus()
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
		testing.ContextLogf(ctx, "Waiting %v for cryptohomed to stabilize", d.Round(time.Millisecond))
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

// These match lines in the output from "tpm_manager_client status --nonsensitive".
var tpmEnabledRegexp = regexp.MustCompile(`(?m)^\s*is_enabled:\s*true\s*$`)
var tpmOwnedRegexp = regexp.MustCompile(`(?m)^\s*is_owned:\s*true\s*$`)

// These match lines in the output from "tpm_manager_client get_supported_features".
var tpmSupportRuntimeSelectionRegexp = regexp.MustCompile(`(?m)^\s*support_runtime_selection:\s*true\s*$`)

// These match lines in the output from "attestation_client status".
var tpmInitializedRegexp = regexp.MustCompile(`(?m)^\s*prepared_for_enrollment:\s*true\s*$`)

// ensureTPMInitialized checks if the TPM is already initialized and tries to take ownership if not.
// nil is returned if the TPM is not enabled (as is the case on VMs).
func ensureTPMInitialized(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	tpmStatus := func(ctx context.Context) (enabled, initialized, owned, runtimeSelect bool, err error) {
		tpmOut, err := testexec.CommandContext(ctx, "tpm_manager_client", "status", "--nonsensitive").Output()
		if err != nil {
			return false, false, false, false, errors.Wrap(err, "tpm_manager_client status failed")
		}
		tpmFeaturesOut, err := testexec.CommandContext(ctx, "tpm_manager_client", "get_supported_features").Output()
		if err != nil {
			return false, false, false, false, errors.Wrap(err, "tpm_manager_client get_supported_features failed")
		}
		attestOut, err := testexec.CommandContext(ctx, "attestation_client", "status").Output()
		if err != nil {
			return false, false, false, false, errors.Wrap(err, "attestation_client status failed")
		}
		return tpmEnabledRegexp.Match(tpmOut), tpmInitializedRegexp.Match(attestOut), tpmOwnedRegexp.Match(tpmOut), tpmSupportRuntimeSelectionRegexp.Match(tpmFeaturesOut), nil
	}

	// Check if the TPM is disabled or already initialized.
	enabled, initialized, owned, runtimeSelect, err := tpmStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to obtain TPM status")
	} else if !enabled || (!runtimeSelect && initialized) {
		return nil
	}

	testing.ContextLog(ctx, "TPM not initialized; taking ownership now to ensure that tests aren't blocked during login")
	if owned {
		testing.ContextLog(ctx, "TPM is already owned; finishing initialization")
	}
	if err := testexec.CommandContext(ctx, "tpm_manager_client", "take_ownership").Run(); err != nil {
		return errors.Wrap(err, "tpm_manager_client take_ownership failed")
	}
	// Don't wait for attestation prepared when we are using TPM runtime selection.
	if runtimeSelect {
		return nil
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, initialized, _, _, err := tpmStatus(ctx); err != nil {
			// attestation error encountered while polling.
			return testing.PollBreak(err)
		} else if !initialized {
			return errors.New("TPM not initialized yet")
		}
		return nil
	}, nil)
}

// ClearPoliciesLogLocation is the location of the error log for clearPolicies.
// If the removing policies did not encounter any errors the file should not exist.
const ClearPoliciesLogLocation = "/tmp/ready-clearPolicies.err"

func errorLogAppendError(path, msg string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to open error log %q", path)
	}
	defer f.Close()

	if _, err := f.WriteString(msg + "\n"); err != nil {
		return errors.Wrapf(err, "failed to append %q to error log %q", msg, path)
	}

	return nil
}

// clearPolicies removes all policies that might have been set by previously running tests.
// Tests that do not work with policies might still be affected by them, so this brings the device back to the default state.
// It is possible that device is already enrolled, but to unenroll the device we need a reboot, so we can do nothing here.
func clearPolicies(ctx context.Context) {
	// /var/lib/devicesettings is a directory containing device policies.
	// /home/chronos/Local State is a file containing local state JSON including user policy data.
	policyPattern := "/var/lib/devicesettings/*"
	policyFiles, err := filepath.Glob(policyPattern)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get policy files: ", err)
	} else {
		for _, policyFile := range policyFiles {
			if err := os.Remove(policyFile); err != nil {
				testing.ContextLog(ctx, "Failed to get policy file: ", err)
			}
		}
	}

	// Clear error log for this function.
	if err := os.RemoveAll(ClearPoliciesLogLocation); err != nil {
		testing.ContextLogf(ctx, "Failed to remove error log %q: %v", ClearPoliciesLogLocation, err)
	}

	localState := "/home/chronos/Local State"
	if err := os.RemoveAll(localState); err != nil {
		msg := fmt.Sprintf("Failed to remove %q: %v", localState, err)
		testing.ContextLog(ctx, msg)
		if err := errorLogAppendError(ClearPoliciesLogLocation, msg); err != nil {
			testing.ContextLog(ctx, err.Error())
		}
	}

	// Services that cache policies (like Chromium) are not restarted here.
	// Tests that depend on the state of those services should perform the restart.
	// Chromium related tests already restart Chromium and session_manager which will reload policies.
}

// EnterpriseOwnedLogLocation is the location of the error log for checkEnterpriseOwned.
// If the the DUT is not enterprise owned the file should not exist.
const EnterpriseOwnedLogLocation = "/tmp/ready-enterpriseOwned.err"

var trueRegex = regexp.MustCompile(`(?m)^\s*[Tt]rue\s*$`)

func checkEnterpriseOwned(ctx context.Context) {
	isEnterpriseOwned := func(ctx context.Context) bool {
		out, err := testexec.CommandContext(ctx, "cryptohome", "--action=install_attributes_get", "--name=enterprise.owned").Output()
		if err != nil {
			// Don't fail as install attributes can be missing. Device is not
			// enterprise owned in that case.
			return false
		}

		owned := trueRegex.Match(out)
		return owned
	}

	// Clear error log for this function.
	if err := os.RemoveAll(EnterpriseOwnedLogLocation); err != nil {
		testing.ContextLogf(ctx, "Failed to remove error log %q: %v", EnterpriseOwnedLogLocation, err)
	}

	if isEnterpriseOwned(ctx) {
		if err := errorLogAppendError(EnterpriseOwnedLogLocation, "Device is enterprise owned"); err != nil {
			testing.ContextLog(ctx, err.Error())
		}
		testing.ContextLog(ctx, "Device is enterprise owned, please clear ownership before running tests")
		testing.ContextLog(ctx, "To clear ownership you can powerwash (Ctrl + Alt + Shift + r at the login screen)")
	}
}
