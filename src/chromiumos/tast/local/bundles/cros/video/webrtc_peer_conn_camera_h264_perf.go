// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WebRTCPeerConnCameraH264Perf,
		Desc:     "Captures performance data about WebRTC loopback (H264)",
		Contacts: []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome_login", "chrome_internal"},
		Pre:          pre.ChromeVideo(),
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback_camera.html"),
	})
}

// WebRTCPeerConnCameraH264Perf is the full version of video.WebRTCPeerConnCameraH264.
// This test performs a WebRTC loopback call for 20 seconds.
// If there is no error while exercising the camera, it uploads statistics of
// black/frozen frames and input/output FPS will be logged.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func WebRTCPeerConnCameraH264Perf(ctx context.Context, s *testing.State) {
	// Run loopback call for 20 seconds.
	result := webrtc.RunWebRTCPeerConnCamera(ctx, s,
		s.PreValue().(*chrome.Chrome), videotype.H264, 20*time.Second)

	if !s.HasError() {
		// Set and upload perf metrics below.
		p := perf.NewValues()
		result.SetPerf(p, videotype.H264)
		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
