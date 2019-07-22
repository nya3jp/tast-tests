// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/common"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/constants"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelUsedVP8,
		Desc:         "Checks HW encoding used for WebRTC/VP8",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8},
		Data:         append(common.LoopbackDataFiles(), "crowd720_25frames.y4m"),
	})
}

func EncodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	common.RunWebRTCVideo(ctx, s, "crowd720_25frames.y4m", constants.RTCVEInitStatus, constants.RTCVEInitSuccess)
}
