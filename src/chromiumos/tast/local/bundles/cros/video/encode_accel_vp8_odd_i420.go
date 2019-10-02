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
		Func:         EncodeAccelVP8OddI420,
		Desc:         "Runs Chrome video_encode_accelerator_unittest from 641x361 I420 raw frames to VP8 stream",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8},
		Data:         []string{encode.Crowd361P.Name},
		Timeout:      4 * time.Minute,
	})
}

// EncodeAccelVP8OddI420 runs video_encode_accelerator_unittest to encode VP8 encoding with 641x361 I420 raw data compressed in crowd-641x361.webm.
// Odd in test name means width and height of raw data to be encoded is not divisible by 2.
func EncodeAccelVP8OddI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.VP8Prof, Params: encode.Crowd361P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
