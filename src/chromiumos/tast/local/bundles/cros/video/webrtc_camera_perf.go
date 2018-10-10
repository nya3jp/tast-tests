// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"

	"fmt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCCameraPerf,
		Desc:         "Captures performance data about getUserMedia video capture",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "autotest-capability:usb_camera"},
		Data:         append(webrtc.DataFiles(), "getusermedia.html"),
	})
}

// WebRTCCameraPerf is the full version of WebRTCCamera.
// It renders the camera's media stream in VGA and 720p for 20 seconds.
// If there is no error while exercising the camera, it uploads statics of
// black/frozen frames.
// This test will fail when an error occurs or too many frames are broken.
//
// This test uses the real webcam unless it is running under QEMU. Under QEMU,
// it uses "vivid" instead, which is the virtual video test driver and can be
// used as an external USB camera.
func WebRTCCameraPerf(ctx context.Context, s *testing.State) {
	// Run tests for 20 seconds per resolution.
	var results []webrtc.WebRTCCameraResult
	webrtc.RunTest(ctx, s, "getusermedia.html", "testNextResolution(20)", &results)

	s.Logf("Results: %#v", results)

	// Set and upload frame statistics below
	p := &perf.Values{}

	for _, result := range results {

		if len(result.Errors) != 0 {
			for _, msg := range result.Errors {
				s.Errorf("%dx%d: %s", result.Width, result.Height, msg)
			}
			continue
		}

		// If the percentage of broken frames is more than 1%, the test will fail.
		// Regardless of the percentage, statistics will be uploaded.
		if err := result.FrameStats.VideoHealthCheck(0.01); err != nil {
			s.Errorf("Video was not healthy for %dx%d: %v",
				result.Width, result.Height, err)
		}

		blackFrames := perf.Metric{
			Name:      fmt.Sprintf("tast_black_frames_percentage_%dx%d", result.Width, result.Height),
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}
		frozenFrames := perf.Metric{
			Name:      fmt.Sprintf("tast_frozen_frames_percentage_%dx%d", result.Width, result.Height),
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}

		p.Set(blackFrames, result.FrameStats.BlackFramesRatio())
		p.Set(frozenFrames, result.FrameStats.FrozenFramesRatio())

	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
