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
		Func: CrasPostMixPreDspLifecycle,

		Desc: "Post mix pre dsp loopback should handle different lifecycle stituations",
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
		// - p: playback
		// - c: capture
		// - b: both playback & capture
		Params: []testing.Param{
			// Only capture
			{
				Name: "c",
				Val: &lifecycle.Param{
					Capture: lifecycle.CapturePostMixPreDsp(0, 3),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
					},
				},
			},

			// No overlaps
			{
				Name: "pc",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(0, 2),
					Capture:  lifecycle.CapturePostMixPreDsp(4, 7),
					Checks: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(5, 6),
					},
				},
			},
			{
				Name: "cp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(5, 7),
					Capture:  lifecycle.CapturePostMixPreDsp(0, 3),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
					},
				},
			},

			// Partial overlap
			{
				Name: "pbc",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(0, 5),
					Capture:  lifecycle.CapturePostMixPreDsp(2, 8),
					Checks: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(3, 4),
						lifecycle.CheckZeroSample(6, 7),
					},
				},
			},
			{
				Name: "cbp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(3, 8),
					Capture:  lifecycle.CapturePostMixPreDsp(0, 6),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
						lifecycle.CheckCaptureSample(4, 5),
					},
				},
			},

			// Contained
			{
				Name: "cbc",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(3, 6),
					Capture:  lifecycle.CapturePostMixPreDsp(0, 9),
					Checks: []lifecycle.Checker{
						lifecycle.CheckZeroSample(1, 2),
						lifecycle.CheckCaptureSample(4, 5),
						lifecycle.CheckZeroSample(7, 8),
					},
				},
			},
			{
				Name: "pbp",
				Val: &lifecycle.Param{
					Playback: lifecycle.Playback(0, 7),
					Capture:  lifecycle.CapturePostMixPreDsp(2, 5),
					Checks: []lifecycle.Checker{
						lifecycle.CheckCaptureSample(3, 4),
					},
				},
			},
		},
	})
}

func CrasPostMixPreDspLifecycle(ctx context.Context, s *testing.State) {
	param := s.Param().(*lifecycle.Param)
	lifecycle.TestLifecycle(ctx, s, param)
}
