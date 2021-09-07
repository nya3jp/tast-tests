// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

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
			Val:  "canvas",
		}},
		//TODO(b/199174572): add a test case for "video" capture.
	})
}

// CaptureFromElement verifies that the homonymous API works as expected.
func CaptureFromElement(ctx context.Context, s *testing.State) {
	if err := capturefromelement.RunCaptureStream(ctx, s, s.FixtValue().(*chrome.Chrome)); err != nil {
		s.Fatal("RunCaptureStream failed: ", err)
	}
}
