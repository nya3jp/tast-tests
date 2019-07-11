// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         EncodeAccelPerfH2641080PI420,
		Desc:         "Runs Chrome video_encode_accelerator_unittest to measure the performance of H264 encoding for 1080p I420 video",
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeH264},
		Data:         []string{encode.Crowd1080P.Name},
		// Default timeout (i.e. 2 minutes) is not enough.
		Timeout: 10 * time.Minute,
	})
}

func EncodeAccelPerfH2641080PI420(ctx context.Context, s *testing.State) {
	encode.RunAccelVideoPerfTest(ctx, s, encode.TestOptions{
		Profile:     videotype.H264Prof,
		Params:      encode.Crowd1080P,
		PixelFormat: videotype.I420,
		InputMode:   encode.SharedMemory,
	})
}
