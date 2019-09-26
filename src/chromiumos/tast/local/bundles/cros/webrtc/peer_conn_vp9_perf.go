// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PeerConnVP9Perf,
		Desc: "Captures performance data about WebRTC loopback (VP9)",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.ChromeFakeCameraPerf(),
		Data:         append(webrtc.DataFiles(), "third_party/munge_sdp.js", "loopback_camera.html"),
	})
}

// PeerConnVP9Perf is the performance-collection version of webrtc.PeerConnVP9.
// This test performs a WebRTC loopback call for 20 seconds. If there is no
// error while exercising the camera, it uploads statistics of black/frozen
// frames and input/output FPS will be logged.
func PeerConnVP9Perf(ctx context.Context, s *testing.State) {
	// Run loopback call for 20 seconds.
	result := webrtc.RunPeerConn(ctx, s,
		s.PreValue().(*chrome.Chrome), videotype.VP9,
		20*time.Second, webrtc.NoVerboseLogging)

	if s.HasError() {
		return
	}

	// Set and upload perf metrics below.
	p := perf.NewValues()
	result.SetPerf(p, videotype.VP9)
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
