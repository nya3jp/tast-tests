// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCEncodeAccelUsedVP8,
		Desc:         "Checks HW encoding used for WebRTC/VP8",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", caps.HWEncodeVP8},
		Data:         append(webrtc.DataFiles(), "crowd720_25frames.y4m", "loopback.html"),
	})
}

func WebRTCEncodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	webrtc.RunWebRTCVideo(ctx, s, "crowd720_25frames.y4m", constants.RTCVEInitStatus, constants.RTCVEInitSuccess)
}
