// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	dataFiles := []string{}
	for _, testData := range decodeAccelSanityVP9Data {
		dataFiles = append(dataFiles, testData.Name)
	}
	testing.AddTest(&testing.Test{
		Func:         DecodeAccelSanityVP9,
		Desc:         "Run Chrome video_decode_accelerator_unittest's NoCrash test on VP9 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP9},
		Data:         dataFiles,
	})
}

var decodeAccelSanityVP9Data = []decode.TestVideoData{
	decode.DecodeAccelSanityVP9BearProfile1,
	decode.DecodeAccelSanityVP9BearProfile2,
	decode.DecodeAccelSanityVP9BearProfile3,
	decode.DecodeAccelSanityVP9ShowExistingFrame,
}

// DecodeAccelSanityVP9 runs NoCrash test in video_decode_accelerator_unittest with input
// video defined in |decodeAccelSanityVP9Data|.
// TODO(crbug.com/900467): This test is failing on elm and hana due to driver issue.
func DecodeAccelSanityVP9(ctx context.Context, s *testing.State) {
	for _, testData := range decodeAccelSanityVP9Data {
		decode.RunAccelVideoTest(ctx, s,
			decode.TestArgument{TestData: testData,
				BufferMode: decode.AllocateBuffer,
				TestFilter: "VideoDecodeAcceleratorTest.NoCrash"})
	}
}
