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
		Func:         EncodeAccelVP9360PI420,
		Desc:         "Runs Chrome video_encode_accelerator_unittest from 360p I420 raw frames to VP9 stream",
		Contacts:     []string{"akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.HWEncodeVP9},
		Data:         []string{encode.Tulip360P.Name},
		Timeout:      4 * time.Minute,
	})
}

func EncodeAccelVP9360PI420(ctx context.Context, s *testing.State) {
	encode.RunAllAccelVideoTestsWithFilter(ctx, s, encode.TestOptions{
		Profile:     videotype.VP9Prof,
		Params:      encode.Tulip360P,
		PixelFormat: videotype.I420,
		InputMode:   encode.SharedMemory},
		// Currently the intel driver cannot set the bitrate at VP9 correctly. Disable these test cases first.
		// TODO(b/134538840): Remove after the driver is fixed.
		encode.BitrateTestFilter)
}
