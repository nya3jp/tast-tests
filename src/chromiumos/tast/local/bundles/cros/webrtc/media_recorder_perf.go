// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderPerf,
		Desc: "Captures performance data about MediaRecorder for both SW and HW",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{mediarecorder.PerfStreamFile, "loopback_media_recorder.html"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "h264",
			Val:  videotype.H264,
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name: "vp8",
			Val:  videotype.VP8,
		}, {
			Name: "vp9",
			Val:  videotype.VP9,
		}},
	})
}

// MediaRecorderPerf captures the perf data of MediaRecorder for HW and SW
// cases with a given codec and uploads to server.
func MediaRecorderPerf(ctx context.Context, s *testing.State) {
	if err := mediarecorder.MeasurePerf(ctx, s.DataFileSystem(), s.OutDir(), s.Param().(videotype.Codec), s.DataPath(mediarecorder.PerfStreamFile)); err != nil {
		s.Error("Failed to measure performance: ", err)
	}
}
