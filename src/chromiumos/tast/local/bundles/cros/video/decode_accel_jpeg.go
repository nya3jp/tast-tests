// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

// testFileList lists the files required by the JPEG decode accelerator test.
var testFileList = []string{"peach_pi-1280x720.jpg", "peach_pi-40x23.jpg",
	"peach_pi-41x22.jpg", "peach_pi-41x23.jpg", "pixel-1280x720.jpg"}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelJpeg,
		Desc:         "Run Chrome jpeg_decode_accelerator_unittest",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeJPEG},
		Data:         testFileList,
	})
}

// DecodeAccelJpeg runs a set of HW JPEG decode tests, defined in
// jpeg_decode_accelerator_unittest.
func DecodeAccelJpeg(ctx context.Context, s *testing.State) {
	decode.RunAccelJpegTest(ctx, s, "DecodeAccelJpeg", testFileList)
}
