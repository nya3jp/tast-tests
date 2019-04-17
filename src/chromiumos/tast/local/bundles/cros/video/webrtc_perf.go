// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCPerf,
		Desc:         "WebRTC loopback performance test",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{},
		Data:         append(webrtc.DataFiles(), "crowd720_25frames.y4m", "loopback.html"),
	})
}

// WebRTCPerf opens video/data/loopback.html and communicates via
// WebRTC in a fake way. The capture stream on WebRTC is streamFile.
// Noted if disableHardwareAcceleration is set, Chrome will be started
// with --disable-accelerated-video-decode.
func WebRTCPerf(ctx context.Context, s *testing.State) {
	webrtc.RunWebRTCPerf(ctx, s, "crowd720_25frames.y4m")
}
