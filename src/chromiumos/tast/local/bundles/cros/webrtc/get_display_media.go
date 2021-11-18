// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/getdisplaymedia"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GetDisplayMedia,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies that WebRTC getDisplayMedia() (screen, window, tab capture) works",
		Contacts: []string{
			"mcasas@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         getdisplaymedia.DataFiles(),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		// See https://w3c.github.io/mediacapture-screen-share/#displaycapturesurfacetype
		// for where the case names come from.
		// TODO(crbug.com/1063449): add other cases when the adequate precondition is ready.
		Params: []testing.Param{{
			Name:              "monitor",
			Val:               "monitor",
			Fixture:           "chromeScreenCapture",
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:    "window",
			Val:     "window",
			Fixture: "chromeWindowCapture",
		}},
	})
}

// GetDisplayMedia verifies that the homonymous API works as expected.
func GetDisplayMedia(ctx context.Context, s *testing.State) {
	if err := getdisplaymedia.RunGetDisplayMedia(ctx, s, s.FixtValue().(*chrome.Chrome), s.Param().(string)); err != nil {
		s.Fatal("TestPlay failed: ", err)
	}
}
