// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelVDH264ResolutionSwitch,
		Desc:         "Runs Chrome video_decode_accelerator_tests with an H.264 resolution switching video on a media::VideoDecoder (see go/vd-migration)",
		Contacts:     []string{"dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeH264},
		Data:         []string{"switch_1080p_720p_240frames.h264", "switch_1080p_720p_240frames.h264.json"},
	})
}

func DecodeAccelVDH264ResolutionSwitch(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, "switch_1080p_720p_240frames.h264", decode.VD)
}
