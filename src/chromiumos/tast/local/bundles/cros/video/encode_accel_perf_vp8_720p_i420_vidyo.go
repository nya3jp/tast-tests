// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelPerfVP8720PI420Vidyo,
		Desc:         "Runs Chrome video_encode_accelerator_unittest to measure the performance of VP8 encoding for 720p I420 video vidyo1, which is more representative of a video-conference scenario",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{caps.HWEncodeVP8},
		Data:         []string{encode.Vidyo720P.Name},
	})
}

func EncodeAccelPerfVP8720PI420Vidyo(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoPerfTest(ctx, s, encode.TestOptions{
		Profile:     videotype.VP8Prof,
		Params:      encode.Vidyo720P,
		PixelFormat: videotype.I420,
		InputMode:   encode.SharedMemory,
	})
}
