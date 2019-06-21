// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelVP8New,
		Desc:         "Run Chrome video_decode_accelerator_tests with an VP8 video",
		Contacts:     []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8},
		Data:         decode.DataFiles(videotype.VP8Prof),
	})
}

// DecodeAccelVP8New runs the video_decode_accelerator_tests with test-25fps.vp8.
// TODO(dstaessens): Drop the 'New' suffix when the old VDA tests have been deprecated.
func DecodeAccelVP8New(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, decode.Test25FPSVP8.Name)
}
