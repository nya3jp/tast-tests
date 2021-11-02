// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/webcodecs"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebCodecsDecode,
		Desc: "Verifies that WebCodecs API works, maybe verifying use of a hardware accelerator",
		Contacts: []string{
			"hiroh@chromium.org", // Test author.
			"chromeos-gfx-video@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         append(webcodecs.DecodeDataFiles(), webcodecs.MP4DemuxerDataFiles()...),
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Fixture:      "chromeWebCodecs",
		Params: []testing.Param{{
			Name:              "h264_sw",
			Val:               webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.h264.mp4", Acceleration: webcodecs.PreferSoftware},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
		}, {
			Name:              "h264_hw",
			Val:               webcodecs.TestDecodeArgs{VideoFile: "bear-320x240.h264.mp4", Acceleration: webcodecs.PreferHardware},
			ExtraSoftwareDeps: []string{"proprietary_codecs", caps.HWDecodeH264},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
		}},
	})
}

func WebCodecsDecode(ctx context.Context, s *testing.State) {
	args := s.Param().(webcodecs.TestDecodeArgs)
	if err := webcodecs.RunDecodeTest(ctx, s.FixtValue().(*chrome.Chrome),
		s.DataFileSystem(), args); err != nil {
		s.Error("Test failed: ", err)
	}
}
