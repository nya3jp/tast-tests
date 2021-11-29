// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"

	"chromiumos/tast/local/bundles/cros/audio/loopback/lifecycle"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrasFloopLifecycle,

		Desc: "Flexible loopback should handle different lifecycle stituations",
		// Specifically:
		//   - Capture should get zero samples and not be blocked when there is no playback stream
		//   - Capture should get corresponding audio when there is a playback stream
		//   - Playback should not be blocked when there is no capture stream
		// No capture/playback stream can be due to the stream:
		//   - hasn't started yet
		//   - has ended

		Contacts: []string{"aaronyu@google.com", "chromeos-audio-bugs@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// Param.Name encoding:
		// - r: request flexible loopback
		// - p: playback
		// - c: capture
		// - b: both playback & capture
		Params: []testing.Param{
			{
				Name: "prp",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(0, 4),
					RequestFloopSec: 2,
					FloopMask:       4,
				},
			},
			{
				Name: "rp",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(2, 4),
					RequestFloopSec: 0,
					FloopMask:       4,
				},
			},
			{
				Name: "rc",
				Val: &lifecycle.Param{
					CaptureTime:     lifecycle.TI(0, 3),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
					},
				},
			},
			{
				Name: "prbp",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(0, 7),
					CaptureTime:     lifecycle.TI(2, 5),
					RequestFloopSec: 2,
					FloopMask:       4,
					Checkers: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(3, 4),
					},
				},
			},
			{
				Name: "rpbp",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(2, 9),
					CaptureTime:     lifecycle.TI(4, 7),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(5, 6),
					},
				},
			},
			{
				Name: "rcbc",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(2, 5),
					CaptureTime:     lifecycle.TI(0, 7),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(3, 4),
					},
				},
			},
			{
				Name: "rcbp",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(3, 8),
					CaptureTime:     lifecycle.TI(0, 6),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
						lifecycle.CheckCaptureSample(4, 5),
					},
				},
			},
			{
				Name: "rpbc",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(2, 7),
					CaptureTime:     lifecycle.TI(4, 10),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(5, 6),
						lifecycle.CheckZeroSample(8, 9),
					},
				},
			},
			{
				Name: "rpbp_mismatch",
				Val: &lifecycle.Param{
					PlaybackTime:    lifecycle.TI(2, 9),
					CaptureTime:     lifecycle.TI(4, 7),
					RequestFloopSec: 0,
					FloopMask:       0, // zero mask will not match the playback client
					Checkers: []lifecycle.Checker{
						lifecycle.CheckZeroSample(5, 6),
					},
				},
			},
		},
	})
}

func CrasFloopLifecycle(ctx context.Context, s *testing.State) {
	param := s.Param().(*lifecycle.Param)
	lifecycle.TestFloopPlaybackCapture(ctx, s, param)
}
