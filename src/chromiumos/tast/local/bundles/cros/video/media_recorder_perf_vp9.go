// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/bundles/cros/video/mediarecorder"
	"chromiumos/tast/testing"
)

const mediaRecorderVP9StreamFile = "crowd720_25frames.y4m"

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderPerfVP9,
		Desc: "Captures performance data about MediaRecorder for SW and HW with H.264",
		Contacts: []string{
			"hiroh@chromium.org",    // Video team
			"shenghao@chromium.org", // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         []string{mediaRecorderVP9StreamFile, "loopback_media_recorder.html"},
		Timeout:      3 * time.Minute,
	})
}

// MediaRecorderPerfVP9 captures the perf data of MediaRecorder for HW and SW cases with VP9 codec and uploads to server.
func MediaRecorderPerfVP9(ctx context.Context, s *testing.State) {
	const (
		fps   = 30
		codec = videotype.VP9
	)
	if err := mediarecorder.MeasurePerf(ctx, s.DataFileSystem(), s.OutDir(), codec, s.DataPath(mediaRecorderVP9StreamFile), fps); err != nil {
		s.Error("Failed to measure performance: ", err)
	}
}
