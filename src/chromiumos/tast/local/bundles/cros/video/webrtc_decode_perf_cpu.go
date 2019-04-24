// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebRTCDecodePerfCPU,
		Desc:         "Measures WebRTC decode performance in terms of CPU usage with and without hardware acceleration",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome_login"},
		Data:         append(webrtc.LoopbackDataFiles(), "crowd720_25frames.y4m", webrtc.AddStatsJSFile),
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 3 * time.Minute,
	})
}

// WebRTCDecodePerfCPU opens a WebRTC loopback page that loops a given capture stream
// to measure CPU usage.
func WebRTCDecodePerfCPU(ctx context.Context, s *testing.State) {
	webrtc.RunWebRTCDecodePerfCPU(ctx, s, "crowd720_25frames.y4m")
}
