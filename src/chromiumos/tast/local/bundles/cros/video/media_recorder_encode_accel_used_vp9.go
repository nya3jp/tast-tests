// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/mediarecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaRecorderEncodeAccelUsedVP9,
		Desc:         "Checks VP9 video encode acceleration is used in MediaRecorder",
		Contacts:     []string{"shenghao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", caps.HWEncodeVP9},
		Data:         []string{"loopback_media_recorder.html"},
	})
}

func MediaRecorderEncodeAccelUsedVP9(ctx context.Context, s *testing.State) {
	mediarecorder.VerifyEncodeAccelUsed(ctx, s, "vp9")
}
