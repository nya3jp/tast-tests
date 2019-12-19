// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"regexp"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	localArc "chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arc.RegisterPerfBootServiceServer(srv, &PerfBootService{s: s})
		},
	})
}

// PerfBootService implements tast.cros.arc.PerfBootService.
type PerfBootService struct {
	s *testing.ServiceState
}

func (c *PerfBootService) WaitUntilCPUCoolDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownStopUI); err != nil {
		return nil, errors.Wrap(err, "failed to wait until CPU is cooled down: ")
	}
	return &empty.Empty{}, nil
}

func (c *PerfBootService) GetPerfValues(ctx context.Context, req *empty.Empty) (*arc.GetPerfValuesResponse, error) {
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
	var logcatEventEntryRegexp = regexp.MustCompile(`\d+ I (boot_progress_[^:]+): (\d+)`)

	// TODO(niwa): Check if we should use GAIA login instead of fake login.
	// (Currently KeepState option only works for fake login.)
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.RestrictARCCPU(),
		chrome.KeepState(), chrome.ExtraArgs("--disable-arc-data-wipe"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome: ")
	}
	defer cr.Close(ctx)

	a, err := localArc.New(ctx, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC: ")
	}
	defer a.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Creating test API connection failed: ")
	}
	defer tconn.Close()

	var arcStartTimeMs float64
	if err := tconn.EvalPromise(ctx, "tast.promisify(chrome.autotestPrivate.getArcStartTime)()", &arcStartTimeMs); err != nil {
		return nil, errors.Wrap(err, "failed to run getArcStartTime()")
	}

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

	res := &arc.GetPerfValuesResponse{}

	testing.ContextLog(ctx, "Scanning logcat output")
	scanner := bufio.NewScanner(pipe)
	scanStartTime := time.Now()
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
		correctedEventTimeMs := eventTimeMs - int64(arcStartTimeMs)

		perfValue := &arc.GetPerfValuesResponse_PerfValue{
			Name:    eventTag,
			ValueMs: correctedEventTimeMs,
		}
		res.Values = append(res.Values, perfValue)

		if eventTag == logcatLastEventTag {
			break
		}

		if time.Now().After(scanStartTime.Add(logcatTimeout)) {
			return nil, errors.Wrapf(err, "failed to wait for event %q to appear in logcat",
				logcatLastEventTag)
		}
	}

	return res, nil
}
