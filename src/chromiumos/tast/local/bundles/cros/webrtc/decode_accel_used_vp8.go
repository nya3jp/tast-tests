// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/video"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelUsedVP8,
		Desc:         "Checks HW decoding used for WebRTC/VP8",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8},
		Data:         append(webrtc.LoopbackDataFiles(), "crowd720_25frames.y4m"),
	})
}

func DecodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	video.RunVideo(ctx, s, "crowd720_25frames.y4m", constants.RTCVDInitStatus, constants.RTCVDInitSuccess)
}
