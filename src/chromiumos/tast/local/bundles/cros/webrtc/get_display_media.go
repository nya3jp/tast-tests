// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/getdisplaymedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GetDisplayMedia,
		Desc: "Verifies that WebRTC getDisplayMedia() (screen, window, tab capture) works",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webrtc.DataFiles(), "getdisplaymedia.html"),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		// TODO(crbug.com/1017374): add "vp9_enc".
		Params: []testing.Param{{
			Name:              "monitor",
			Val:               "monitor",
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}, {
			Name:              "browser",
			Val:               "browser",
			Pre:               pre.ChromeVideoWithFakeWebcam(),
		}},
	})
}

// GetDisplayMedia verifies that the homonymous API works as expected.
func GetDisplayMedia(ctx context.Context, s *testing.State) {
	// TODO(mcasas): Experiment with `--auto-select-desktop-capture-source="Entire screen"`
	getdisplaymedia.RunGetDisplayMedia(ctx, s, s.PreValue().(*chrome.Chrome), s.Param().(string))
}
