// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/constants"
	// TODO(crbug.com/971922): Remove /media/webrtc package.
	mediaWebRTC "chromiumos/tast/local/media/webrtc"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCEncodeAccelUsedVP8,
		Desc:         "Checks HW encoding used for WebRTC/VP8",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8},
		Data:         append(webrtc.LoopbackDataFiles(), "crowd720_25frames.y4m"),
	})
}

func WebRTCEncodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	mediaWebRTC.RunWebRTCVideo(ctx, s, "crowd720_25frames.y4m", constants.RTCVEInitStatus, constants.RTCVEInitSuccess)
}
