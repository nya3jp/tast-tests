// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import (
	"context"

	"chromiumos/tast/local/bundles/cros/webrtc/mediarecorder"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MediaRecorderEncodeAccelUsedH264,
		Desc: "Verifies that H.264 video encode accelerator is used in MediaRecorder",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome", "chrome_internal", caps.HWEncodeH264},
		Data:         []string{"loopback_media_recorder.html"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func MediaRecorderEncodeAccelUsedH264(ctx context.Context, s *testing.State) {
	mediarecorder.VerifyMediaRecorderUsesEncodeAccelerator(ctx, s, videotype.H264)
}
