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
		Func: MediaRecorderEncodeAccelUsedVP9,
		Desc: "Checks VP9 video encode acceleration is used in MediaRecorder",
		Contacts: []string{
			"hiroh@chromium.org", // Video team
			"wtlee@chromium.org", // Camera team
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP9},
		Data:         []string{"loopback_media_recorder.html"},
	})
}

func MediaRecorderEncodeAccelUsedVP9(ctx context.Context, s *testing.State) {
	mediarecorder.VerifyEncodeAccelUsed(ctx, s, videotype.VP9)
}
