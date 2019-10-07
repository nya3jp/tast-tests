// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PeerConnH264Perf,
		Desc: "Captures performance data about WebRTC loopback (H264)",
		Contacts: []string{
			"keiichiw@chromium.org", // Video team
			"shik@chromium.org",     // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr: []string{"group:crosbolt", "crosbolt_perbuild"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{caps.BuiltinOrVividCamera, "chrome", "chrome_internal"},
		Pre:          pre.ChromeCameraPerf(),
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback_camera.html"),
	})
}

// PeerConnH264Perf is the full version of webrtc.PeerConnH264. This
// test performs a WebRTC loopback call for 20 seconds. If there is no error
// while exercising the camera, it uploads statistics of black/frozen frames and
// input/output FPS will be logged.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func PeerConnH264Perf(ctx context.Context, s *testing.State) {
	// Run loopback call for 20 seconds.
	result := webrtc.RunPeerConn(ctx, s,
		s.PreValue().(*chrome.Chrome), videotype.H264, 20*time.Second,
		webrtc.NoVerboseLogging)

	if !s.HasError() {
		// Set and upload perf metrics below.
		p := perf.NewValues()
		result.SetPerf(p, videotype.H264)
		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
