// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var av1CommonFiles = []string{
	"test_vectors/av1/8-bit/00000527.ivf",
	"test_vectors/av1/8-bit/00000535.ivf",
	"test_vectors/av1/8-bit/00000548.ivf",
	"test_vectors/av1/8-bit/48_delayed.ivf",
	"test_vectors/av1/8-bit/av1-1-b8-02-allintra.ivf",
	"test_vectors/av1/8-bit/av1-1-b8-03-sizeup.ivf",
	"test_vectors/av1/8-bit/frames_refs_short_signaling.ivf",
	"test_vectors/av1/8-bit/non_uniform_tiling.ivf",
	"test_vectors/av1/8-bit/test-25fps-192x288-only-tile-cols-is-power-of-2.ivf",
	"test_vectors/av1/8-bit/test-25fps-192x288-only-tile-rows-is-power-of-2.ivf",
	"test_vectors/av1/8-bit/test-25fps-192x288-tile-rows-3-tile-cols-3.ivf",
}

var av1FilmGrainFiles = []string{
	"test_vectors/av1/8-bit/av1-1-b8-23-film_grain-50.ivf",
	"test_vectors/av1/8-bit/ccvb_film_grain.ivf",
}

var av1Files = append(av1CommonFiles, av1FilmGrainFiles...)

var av110BitCommonFiles = []string{
	"test_vectors/av1/10-bit/00000671.ivf",
	"test_vectors/av1/10-bit/00000672.ivf",
	"test_vectors/av1/10-bit/00000673.ivf",
	"test_vectors/av1/10-bit/00000674.ivf",
	"test_vectors/av1/10-bit/00000675.ivf",
	"test_vectors/av1/10-bit/00000716.ivf",
	"test_vectors/av1/10-bit/00000717.ivf",
	"test_vectors/av1/10-bit/00000718.ivf",
	"test_vectors/av1/10-bit/00000719.ivf",
	"test_vectors/av1/10-bit/00000720.ivf",
	"test_vectors/av1/10-bit/00000761.ivf",
	"test_vectors/av1/10-bit/00000762.ivf",
	"test_vectors/av1/10-bit/00000763.ivf",
	"test_vectors/av1/10-bit/00000764.ivf",
	"test_vectors/av1/10-bit/00000765.ivf",
	"test_vectors/av1/10-bit/av1-1-b10-00-quantizer-00.ivf",
	"test_vectors/av1/10-bit/av1-1-b10-00-quantizer-10.ivf",
	"test_vectors/av1/10-bit/av1-1-b10-00-quantizer-20.ivf",
	"test_vectors/av1/10-bit/av1-1-b10-00-quantizer-30.ivf",
	"test_vectors/av1/10-bit/av1-1-b10-00-quantizer-40.ivf",
	"test_vectors/av1/10-bit/av1-1-b10-00-quantizer-50.ivf",
	"test_vectors/av1/10-bit/av1-1-b10-00-quantizer-60.ivf",
}

var av110BitFilmGrainFiles = []string{
	"test_vectors/av1/10-bit/av1-1-b10-23-film_grain-50.ivf",
}

var av110BitFiles = append(av110BitCommonFiles, av110BitFilmGrainFiles...)

func testFiles(videoFiles []string) []string {
	var tf []string
	for _, file := range videoFiles {
		tf = append(tf, file)
		tf = append(tf, file+".json")
	}
	return tf
}

// testOpt is used to describe the options used to run each test.
type decodeComplianceTestParam struct {
	videoFiles    []string               // The paths of video files to be tested.
	validatorType decoding.ValidatorType // The frame validation type of video_decode_accelerator_tests.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DecodeCompliance,
		Desc:         "Verifies the result of decoding a variety of videos (i.e., test vectors) that target mostly specific codec features by running the video_decode_accelerator_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		SoftwareDeps: []string{"chrome", "video_decoder_direct"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "av1_common",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         testFiles(av1CommonFiles),
			Val: decodeComplianceTestParam{
				videoFiles:    av1CommonFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_film_grain",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			// Different decoders may use different film grain synthesis methods while producing
			// a visually correct output (AV1 spec 7.2). Thus, for volteer, we don't validate
			// the decoding of film-grain streams using MD5. Instead, we validate them using
			// SSIM (see the av1_ssim test).
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("volteer")),
			ExtraData:         testFiles(av1FilmGrainFiles),
			Val: decodeComplianceTestParam{
				videoFiles:    av1FilmGrainFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_ssim",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         testFiles(av1Files),
			Val: decodeComplianceTestParam{
				videoFiles:    av1Files,
				validatorType: decoding.SSIM,
			},
		}, {
			Name:              "av1_10bit_common",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
			ExtraData:         testFiles(av110BitCommonFiles),
			Val: decodeComplianceTestParam{
				videoFiles:    av110BitCommonFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_10bit_film_grain",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
			// Different decoders may use different film grain synthesis methods while producing
			// a visually correct output (AV1 spec 7.2). Thus, for volteer, we don't validate
			// the decoding of film-grain streams using MD5. Instead, we validate them using
			// SSIM (see the av1_10bit_ssim test).
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("volteer")),
			ExtraData:         testFiles(av110BitFilmGrainFiles),
			Val: decodeComplianceTestParam{
				videoFiles:    av110BitFilmGrainFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_10bit_ssim",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
			ExtraData:         testFiles(av110BitFiles),
			Val: decodeComplianceTestParam{
				videoFiles:    av110BitFiles,
				validatorType: decoding.SSIM,
			},
		}},
	})
}

func DecodeCompliance(ctx context.Context, s *testing.State) {
	var tv []string
	param := s.Param().(decodeComplianceTestParam)
	for _, file := range param.videoFiles {
		tv = append(tv, s.DataPath(file))
	}

	if err := decoding.RunAccelVideoTestWithTestVectors(ctx, s.OutDir(), tv, param.validatorType); err != nil {
		s.Fatal("test failed: ", err)
	}
}
