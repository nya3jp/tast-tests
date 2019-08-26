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
		Func: MediaRecorderPerfVP9,
		Desc: "Captures performance data about MediaRecorder for SW and HW with VP9",
		Contacts: []string{
			"hiroh@chromium.org", // Video team
			"wtlee@chromium.org", // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{mediarecorder.PerfStreamFile, "loopback_media_recorder.html"},
		Timeout:      3 * time.Minute,
	})
}

// MediaRecorderPerfVP9 captures the perf data of MediaRecorder for HW and SW cases with VP9 codec and uploads to server.
func MediaRecorderPerfVP9(ctx context.Context, s *testing.State) {
	const fps = 30
	if err := mediarecorder.MeasurePerf(ctx, s.DataFileSystem(), s.OutDir(), videotype.VP8, s.DataPath(mediarecorder.PerfStreamFile), fps); err != nil {
		s.Error("Failed to measure performance: ", err)
	}
}
