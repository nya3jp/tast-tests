// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	defaultSuspendTimeout    = 20 * time.Second
	defaultSuspendWakeupTime = 30 * time.Second

	defaultSuspendMin = 30
	defaultSuspendMax = 35
	defaultWakeMin    = 10
	defaultWakeMax    = 25

	amdS0ixResidencyFile    = "/sys/kernel/debug/amd_pmc/s0ix_stats"
	intelS0ixResidencyFile1 = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
	intelS0ixResidencyFile2 = "/sys/kernel/debug/telemetry/s0ix_residency_usec"

	amdS0ixResidencyFilePattern = "Residency Time: "

	s2IdleResidencyFilePathPattern = "/sys/devices/system/cpu/cpu[0-9]/cpuidle/state*/s2idle/time"

	suspendStressTestPath = "/usr/bin/suspend_stress_test"
)

// SuspendStressTest executes the suspend_stress_test on the DUT with the given
// number of loops.
func SuspendStressTest(ctx context.Context, duration time.Duration) (string, error) {
	minSuspend := "--suspend_min=" + strconv.Itoa(defaultSuspendMin)
	maxSuspend := "--suspend_max=" + strconv.Itoa(defaultSuspendMax)
	minWake := "--wake_min=" + strconv.Itoa(defaultWakeMin)
	maxWake := "--wake_max=" + strconv.Itoa(defaultWakeMax)

	durationSec := int(duration / time.Second)
	loopMaxTime := defaultSuspendMax + defaultWakeMax
	numLoops := durationSec / loopMaxTime

	loops := "--count=" + strconv.Itoa(numLoops)

	cmd := testexec.CommandContext(ctx, suspendStressTestPath, minSuspend,
		maxSuspend, minWake, maxWake, loops)
	testing.ContextLog(ctx, "Running command: ", cmd)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to run suspend_stress_test")
	}

	return string(out), nil
}

// Suspend suspends the device for a pre-defined time and then resumes it.
func Suspend(ctx context.Context, skipResidencyCheck bool) error {
	return suspend(ctx, defaultSuspendTimeout, defaultSuspendWakeupTime, skipResidencyCheck)
}

func suspend(ctx context.Context, timeout, wakeup time.Duration, skipResidencyCheck bool) error {
	inSec := func(dur time.Duration) string {
		return strconv.Itoa(int(dur / time.Second))
	}

	var s0ixDuration, s0ixDurationFinal time.Duration
	var err error
	var isFreeze bool
	if skipResidencyCheck {
		testing.ContextLog(ctx, "Skipping residency check")
	} else {
		if isFreeze, err = isSleepStateFreeze(); err != nil {
			testing.ContextLog(ctx, "Failed to aquire sleep state config: ", err)
		}
		if isFreeze {
			if s0ixDuration, err = getResidencyStats(); err != nil {
				testing.ContextLog(ctx, "Failed to aquire s0ix residency time: ", err)
				skipResidencyCheck = true
			}
		}
	}

	testing.ContextLog(ctx, "Suspending DUT, wakeup = ", wakeup)
	err = testexec.CommandContext(ctx, "powerd_dbus_suspend",
		"--timeout="+inSec(timeout),
		"--wakeup_timeout="+inSec(wakeup)).Run(testexec.DumpLogOnError)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// It is a normal behavior if suspend is interrupted by context deadline.
			testing.ContextLog(ctx, "Deadline Exceeded")
			return nil
		}
		// Device might still be trying to suspend, so need to restart.
		restartPowerd(ctx)

		return errors.Wrap(err, "failed to suspend device")
	}

	if !skipResidencyCheck && isFreeze {
		s0ixDurationFinal, err = getResidencyStats()
		if err != nil {
			return errors.Wrap(err, "failed to aquire s0ix residency time")
		}
		if s0ixDuration == s0ixDurationFinal {
			return errors.Errorf("s0ix resudency duration did not change: %d", s0ixDuration)
		}
	}

	return nil
}

func restartPowerd(ctx context.Context) {
	testing.ContextLog(ctx, "Restarting powerd")
	err := testexec.CommandContext(ctx, "restart", "powerd").Run(testexec.DumpLogOnError)
	if err != nil {
		testing.ContextLog(ctx, "Failed restarting DUT powerd: ", err)
	}
}

func getResidencyStats() (time.Duration, error) {
	if runtime.GOARCH == "amd64" {
		return getS0ixResidencyStatsFromFiles(amdS0ixResidencyFilePattern,
			[]string{intelS0ixResidencyFile1, intelS0ixResidencyFile2})
	} else if runtime.GOARCH == "arm64" {
		return getS2IdleResidencyStats(s2IdleResidencyFilePathPattern)
	} else {
		return 0, errors.Errorf("unsupported architecture for residency stats: %s", runtime.GOARCH)
	}
}

func getS2IdleResidencyStats(filePattern string) (time.Duration, error) {
	var totalDuration time.Duration

	files, err := filepath.Glob(filePattern)
	if err != nil {
		return 0, errors.Wrap(err, "error reading s2 idle files")
	}

	if len(files) == 0 {
		return 0, errors.Errorf("no s2idle residency files found for path: %s", filePattern)
	}

	for _, file := range files {
		if data, err := ioutil.ReadFile(file); err == nil {
			if duration, err := parseDuration(string(data)); err == nil {
				totalDuration += duration
			} else {
				return 0, errors.Wrapf(err, "error parsing value from: %s", file)
			}
		} else {
			return 0, errors.Wrapf(err, "error reading s2idle file: %s", file)
		}
	}

	return totalDuration, nil
}

func getS0ixResidencyStatsFromFiles(amdResidencyFile string,
	intelResidencyFiles []string) (time.Duration, error) {
	if f, err := os.Open(amdResidencyFile); err == nil {
		// Trying AMD status file first.
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, amdS0ixResidencyFilePattern) {
				return parseDuration(line[len(amdS0ixResidencyFilePattern):])
			}
		}
	} else {
		// Trying Intel status files.
		for _, path := range intelResidencyFiles {
			if data, err := ioutil.ReadFile(path); err == nil {
				return parseDuration(string(data))
			}
		}
	}

	return 0, errors.New("unable to find s0ix residency file on the system")
}

func parseDuration(line string) (time.Duration, error) {
	if len(line) == 0 {
		return 0, errors.New("duration value string is empty")
	}

	var duration int64
	var err error
	if duration, err = strconv.ParseInt(strings.TrimSpace(line), 10, 64); err != nil {
		return 0, errors.Wrap(err, "failed to parse duration value")
	}

	return time.Duration(duration) * time.Nanosecond, nil
}

func isSleepStateFreeze() (bool, error) {
	cmd := exec.Command("check_powerd_config", "--suspend_to_idle")
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode() == 0, nil
		}
		return false, errors.Wrap(err, "failed acquiring sleep state")
	}
	// If command runs without error, exit status must be zero.
	return true, nil
}
