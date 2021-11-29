// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lifecycle

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type requestFlexibleLoopbackAction struct {
	requestSec int
	floopMask  int
}

var _ Action = requestFlexibleLoopbackAction{}

func (a requestFlexibleLoopbackAction) Do(ctx context.Context, s *testing.State, t *tester) {
	t.logEvent(ctx, a.requestSec, "request flexible loopback", true)
	floopDev, err := requestFloopMask(ctx, a.floopMask)
	if err != nil {
		s.Fatal("Request floop failed: ", floopDev)
	}
}

func (a requestFlexibleLoopbackAction) maybeLogSchedule(ctx context.Context, t *tester) {
	t.logScheduleRow(ctx, "floop active", 'f', schedule{a.requestSec, t.testDurationSec})
}

func (a requestFlexibleLoopbackAction) getSchedule() schedule {
	return schedule{a.requestSec, a.requestSec}
}

func (a requestFlexibleLoopbackAction) durationSec() int {
	return 0
}

// RequestFloopOnly returns a Action which requests the flexible loopback at requestSec
func RequestFloopOnly(requestSec int) Action {
	return &requestFlexibleLoopbackAction{
		requestSec: requestSec,
		floopMask:  4,
	}
}

type captureFlexibleLoopbackAction struct {
	isCaptureAction
	schedule
	requestSec int
	floopMask  int
}

var _ CaptureAction = captureFlexibleLoopbackAction{}

func (a captureFlexibleLoopbackAction) Do(ctx context.Context, s *testing.State, t *tester) {
	t.logEvent(ctx, a.requestSec, "request flexible loopback", true)
	floopDev, err := requestFloopMask(ctx, a.floopMask)
	if err != nil {
		s.Fatal("Request floop failed: ", floopDev)
	}

	loopbackArgs := []string{
		fmt.Sprintf("--capture_file=%s", t.captureRaw),
		fmt.Sprintf("--pin_device=%d", floopDev),
	}

	runCapture(ctx, s, t, a.startSec, a.endSec, loopbackArgs)
}

func requestFloopMask(ctx context.Context, mask int) (dev int, err error) {
	cmd := testexec.CommandContext(
		ctx,
		"cras_test_client",
		fmt.Sprintf("--request_floop_mask=%d", mask),
	)
	stdout, _, err := cmd.SeparatedOutput()
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(`flexible loopback dev id: (\d+)`)
	m := re.FindSubmatch(stdout)
	if m == nil {
		return -1, errors.Errorf("output %q not matching %q", string(stdout), re)
	}
	return strconv.Atoi(string(m[1]))
}

func (a captureFlexibleLoopbackAction) maybeLogSchedule(ctx context.Context, t *tester) {
	t.logScheduleRow(ctx, "floop active", 'f', schedule{a.requestSec, t.testDurationSec})
	t.logScheduleRow(ctx, "capture", 'c', a.schedule)
}

// CaptureFloop returns a CaptureAction which:
// - requests the flexible loopback at requestSec
// - captures from the flexible loopback between startSec and endSec
func CaptureFloop(requestSec, startSec, endSec int) CaptureAction {
	return &captureFlexibleLoopbackAction{
		schedule:   schedule{startSec, endSec},
		requestSec: requestSec,
		floopMask:  4,
	}
}

// CaptureMismatchedFloop is like FloopCapture but the requested flexible loopback device
// is configured to not match the Playback CRAS client, so no audio should be captured
func CaptureMismatchedFloop(requestSec, startSec, endSec int) CaptureAction {
	return &captureFlexibleLoopbackAction{
		schedule:   schedule{startSec, endSec},
		requestSec: requestSec,
		floopMask:  0,
	}
}
