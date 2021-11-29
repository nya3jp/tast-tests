// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lifecycle

import (
	"context"
	"fmt"

	"chromiumos/tast/testing"
)

type postMixType int

const (
	preDSP  postMixType = 0
	postDSP             = 1
)

type capturePostMixLoopbackAction struct {
	isCaptureAction
	schedule
	loopbackType postMixType
}

var _ CaptureAction = capturePostMixLoopbackAction{}

func (a capturePostMixLoopbackAction) Do(ctx context.Context, s *testing.State, t *tester) {
	loopbackArgs := []string{
		fmt.Sprintf("--loopback_file=%s", t.captureRaw),
		fmt.Sprintf("--post_dsp=%d", a.loopbackType),
	}

	runCapture(ctx, s, t, a.startSec, a.endSec, loopbackArgs)
}

func (a capturePostMixLoopbackAction) maybeLogSchedule(ctx context.Context, t *tester) {
	t.logScheduleRow(ctx, "capture", 'c', a.schedule)
}

// CapturePostMixPreDsp returns a CaptureAction which captures from the post mix pre dsp loopback
func CapturePostMixPreDsp(startSec, endSec int) CaptureAction {
	return capturePostMixLoopbackAction{
		loopbackType: preDSP,
		schedule:     schedule{startSec, endSec},
	}
}
