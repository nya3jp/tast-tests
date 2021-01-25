// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
)

var av1Files = []string{
	"test_vectors/av1/00000548.ivf",
	"test_vectors/av1/48_delayed.ivf",
	"test_vectors/av1/av1-1-b8-02-allintra.ivf",
	"test_vectors/av1/av1-1-b8-03-sizeup.ivf",
	"test_vectors/av1/crosvideo_last_2sec.ivf",
	"test_vectors/av1/frames_refs_short_signaling.ivf",
	"test_vectors/av1/non_uniform_tiling.ivf",
	"test_vectors/av1/test-25fps-192x288-only-tile-cols-is-power-of-2.ivf",
	"test_vectors/av1/test-25fps-192x288-only-tile-rows-is-power-of-2.ivf",
	"test_vectors/av1/test-25fps-192x288-tile-rows-3-tile-cols-3.ivf",
}

var av1FilmGrainFiles = []string{
	"test_vectors/av1/av1-1-b8-23-film_grain-50.ivf",
	"test_vectors/av1/ccvb_film_grain.ivf",
}

func testFiles(videoFiles []string) []string {
	var tf []string
	for _, file := range videoFiles {
		tf = append(tf, file)
		tf = append(tf, file+".json")
	}
	return tf
}

// testOpt is used to describe the options used to run each test.
type testOpt struct {
	videoFiles       []string // The paths of video files to be tested.
	validateByVisual bool     // If this is true, video_decode_accelerator_tests runs with --validate_by_visual.
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
			ExtraData:         testFiles(av1Files),
			Val: testOpt{
				videoFiles:       av1Files,
				validateByVisual: false,
			},
		}, {
			Name:              "av1_film_grain_test_vectors",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         testFiles(av1FilmGrainFiles),
			Val: testOpt{
				videoFiles: av1FilmGrainFiles,
				// Decoded images in an av1 film grain stream can be different among decoders. (AV1 spec 7.2)
				// Therefore, we validates by visually not md5 checksum.
				validateByVisual: true,
			},
		}},
	})
}

func DecodeCompliance(ctx context.Context, s *testing.State) {
	var tv []string
	opt := s.Param().(testOpt)
	for _, file := range opt.videoFiles {
		tv = append(tv, s.DataPath(file))
	}

	if err := decoding.RunAccelVideoTestWithTestVectors(ctx, s.OutDir(), tv, opt.validateByVisual); err != nil {
		s.Fatal("test failed: ", err)
	}
}
