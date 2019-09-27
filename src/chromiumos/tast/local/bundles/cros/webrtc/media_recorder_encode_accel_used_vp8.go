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
		Func: MediaRecorderEncodeAccelUsedVP8,
		Desc: "Verifies that VP8 video encode accelerator is used in MediaRecorder",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8},
		Data:         []string{"loopback_media_recorder.html"},
	})
}

func MediaRecorderEncodeAccelUsedVP8(ctx context.Context, s *testing.State) {
	mediarecorder.VerifyMediaRecorderUsesEncodeAccelerator(ctx, s, videotype.VP8)
}
