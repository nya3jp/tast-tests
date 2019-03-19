// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCCameraPerf,
		Desc:         "Captures performance data about getUserMedia video capture",
		Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{caps.AnyCamera, "chrome_login", "camera_720p"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// WebRTCCameraPerf is the full version of WebRTCCamera.
// It renders the camera's media stream in VGA and 720p for 20 seconds.
// If there is no error while exercising the camera, it uploads statistics of
// black/frozen frames.
// This test will fail when an error occurs or too many frames are broken.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func WebRTCCameraPerf(ctx context.Context, s *testing.State) {
	// Run tests for 20 seconds per resolution.
	results := webrtc.RunWebRTCCamera(ctx, s, 20*time.Second)

	if !s.HasError() {
		// Set and upload frame statistics below.
		p := perf.NewValues()
		results.SetPerf(p)
		if err := p.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}
}
