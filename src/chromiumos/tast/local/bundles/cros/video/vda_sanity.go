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

var testDataset = [...]decode.TestVideoData{
	decode.VDASanityBearProfile1VP9,
	decode.VDASanityBearProfile2VP9,
	decode.VDASanityBearProfile3VP9,
	decode.VDASanityVP90217ShowExistingFrame,
}

func init() {
	dataFiles := []string{}
	for _, testData := range testDataset {
		dataFiles = append(dataFiles, testData.Name)
	}
	testing.AddTest(&testing.Test{
		Func:         VDASanity,
		Desc:         "Run Chrome video_decode_accelerator_unittest with --gtest_filter=VideoDecodeAcceleratorTest.NoCrash on  a VP9 video",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{caps.HWDecodeVP9},
		Data:         dataFiles,
	})
}

// VDASanity runs video_decode_accelerator_unittest with
// --gtest_filter=VideoDecodeAcceleratorTest.NoCrash given tests in
// |testDataset|.
func VDASanity(ctx context.Context, s *testing.State) {
	for _, testData := range testDataset {
		decode.RunVDASanityTest(ctx, s, testData)
	}
}
