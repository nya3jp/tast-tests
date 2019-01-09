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
		Func:         DecodeAccelSanityVP9BearProfile2,
		Desc:         "Run Chrome video_decode_accelerator_unittest's NoCrash test on a VP9 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP9},
		Data:         []string{decode.DecodeAccelSanityVP9BearProfile2.Name},
	})
}

// DecodeAccelSanityVP9BearProfile2 runs NoCrash test in video_decode_accelerator_unittest with
// video defined in decode.DecodeAccelSanityVP9BearProfile2.
func DecodeAccelSanityVP9BearProfile2(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, decode.DecodeAccelSanityVP9BearProfile2)
}
