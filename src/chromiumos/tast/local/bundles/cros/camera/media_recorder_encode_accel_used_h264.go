// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/mediarecorder"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderEncodeAccelUsedH264,
		Desc: "Checks H.264 video encode acceleration is used in MediaRecorder",
		Contacts: []string{
			"hiroh@chromium.org", // Video team
			"wtlee@chromium.org", // Camera team
			"chromeos-camera-eng@google.com",
		},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome", "chrome_internal", caps.HWEncodeH264},
		Data:         []string{"loopback_media_recorder.html"},
	})
}

func MediaRecorderEncodeAccelUsedH264(ctx context.Context, s *testing.State) {
	mediarecorder.VerifyEncodeAccelUsed(ctx, s, videotype.H264)
}
