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
		Func:     DecodeAccelSanityVP93,
		Desc:     "Run Chrome video_decode_accelerator_unittest's NoCrash test on a VP9.3 video",
		Contacts: []string{"deanliao@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "vp9_sanity" is a whitelist of devices that stay alive playing unsupported VP9 profile stream.
		// Currently RK3399 devices may crash playing the VP9 profile 3 stream, so they are excluded.
		// See crbug.com/971032 for detail.
		SoftwareDeps: []string{"chrome", caps.HWDecodeVP9, "vp9_sanity"},
		Data:         []string{decode.DecodeAccelSanityVP93.Name},
	})
}

// DecodeAccelSanityVP93 runs NoCrash test in video_decode_accelerator_unittest with video defined
// in decode.DecodeAccelSanityVP93.
func DecodeAccelSanityVP93(ctx context.Context, s *testing.State) {
	decode.RunAccelVideoSanityTest(ctx, s, decode.DecodeAccelSanityVP93)
}
