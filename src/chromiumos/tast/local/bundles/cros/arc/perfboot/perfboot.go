// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perfboot

import (
	"bufio"
	"context"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// GetPerfValues parses ARC log files and extracts performance metrics Android boot flow.
func GetPerfValues(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) (map[string]time.Duration, error) {
	const (
		logcatTimeout = 30 * time.Second

		// logcatLastEventTag is the last event tag to be processed.
		// The test should read logcat until this tag appears.
		logcatLastEventTag = "boot_progress_enable_screen"

		// logcatIgnoreEventTag is a logcat event tags to be ignored.
		// TODO(niwa): Clean this up after making PerfBoot reboot DUT.
		// (Using time of boot_progress_system_run makes sense only after rebooting DUT.)
		logcatIgnoreEventTag = "boot_progress_system_run"
	)

	// logcatEventEntryRegexp extracts boot pregress event name and time from a logcat entry.
	logcatEventEntryRegexp := regexp.MustCompile(`\d+ I (boot_progress_[^:]+): (\d+)`)

	var arcStartTimeMS float64
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.getArcStartTime)()", &arcStartTimeMS); err != nil {
		return nil, errors.Wrap(err, "failed to run getArcStartTime()")
	}
	adjustedArcStartTime := time.Duration(arcStartTimeMS * float64(time.Millisecond))
	testing.ContextLogf(ctx, "ARC start time in host clock: %fs", adjustedArcStartTime.Seconds())

	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check whether ARCVM is enabled")
	}
	if vmEnabled {
		clockDelta, err := clockDelta(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain clock delta")
		}
		// Guest clock and host clock are different on ARCVM, so we adjust ARC start time.
		// adjustedArcStartTime is expected to be a negative value.
		adjustedArcStartTime -= clockDelta
		testing.ContextLogf(ctx, "ARC start time in guest clock: %fs", adjustedArcStartTime.Seconds())
	}

	// Set timeout for the logcat command below.
	ctx, cancel := context.WithTimeout(ctx, logcatTimeout)
	defer cancel()

	cmd := a.Command(ctx, "logcat", "-b", "events", "-v", "threadtime")
	cmdStr := shutil.EscapeSlice(cmd.Args)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to obtain a pipe for %s", cmdStr)
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "failed to start %s", cmdStr)
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	p := make(map[string]time.Duration)
	lastEventSeen := false

	testing.ContextLog(ctx, "Scanning logcat output")
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		l := scanner.Text()

		m := logcatEventEntryRegexp.FindStringSubmatch(l)
		if m == nil {
			continue
		}

		eventTag := m[1]
		if eventTag == logcatIgnoreEventTag {
			continue
		}

		eventTimeMs, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to extract event time from %q", l)
		}

		p[eventTag] = time.Duration(eventTimeMs*int64(time.Millisecond)) - adjustedArcStartTime

		if eventTag == logcatLastEventTag {
			lastEventSeen = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error while scanning logcat")
	}
	if !lastEventSeen {
		return nil, errors.Errorf("timeout while waiting for event %q to appear in logcat",
			logcatLastEventTag)
	}

	return p, nil
}

// clockDelta returns (the host's CLOCK_MONOTONIC - the guest's CLOCK_MONOTONIC) as time.Duration.
func clockDelta(ctx context.Context) (time.Duration, error) {
	// /proc/timer_list contains a line which says "now at %Ld nsecs".
	// This clock value comes from CLOCK_MONOTONIC (see the kernel's kernel/time/timer_list.c).
	parse := func(output string) (int64, error) {
		for _, line := range strings.Split(output, "\n") {
			tokens := strings.Split(line, " ")
			if len(tokens) == 4 && tokens[0] == "now" && tokens[1] == "at" && tokens[3] == "nsecs" {
				return strconv.ParseInt(tokens[2], 10, 64)
			}
		}
		return 0, errors.Errorf("unexpected format of /proc/timer_list: %q", output)
	}

	// Use android-sh to read /proc/timer_list which only root can read.
	out, err := arc.BootstrapCommand(ctx, "/system/bin/cat", "/proc/timer_list").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read guest's /proc/timer_list")
	}
	guestClockNS, err := parse(string(out))
	if err != nil {
		return 0, errors.Wrap(err, "failed to prase guest's /proc/timer_list")
	}

	out, err = ioutil.ReadFile("/proc/timer_list")
	if err != nil {
		return 0, errors.Wrap(err, "failed to read host's /proc/timer_list")
	}
	hostClockNS, err := parse(string(out))
	if err != nil {
		return 0, errors.Wrap(err, "failed to prase host's /proc/timer_list")
	}

	testing.ContextLogf(ctx, "Host clock: %d ns, Guest clock: %d ns", hostClockNS, guestClockNS)
	return time.Duration((hostClockNS - guestClockNS) * int64(time.Nanosecond)), nil
}
