// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"

	"chromiumos/tast/local/bundles/cros/audio/floop"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrasFloop,

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
				Val: &floop.Param{
					PlaybackTime:    floop.TI(0, 4),
					RequestFloopSec: 2,
					FloopMask:       4,
				},
			},
			{
				Name: "rp",
				Val: &floop.Param{
					PlaybackTime:    floop.TI(2, 4),
					RequestFloopSec: 0,
					FloopMask:       4,
				},
			},
			{
				Name: "rc",
				Val: &floop.Param{
					CaptureTime:     floop.TI(0, 3),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []floop.Checker{
						floop.CheckZeroSample(1, 2),
					},
				},
			},
			{
				Name: "prbp",
				Val: &floop.Param{
					PlaybackTime:    floop.TI(0, 7),
					CaptureTime:     floop.TI(2, 5),
					RequestFloopSec: 2,
					FloopMask:       4,
					Checkers: []floop.Checker{
						floop.CheckCaptureSample(3, 4),
					},
				},
			},
			{
				Name: "rpbp",
				Val: &floop.Param{
					PlaybackTime:    floop.TI(2, 9),
					CaptureTime:     floop.TI(4, 7),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []floop.Checker{
						floop.CheckCaptureSample(5, 6),
					},
				},
			},
			{
				Name: "rcbc",
				Val: &floop.Param{
					PlaybackTime:    floop.TI(2, 5),
					CaptureTime:     floop.TI(0, 7),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []floop.Checker{
						floop.CheckCaptureSample(3, 4),
					},
				},
			},
			{
				Name: "rcbp",
				Val: &floop.Param{
					PlaybackTime:    floop.TI(3, 8),
					CaptureTime:     floop.TI(0, 6),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []floop.Checker{
						floop.CheckZeroSample(1, 2),
						floop.CheckCaptureSample(4, 5),
					},
				},
			},
			{
				Name: "rpbc",
				Val: &floop.Param{
					PlaybackTime:    floop.TI(2, 7),
					CaptureTime:     floop.TI(4, 10),
					RequestFloopSec: 0,
					FloopMask:       4,
					Checkers: []floop.Checker{
						floop.CheckCaptureSample(5, 6),
						floop.CheckZeroSample(8, 9),
					},
				},
			},
			{
				Name: "rpbp_mismatch",
				Val: &floop.Param{
					PlaybackTime:    floop.TI(2, 9),
					CaptureTime:     floop.TI(4, 7),
					RequestFloopSec: 0,
					FloopMask:       0, // zero mask will not match the playback client
					Checkers: []floop.Checker{
						floop.CheckZeroSample(5, 6),
					},
				},
			},
		},
	})
}

func CrasFloop(ctx context.Context, s *testing.State) {
	param := s.Param().(*floop.Param)
	floop.TestFloopPlaybackCapture(ctx, s, param)
}
