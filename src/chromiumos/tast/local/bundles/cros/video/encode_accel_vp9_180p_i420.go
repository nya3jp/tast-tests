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
		Func:         EncodeAccelVP9180PI420,
		Desc:         "Runs Chrome video_encode_accelerator_unittest from 180p I420 raw frames to VP9 stream",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeVP9},
		Data:         []string{encode.Tulip180P.Name},
	})
}

// EncodeAccelVP9180PI420 runs video_encode_accelerator_unittest to encode VP9 encoding with 180p I420 raw data compressed in tulip2-320x180.webm.
func EncodeAccelVP9180PI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{Profile: videotype.VP9Prof, Params: encode.Tulip180P, PixelFormat: videotype.I420, InputMode: encode.SharedMemory})
}
