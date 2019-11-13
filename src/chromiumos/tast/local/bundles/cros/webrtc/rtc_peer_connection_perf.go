// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/camera"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RTCPeerConnectionPerf,
		Desc: "Collects performance data about WebRTC loopback",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.DataFiles(), "loopback_camera.html"),
		Pre:          pre.ChromeFakeCameraPerf(),
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
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

// RTCPeerConnectionPerf is the performance-collection version of
// webrtc.RTCPeerConnection. This test performs a WebRTC loopback call for 20
// seconds. If there is no error while exercising the camera, it uploads
// statistics of black/frozen frames and input/output FPS will be logged.
func RTCPeerConnectionPerf(ctx context.Context, s *testing.State) {
	// Run loopback call for 20 seconds.
	result := runPeerConn(ctx, s, s.PreValue().(*chrome.Chrome), s.Param().(videotype.Codec), 20*time.Second, camera.NoVerboseLogging)

	if !s.HasError() {
		// Set and upload perf metrics below.
		p := perf.NewValues()
		result.setPerf(p, s.Param().(videotype.Codec))
		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
