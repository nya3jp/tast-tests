// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCEncodeAccel,
		Desc:         "Verifies ARC++ hardware encode acceleration by running the arcvideoencoder_test binary",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(), // TODO(akahuang): Implement new precondition to boot ARC and enable verbose at chromium.
		Params: []testing.Param{{
			Name: "h264_192p_i420",
			Val: encode.TestOptions{
				Profile:     videotype.H264Prof,
				Params:      encode.Bear192P,
				PixelFormat: videotype.I420,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
			ExtraData:         []string{encode.Bear192P.Name},
		}},
	})
}

func ARCEncodeAccel(ctx context.Context, s *testing.State) {
	encode.RunARCVideoTest(ctx, s, s.PreValue().(arc.PreData).ARC, s.Param().(encode.TestOptions))
}
