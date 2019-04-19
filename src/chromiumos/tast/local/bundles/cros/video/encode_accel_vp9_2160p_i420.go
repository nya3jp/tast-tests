// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EncodeAccelVP92160PI420,
		Desc:         "Runs Chrome video_encode_accelerator_unittest from 2160p I420 raw frames to VP9 stream",
		Contacts:     []string{"johnylin@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP9_4K},
		Data:         []string{encode.Crowd2160P.Name},
		Timeout:      4 * time.Minute,
	})
}

func EncodeAccelVP92160PI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTests(ctx, s, encode.TestOptions{
		Profile:     videotype.VP9Prof,
		Params:      encode.Crowd2160P,
		PixelFormat: videotype.I420,
		InputMode:   encode.SharedMemory,
	})
}
