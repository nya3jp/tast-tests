// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"io/ioutil"
	"time"

	// TODO(crbug.com/971922): Remove /media/webrtc package.
	mediaWebRTC "chromiumos/tast/local/media/webrtc"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCDecodePerf,
		Desc:         "Measures WebRTC decode performance in terms of CPU usage and decode time with and without hardware acceleration",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.LoopbackDataFiles(), "crowd720_25frames.y4m", webrtc.AddStatsJSFile),
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
	})
}

// WebRTCDecodePerf opens a WebRTC loopback page that loops a given capture stream to measure decode time and CPU usage.
func WebRTCDecodePerf(ctx context.Context, s *testing.State) {
	addStatsJS, err := ioutil.ReadFile(s.DataPath(webrtc.AddStatsJSFile))
	if err != nil {
		s.Fatal("Failed to read JS for gathering decode time: ", err)
	}
	mediaWebRTC.RunWebRTCDecodePerf(ctx, s, "crowd720_25frames.y4m", mediaWebRTC.MeasureConfig{
		CPUStabilize:      10 * time.Second,
		CPUMeasure:        30 * time.Second,
		DecodeTimeTimeout: 30 * time.Second,
		DecodeTimeSamples: 10,
		AddStatsJS:        string(addStatsJS),
	})
}
