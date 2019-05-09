// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelH264ResolutionSwitch,
		Desc:         "Runs Chrome video_decode_accelerator_tests with an H.264 resolution switching video",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeH264},
		// TODO(crbug.com/933034) Use decode.DataFiles when the old VDA tests have been deprecated.
		Data: []string{"switch_1080p_720p_240frames.h264", "switch_1080p_720p_240frames.h264.json"},
	})
}

// DecodeAccelH264ResolutionSwitch runs the video_decode_accelerator_tests with switch_1080p_720p_240frames.h264.
func DecodeAccelH264ResolutionSwitch(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, "switch_1080p_720p_240frames.h264")
}
