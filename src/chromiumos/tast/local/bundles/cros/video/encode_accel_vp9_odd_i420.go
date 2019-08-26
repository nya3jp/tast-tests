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
		Func:     EncodeAccelVP9OddI420,
		Desc:     "Runs Chrome video_encode_accelerator_unittest from 641x361 I420 raw frames to VP9 stream",
		Contacts: []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		// Disabled because an intel encoder driver always aligns a visible size by 16. (b/139846661)
		// TODO(b/139846661): Enable once an intel encoder driver issue is fixed.
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP9},
		Data:         []string{encode.Crowd361P.Name},
		Timeout:      4 * time.Minute,
	})
}

// EncodeAccelVP9OddI420 runs video_encode_accelerator_unittest to encode VP9 encoding with 641x361 I420 raw data compressed in crowd-641x361.webm.
// Odd in test name means width and height of raw data to be encoded is not divisible by 2.
func EncodeAccelVP9OddI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.VP9Prof, Params: encode.Crowd361P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
