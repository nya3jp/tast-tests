// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/bundles/cros/video/vea"
	"chromiumos/tast/testing"
)

var bearNV12 vea.StreamParam = vea.StreamParam{
	Name:    "bear_320x192_40frames.nv12.yuv",
	Width:   320,
	Height:  192,
	Bitrate: 200000,
	Format:  vea.NV12,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VeaTestH264NV12,
		Desc:         "Run Chrome video_encode_accelerator_unittest for H264",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWEncodeH264},
		Data:         []string{bearNV12.Name},
	})
}

func VeaTestH264NV12(ctx context.Context, s *testing.State) {
	vea.RunTest(ctx, s, vea.H264, bearNV12)
}
