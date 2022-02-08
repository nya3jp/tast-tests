// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/webrtc/capturefromelement"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureFromElementPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Collects performance values for WebRTC captureStream() (canvas, video)",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"hiroh@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         capturefromelement.DataFiles(),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name:    "canvas",
			Val:     capturefromelement.UseGlClearColor,
			Fixture: "chromeVideo",
		}, {
			Name:    "canvas_from_video",
			Val:     capturefromelement.UseVideo,
			Fixture: "chromeVideoWithFakeWebcam",
		}},
		//TODO(b/199174572): add a test case for "video" capture.
	})
}

// CaptureFromElementPerf collects perf metrics for the homonymous API.
func CaptureFromElementPerf(ctx context.Context, s *testing.State) {
	const measurementDuration = 25 * time.Second
	if err := capturefromelement.RunCaptureStream(ctx, s, s.FixtValue().(*chrome.Chrome), s.Param().(capturefromelement.CanvasSource), measurementDuration); err != nil {
		s.Fatal("RunCaptureStream failed: ", err)
	}
}
