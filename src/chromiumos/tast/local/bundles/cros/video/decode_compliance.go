// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
)

var videoTestFiles = []string{
	"test_vectors/av1/00000527.ivf",
	"test_vectors/av1/00000527.ivf.json",
	"test_vectors/av1/00000535.ivf",
	"test_vectors/av1/00000535.ivf.json",
	"test_vectors/av1/00000548.ivf",
	"test_vectors/av1/00000548.ivf.json",
	"test_vectors/av1/48_delayed.ivf",
	"test_vectors/av1/48_delayed.ivf.json",
	"test_vectors/av1/av1-1-b8-02-allintra.ivf",
	"test_vectors/av1/av1-1-b8-02-allintra.ivf.json",
	"test_vectors/av1/av1-1-b8-03-sizeup.ivf",
	"test_vectors/av1/av1-1-b8-03-sizeup.ivf.json",
	// TODO(b/176927551): Test with film grain streams if the intel driver fixes.
	//	"test_vectors/av1/av1-1-b8-23-film_grain-50.ivf",
	//	"test_vectors/av1/av1-1-b8-23-film_grain-50.ivf.json",
	//	"test_vectors/av1/ccvb_film_grain.ivf",
	//	"test_vectors/av1/ccvb_film_grain.ivf.json",
	"test_vectors/av1/crosvideo_last_2sec.ivf",
	"test_vectors/av1/crosvideo_last_2sec.ivf.json",
	"test_vectors/av1/frames_refs_short_signaling.ivf",
	"test_vectors/av1/frames_refs_short_signaling.ivf.json",
	"test_vectors/av1/non_uniform_tiling.ivf",
	"test_vectors/av1/non_uniform_tiling.ivf.json",
	"test_vectors/av1/test-25fps-192x288-only-tile-cols-is-power-of-2.ivf",
	"test_vectors/av1/test-25fps-192x288-only-tile-cols-is-power-of-2.ivf.json",
	"test_vectors/av1/test-25fps-192x288-only-tile-rows-is-power-of-2.ivf",
	"test_vectors/av1/test-25fps-192x288-only-tile-rows-is-power-of-2.ivf.json",
	"test_vectors/av1/test-25fps-192x288-tile-rows-3-tile-cols-3.ivf",
	"test_vectors/av1/test-25fps-192x288-tile-rows-3-tile-cols-3.ivf.json",
}

func testVectors() []string {
	var tv []string
	for _, file := range videoTestFiles {
		if filepath.Ext(file) == ".ivf" {
			tv = append(tv, file)
		}
	}
	return tv
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeCompliance,
		Desc:         "Verifies the result of decoding a variety of videos (i.e., test vectors) that target mostly specific codec features by running the video_decode_accelerator_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		SoftwareDeps: []string{"chrome", "video_decoder_direct"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "av1_test_vectors",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         videoTestFiles,
		}},
	})
}

func DecodeCompliance(ctx context.Context, s *testing.State) {
	var tv []string
	for _, file := range testVectors() {
		tv = append(tv, s.DataPath(file))
	}

	if err := decoding.RunAccelVideoTestWithTestVectors(ctx, s.OutDir(), tv); err != nil {
		s.Fatal("test failed: ", err)
	}
}
