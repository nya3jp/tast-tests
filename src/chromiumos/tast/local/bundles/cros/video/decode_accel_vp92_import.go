// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DecodeAccelVP92Import,
		Desc:     "Run Chrome video_decode_accelerator_unittest with a VP9.2 video",
		Contacts: []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// VDA unittest cannot run with IMPORT mode on devices where ARC++ is disabled. (cf. crbug.com/881729)
		SoftwareDeps: []string{"chrome", "android", caps.HWDecodeVP9_2},
		Data:         decode.DataFiles(videotype.VP9_2Prof, decode.ImportBuffer),
		Timeout:      4 * time.Minute,
	})
}

// DecodeAccelVP92Import runs video_decode_accelerator_unittest in IMPORT mode with test-25fps.vp9_2.
func DecodeAccelVP92Import(ctx context.Context, s *testing.State) {
	decode.RunAllAccelVideoTest(ctx, s, decode.Test25FPSVP92, decode.ImportBuffer)
}
