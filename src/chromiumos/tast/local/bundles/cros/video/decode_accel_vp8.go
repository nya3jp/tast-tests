// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelVP8,
		Desc:         "Run Chrome video_decode_accelerator_unittest with a VP8 video",
		Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP8},
		Data:         decode.DataFiles(videotype.VP8Prof, decode.AllocateBuffer),
		Timeout:      4 * time.Minute,
	})
}

// DecodeAccelVP8 runs video_decode_accelerator_unittest in ALLOCATE mode with test-25fps.vp8.
func DecodeAccelVP8(ctx context.Context, s *testing.State) {
	decode.RunAllAccelVideoTest(ctx, s, decode.Test25FPSVP8, decode.AllocateBuffer)
}
