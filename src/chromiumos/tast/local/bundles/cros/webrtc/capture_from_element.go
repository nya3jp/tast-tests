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
		Func: CaptureFromElement,
		Desc: "Verifies that WebRTC captureStream() (canvas, video) works",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         capturefromelement.DataFiles(),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Fixture:      "chromeVideo",
		Params: []testing.Param{{
			Name: "canvas",
			Val:  capturefromelement.UseGlClearColor,
		}},
		//TODO(b/199174572): add a test case for "video" capture.
	})
}

// CaptureFromElement verifies that the homonymous API works as expected.
func CaptureFromElement(ctx context.Context, s *testing.State) {
	const noMeasurement = 0 * time.Second
	if err := capturefromelement.RunCaptureStream(ctx, s, s.FixtValue().(*chrome.Chrome), s.Param().(capturefromelement.CanvasSource), noMeasurement); err != nil {
		s.Fatal("RunCaptureStream failed: ", err)
	}
}
