// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/constants"
	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

// streamFile is a y4m stream to be used as a fake camera stream.
const streamFile = "crowd720_25frames.y4m"

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCPerfDecodeTime,
		Desc:         "Measures WebRTC loopback performance",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         append(webrtc.DataFiles(), streamFile, constants.RTCLoopbackPage, constants.RTCAddStatsJs),
	})
}

// WebRTCPerfDecodeTime opens a WebRTC loopback page and loops a given capture stream
// to measure decode time.
func WebRTCPerfDecodeTime(ctx context.Context, s *testing.State) {
	webrtc.RunWebRTCPerf(ctx, s, streamFile)
}
