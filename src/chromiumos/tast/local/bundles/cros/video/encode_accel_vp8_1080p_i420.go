// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func:         EncodeAccelVP81080PI420,
		Desc:         "Run Chrome video_encode_accelerator_unittest from 1080p I420 raw frames to VP8 stream",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP8},
		Data:         []string{encode.Crowd1080P.Name},
		Timeout:      4 * time.Minute,
	})
}

// EncodeAccelVP81080PI420 runs video_encode_accelerator_unittest to encode VP8 encoding with 1080p I420 raw data compressed in crowd-1920x1080.webm.
func EncodeAccelVP81080PI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.VP8Prof, Params: encode.Crowd1080P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
