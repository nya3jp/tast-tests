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
			// Only playback/capture
			{
				Name: "prp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(0, 4),
					ExtraActions: []lifecycle.Action{
						lifecycle.RequestFloopOnly(2),
					},
				},
			},
			{
				Name: "rp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(2, 4),
					ExtraActions: []lifecycle.Action{
						lifecycle.RequestFloopOnly(0),
					},
				},
			},
			{
				Name: "rc",
				Val: &lifecycle.Param{
					Capture: lifecycle.CaptureFloop(0, 0, 3),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
					},
				},
			},

			// No overlap
			{
				Name: "rcp",
				Val: &lifecycle.Param{
					Capture:  lifecycle.CaptureFloop(0, 0, 3),
					Playback: lifecycle.Playback(5, 7),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
					},
				},
			},
			{
				Name: "prc",
				Val: &lifecycle.Param{
					Capture:  lifecycle.CaptureFloop(4, 4, 7),
					Playback: lifecycle.Playback(0, 2),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(5, 6),
					},
				},
			},
			{
				Name: "rpc",
				Val: &lifecycle.Param{
					Capture:  lifecycle.CaptureFloop(0, 6, 9),
					Playback: lifecycle.Playback(2, 4),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(7, 8),
					},
				},
			},

			// Partial overlap
			{
				Name: "rcbp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(3, 8),
					Capture:  lifecycle.CaptureFloop(0, 0, 6),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
						lifecycle.CheckCaptureSample(4, 5),
					},
				},
			},
			{
				Name: "rpbc",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(2, 7),
					Capture:  lifecycle.CaptureFloop(0, 4, 10),
					Checks: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(5, 6),
						lifecycle.CheckZeroSample(8, 9),
					},
				},
			},
			{
				Name: "prbc",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(0, 5),
					Capture:  lifecycle.CaptureFloop(2, 2, 8),
					Checks: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(3, 4),
						lifecycle.CheckZeroSample(6, 7),
					},
				},
			},

			// Contained
			{
				Name: "prbp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(0, 7),
					Capture:  lifecycle.CaptureFloop(2, 2, 5),
					Checks: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(3, 4),
					},
				},
			},
			{
				Name: "rpbp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(2, 9),
					Capture:  lifecycle.CaptureFloop(0, 4, 7),
					Checks: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(5, 6),
					},
				},
			},
			{
				Name: "rcbc",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(3, 6),
					Capture:  lifecycle.CaptureFloop(0, 0, 9),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
						lifecycle.CheckCaptureSample(4, 5),
						lifecycle.CheckZeroSample(7, 8),
					},
				},
			},

			// Mismatch
			{
				Name: "rpbp_mismatch",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(2, 7),
					Capture:  lifecycle.CaptureMismatchedFloop(0, 4, 7),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(5, 6),
					},
				},
			},
		},
	})
}

func CrasFloopLifecycle(ctx context.Context, s *testing.State) {
	param := s.Param().(*lifecycle.Param)
	lifecycle.TestLifecycle(ctx, s, param)
}
