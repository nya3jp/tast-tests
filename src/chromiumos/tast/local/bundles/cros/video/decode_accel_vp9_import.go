// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/lib/videotype"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DecodeAccelVP9Import,
		Desc: "Run Chrome video_decode_accelerator_unittest with a VP9 video",
		Attr: []string{"informational"},
		// VDA unittest cannot run with IMPORT mode on devices where ARC++ is disabled. (cf. crbug.com/881729)
		SoftwareDeps: []string{"android", caps.HWDecodeVP9},
		Data:         decode.DataFiles(videotype.VP9Prof, decode.ImportBuffer),
	})
}

// DecodeAccelVP9Import runs video_decode_accelerator_unittest in IMPORT mode with test-25fps.vp9.
func DecodeAccelVP9Import(ctx context.Context, s *testing.State) {
	decode.RunAllAccelVideoTestImportMode(ctx, s, decode.Test25FPSVP9)
}
