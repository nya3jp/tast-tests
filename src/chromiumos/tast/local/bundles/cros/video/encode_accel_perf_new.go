// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelPerfNew,
		Desc:         "Measures hardware video encode performance by running the video_encode_accelerator_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
		Data:    encode.TestData(encode.Tulip720P.Name),
		Params: []testing.Param{{
			Name: "h264_720p",
			Val: encode.TestOptionsNew{
				WebMName: encode.Tulip720P.Name,
				Profile:  videotype.H264Prof,
			},
			ExtraSoftwareDeps: []string{caps.HWEncodeH264},
		}},
	})
}

func EncodeAccelPerfNew(ctx context.Context, s *testing.State) {
	encode.RunNewAccelVideoPerfTest(ctx, s, s.Param().(encode.TestOptionsNew))
}
