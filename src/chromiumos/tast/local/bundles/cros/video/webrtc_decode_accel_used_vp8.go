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
		Func:         WebRTCDecodeAccelUsedVP8,
		Desc:         "Check HW decoding used for WebRTC/vp8 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", caps.HWDecodeVP8},
		Data:         []string{"crowd720_25frames.y4m", "loopback.html"},
	})
}

func WebRTCDecodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	webrtc.RunWebRTCVideo(ctx, s, "crowd720_25frames.y4m", constants.RTCVDInitStatus, constants.RTCVDInitSuccess)
}
