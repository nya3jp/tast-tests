// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/peerconnection"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodePerf,
		Desc:         "Measures WebRTC decode performance in terms of CPU usage and decode time with and without hardware acceleration",
		Contacts:     []string{
			"hiroh@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.LoopbackDataFiles(), "crowd720_25frames.y4m", webrtc.AddStatsJSFile),
		Params: []testing.Param{{
			Name:              "h264",
			Val:               "H264",
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
		}},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
	})
}

// DecodePerf opens a WebRTC loopback page that loops a given capture stream to measure decode time and CPU usage.
func DecodePerf(ctx context.Context, s *testing.State) {
	addStatsJS, err := ioutil.ReadFile(s.DataPath(webrtc.AddStatsJSFile))
	if err != nil {
		s.Fatal("Failed to read JS for gathering decode time: ", err)
	}
	peerconnection.RunDecodePerf(ctx, s, s.Param().(string), "crowd720_25frames.y4m", peerconnection.MeasureConfig{
		CPUStabilize:      10 * time.Second,
		CPUMeasure:        30 * time.Second,
		DecodeTimeTimeout: 30 * time.Second,
		DecodeTimeSamples: 10,
		AddStatsJS:        string(addStatsJS),
	})
}
