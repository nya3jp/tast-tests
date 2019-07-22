// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         DecodeAccelUsedJPEG,
		Desc:         "Checks HW decoding used for MJPEG in WebRTC",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeJPEG},
		Data:         append(common.LoopbackDataFiles(), "crowd720_25frames.mjpeg"),
	})
}

func DecodeAccelUsedJPEG(ctx context.Context, s *testing.State) {
	common.RunWebRTCVideo(ctx, s, "crowd720_25frames.mjpeg", constants.RTCJPEGInitStatus, constants.RTCJPEGInitSuccess)
}
