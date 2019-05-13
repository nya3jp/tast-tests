// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/local/bundles/cros/video/mediarecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderEncodeAccelUsedH264,
		Desc: "Checks H.264 video encode acceleration is used in MediaRecorder",
		Contacts: []string{
			"hiroh@chromium.org",    // Video team
			"shenghao@chromium.org", // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr: []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome_login", "chrome_internal", caps.HWEncodeH264},
		Data:         []string{"loopback_media_recorder.html"},
	})
}

func MediaRecorderEncodeAccelUsedH264(ctx context.Context, s *testing.State) {
	mediarecorder.VerifyEncodeAccelUsed(ctx, s, videotype.H264)
}
