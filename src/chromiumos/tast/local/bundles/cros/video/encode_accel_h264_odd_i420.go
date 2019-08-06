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
		Func:         EncodeAccelH264OddI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from 641x361 I420 raw frames to H264 stream",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeH264},
		Data:         []string{encode.Crowd361P.Name},
		Timeout:      4 * time.Minute,
	})
}

// EncodeAccelH264OddI420 runs video_encode_accelerator_unittest to encode H264 encoding with 641x361 I420 raw data compressed in crowd-641x361.webm.
func EncodeAccelH264OddI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.H264Prof, Params: encode.Crowd361P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
