// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/perf/perfpb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/local/power"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterPerfBootServiceServer(srv, &PerfBootService{s: s})
		},
	})
}

// PerfBootService implements tast.cros.arc.PerfBootService.
type PerfBootService struct {
	s *testing.ServiceState
}

func (c *PerfBootService) WaitUntilCPUCoolDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if _, err := power.WaitUntilCPUCoolDown(ctx, power.DefaultCoolDownConfig(power.CoolDownStopUI)); err != nil {
		return nil, errors.Wrap(err, "failed to wait until CPU is cooled down")
	}
	return &empty.Empty{}, nil
}

func (c *PerfBootService) GetPerfValues(ctx context.Context, req *empty.Empty) (*perfpb.Values, error) {
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

	// TODO(niwa): Check if we should use GAIA login instead of fake login.
	// (Currently KeepState option only works for fake login.)
	// TODO(niwa): Check if we should really use KeepState.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.RestrictARCCPU(),
		chrome.KeepState(), chrome.ExtraArgs("--disable-arc-data-wipe", "--ignore-arcvm-dev-conf"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	defer os.RemoveAll(td)

	a, err := arc.New(ctx, td)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Creating test API connection failed")
	}
	defer tconn.Close()

	var arcStartTimeMS float64
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.getArcStartTime)()", &arcStartTimeMS); err != nil {
		return nil, errors.Wrap(err, "failed to run getArcStartTime()")
	}
	adjustedArcStartTimeMS := int64(arcStartTimeMS)
	testing.ContextLogf(ctx, "ARC start time in host clock: %dms", adjustedArcStartTimeMS)

	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		return nil, errors.Wrap(err, "failed to check whether ARCVM is enabled")
	}
	if vmEnabled {
		clockDeltaMS, err := clockDeltaMS(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain clock delta")
		}
		// Guest clock and host clock are different on ARCVM, so we adjust ARC start time.
		// adjustedArcStartTimeMS is expected to be a negative value.
		adjustedArcStartTimeMS -= clockDeltaMS
		testing.ContextLogf(ctx, "ARC start time in guest clock: %dms", adjustedArcStartTimeMS)
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

	p := perf.NewValues()
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

		eventTimeMS, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to extract event time from %q", l)
		}
		dur := time.Duration(eventTimeMS-adjustedArcStartTimeMS) * time.Millisecond

		p.Set(perf.Metric{
			Name:      eventTag,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, float64(dur.Milliseconds()))

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

	if err := metrics.MemoryMetrics(ctx, nil, a, p, "", ""); err != nil {
		return nil, errors.Wrap(err, "failed to collect memory metrics")
	}

	return p.Proto(), nil
}

// clockDeltaMS returns (the host's CLOCK_MONOTONIC - the guest's CLOCK_MONOTONIC) in milliseconds.
func clockDeltaMS(ctx context.Context) (int64, error) {
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

	out, err = testexec.CommandContext(ctx, "cat", "/proc/timer_list").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read host's /proc/timer_list")
	}
	hostClockNS, err := parse(string(out))
	if err != nil {
		return 0, errors.Wrap(err, "failed to prase host's /proc/timer_list")
	}

	testing.ContextLogf(ctx, "Host clock: %d ns, Guest clock: %d ns", hostClockNS, guestClockNS)
	return (hostClockNS - guestClockNS) / 1000000, nil
}
