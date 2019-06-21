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
		Func:         DecodeAccelVP92New,
		Desc:         "Run Chrome video_decode_accelerator_tests with an VP9.2 video",
		Contacts:     []string{"acourbot@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9_2},
		Data:         decode.DataFiles(videotype.VP9_2Prof),
	})
}

// DecodeAccelVP92New runs the video_decode_accelerator_tests with test-25fps.vp9_2.
// TODO(dstaessens): Drop the 'New' suffix when the old VDA tests have been deprecated.
func DecodeAccelVP92New(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoTestNew(ctx, s, decode.Test25FPSVP92.Name)
}
