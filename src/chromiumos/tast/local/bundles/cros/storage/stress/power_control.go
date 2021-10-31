// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stress

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
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

	amdS0ixResidencyFile    = "/sys/kernel/debug/amd_pmc/s0ix_stats"
	intelS0ixResidencyFile1 = "/sys/kernel/debug/pmc_core/slp_s0_residency_usec"
	intelS0ixResidencyFile2 = "/sys/kernel/debug/telemetry/s0ix_residency_usec"

	amdS0ixResidencyFilePattern = "Residency Time: "
)

// Suspend suspends the device for a pre-defined time and then resumes it.
func Suspend(ctx context.Context, skipS0iXResidencyCheck bool) error {
	return suspend(ctx, defaultSuspendTimeout, defaultSuspendWakeupTime, skipS0iXResidencyCheck)
}

func suspend(ctx context.Context, timeout, wakeup time.Duration, skipS0iXResidencyCheck bool) error {
	inSec := func(dur time.Duration) string {
		return strconv.Itoa(int(dur / time.Second))
	}

	var s0ixDuration, s0ixDurationFinal time.Duration
	var err error
	if skipS0iXResidencyCheck {
		testing.ContextLog(ctx, "Skipping s0ix residency check")
	} else {
		if s0ixDuration, err = getS0ixResidencyStats(); err != nil {
			return errors.Wrap(err, "failed to aquire s0ix residency time")
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

	if !skipS0iXResidencyCheck {
		s0ixDurationFinal, err = getS0ixResidencyStats()
		if err != nil {
			return errors.Wrap(err, "failed to aquire s0ix residency time")
		}
	}

	if !skipS0iXResidencyCheck && s0ixDuration == s0ixDurationFinal {
		return errors.Errorf("s0ix resudency duration did not change: %d", s0ixDuration)
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

func getS0ixResidencyStats() (time.Duration, error) {
	return getS0ixResidencyStatsFromFiles(amdS0ixResidencyFilePattern,
		[]string{intelS0ixResidencyFile1, intelS0ixResidencyFile2})
}

func getS0ixResidencyStatsFromFiles(amdResidencyFile string,
	intelResidencyFiles []string) (time.Duration, error) {
	// Trying AMD status file first.
	if f, err := os.Open(amdResidencyFile); err == nil {
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
