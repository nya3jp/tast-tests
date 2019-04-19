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
		Func:         DecodeAccelSanityVP93,
		Desc:         "Run Chrome video_decode_accelerator_unittest's NoCrash test on a VP9.3 video",
		Contacts:     []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9},
		Data:         []string{decode.DecodeAccelSanityVP93.Name},
	})
}

// DecodeAccelSanityVP93 runs NoCrash test in video_decode_accelerator_unittest with video defined
// in decode.DecodeAccelSanityVP93.
func DecodeAccelSanityVP93(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, decode.DecodeAccelSanityVP93)
}
