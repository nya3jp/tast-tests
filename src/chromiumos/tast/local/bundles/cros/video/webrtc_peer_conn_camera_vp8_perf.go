// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCPeerConnCameraVP8Perf,
		Desc:         "Captures performance data about WebRTC loopback (VP8)",
		Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{caps.BuiltinCamera, "chrome_login"},
		Pre:          chrome.LoggedInVideo(),
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback_camera.html"),
	})
}

// WebRTCPeerConnCameraVP8Perf is the full version of video.WebRTCPeerConnCameraVP8.
// This test performs a WebRTC loopback call for 20 seconds.
// If there is no error while exercising the camera, it uploads statistics of
// black/frozen frames and input/output FPS will be logged.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func WebRTCPeerConnCameraVP8Perf(ctx context.Context, s *testing.State) {
	// Run loopback call for 20 seconds.
	result := webrtc.RunWebRTCPeerConnCamera(ctx, s,
		s.PreValue().(*chrome.Chrome), videotype.VP8, 20*time.Second)

	if !s.HasError() {
		// Set and upload perf metrics below.
		p := perf.NewValues()
		result.SetPerf(p, videotype.VP8)
		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
