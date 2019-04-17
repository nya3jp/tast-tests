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
		Func:         WebRTCDecodePerf,
		Desc:         "Measures WebRTC decode performance in terms of CPU usage and decode time with and without hardware acceleration",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         append(webrtc.LoopbackDataFiles(), y4mCameraStreamFile, webrtc.AddStatsJSFile),
	})
}

// y4mCameraStreamFile is a y4m stream to be used as a fake camera stream.
const y4mCameraStreamFile = "crowd720_25frames.y4m"

// WebRTCDecodePerf opens a WebRTC loopback page that loops a given capture stream
// to measure decode time and CPU usage.
func WebRTCDecodePerf(ctx context.Context, s *testing.State) {
	webrtc.RunWebRTCDecodePerf(ctx, s, y4mCameraStreamFile)
}
