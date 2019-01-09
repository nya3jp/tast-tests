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
		Func:         DecodeAccelSanityVP9ShowExistingFrame,
		Desc:         "Run Chrome video_decode_accelerator_unittest's NoCrash test on a VP9 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP9},
		Data:         []string{decode.DecodeAccelSanityVP9ShowExistingFrame.Name},
	})
}

// DecodeAccelSanityVP9ShowExistingFrame runs NoCrash test in video_decode_accelerator_unittest with
// video defined in decode.DecodeAccelSanityVP9ShowExistingFrame.
// TODO(crbug.com/900467): This test is failing on elm and hana due to driver issue.
func DecodeAccelSanityVP9ShowExistingFrame(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, decode.DecodeAccelSanityVP9ShowExistingFrame)
}
