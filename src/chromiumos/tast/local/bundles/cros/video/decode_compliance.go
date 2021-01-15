// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"strconv"

	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/decoding"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var av1CommonFiles = []string{
	"test_vectors/av1/00000527.ivf",
	"test_vectors/av1/00000535.ivf",
	"test_vectors/av1/00000548.ivf",
	"test_vectors/av1/48_delayed.ivf",
	"test_vectors/av1/av1-1-b8-02-allintra.ivf",
	"test_vectors/av1/av1-1-b8-03-sizeup.ivf",
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

var av1Files = append(av1CommonFiles, av1FilmGrainFiles...)

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
	params := []testing.Param{{
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
	}}

	for i := 0; i < len(videoTestFiles10Bit); i += 10 {
		end := i + 10
		if end > len(videoTestFiles10Bit) {
			end = len(videoTestFiles10Bit)
		}
		videos := videoTestFiles10Bit[i:end]
		params = append(params,
			testing.Param{
				Name:              "av1_10bit_" + strconv.Itoa(i),
				ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
				ExtraData:         testFiles(videos),
				Val: decodeComplianceTestParam{
					videoFiles:    videos,
					validatorType: decoding.MD5,
				},
			})
	}
	testing.AddTest(&testing.Test{
		Func:         DecodeCompliance,
		Desc:         "Verifies the result of decoding a variety of videos (i.e., test vectors) that target mostly specific codec features by running the video_decode_accelerator_tests binary",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		SoftwareDeps: []string{"chrome", "video_decoder_direct"},
		Attr:         []string{"group:mainline", "informational"},
		Params:       params,
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

var videoTestFiles10Bit = []string{
	"test_vectors/av1/10-bit/data/00000671.ivf",
	"test_vectors/av1/10-bit/data/00000672.ivf",
	"test_vectors/av1/10-bit/data/00000673.ivf",
	"test_vectors/av1/10-bit/data/00000674.ivf",

	"test_vectors/av1/10-bit/data/00000675.ivf",
	"test_vectors/av1/10-bit/data/00000676.ivf",
	"test_vectors/av1/10-bit/data/00000677.ivf",
	"test_vectors/av1/10-bit/data/00000678.ivf",
	"test_vectors/av1/10-bit/data/00000679.ivf",
	"test_vectors/av1/10-bit/data/00000680.ivf",
	"test_vectors/av1/10-bit/data/00000681.ivf",
	"test_vectors/av1/10-bit/data/00000682.ivf",
	"test_vectors/av1/10-bit/data/00000683.ivf",
	"test_vectors/av1/10-bit/data/00000684.ivf",
	"test_vectors/av1/10-bit/data/00000685.ivf",
	"test_vectors/av1/10-bit/data/00000716.ivf",
	"test_vectors/av1/10-bit/data/00000717.ivf",
	"test_vectors/av1/10-bit/data/00000718.ivf",
	"test_vectors/av1/10-bit/data/00000719.ivf",
	"test_vectors/av1/10-bit/data/00000720.ivf",
	"test_vectors/av1/10-bit/data/00000721.ivf",
	"test_vectors/av1/10-bit/data/00000722.ivf",
	"test_vectors/av1/10-bit/data/00000723.ivf",
	"test_vectors/av1/10-bit/data/00000724.ivf",
	"test_vectors/av1/10-bit/data/00000725.ivf",
	"test_vectors/av1/10-bit/data/00000726.ivf",
	"test_vectors/av1/10-bit/data/00000727.ivf",
	"test_vectors/av1/10-bit/data/00000728.ivf",
	"test_vectors/av1/10-bit/data/00000729.ivf",
	"test_vectors/av1/10-bit/data/00000730.ivf",
	"test_vectors/av1/10-bit/data/00000761.ivf",
	"test_vectors/av1/10-bit/data/00000762.ivf",
	"test_vectors/av1/10-bit/data/00000763.ivf",
	"test_vectors/av1/10-bit/data/00000764.ivf",
	"test_vectors/av1/10-bit/data/00000765.ivf",
	"test_vectors/av1/10-bit/data/00000766.ivf",
	"test_vectors/av1/10-bit/data/00000767.ivf",
	"test_vectors/av1/10-bit/data/00000768.ivf",
	"test_vectors/av1/10-bit/data/00000769.ivf",
	"test_vectors/av1/10-bit/data/00000770.ivf",
	"test_vectors/av1/10-bit/data/00000771.ivf",
	"test_vectors/av1/10-bit/data/00000772.ivf",
	"test_vectors/av1/10-bit/data/00000773.ivf",
	"test_vectors/av1/10-bit/data/00000774.ivf",
	"test_vectors/av1/10-bit/data/00000775.ivf",
	"test_vectors/av1/10-bit/data/00000820.ivf",
	"test_vectors/av1/10-bit/data/00000821.ivf",
	"test_vectors/av1/10-bit/data/00000822.ivf",
	"test_vectors/av1/10-bit/data/00000823.ivf",
	"test_vectors/av1/10-bit/data/00000824.ivf",
	"test_vectors/av1/10-bit/data/00000825.ivf",
	"test_vectors/av1/10-bit/data/00000826.ivf",
	"test_vectors/av1/10-bit/data/00000827.ivf",
	"test_vectors/av1/10-bit/data/00000828.ivf",
	"test_vectors/av1/10-bit/data/00000829.ivf",
	"test_vectors/av1/10-bit/data/00000830.ivf",
	"test_vectors/av1/10-bit/data/00000831.ivf",
	"test_vectors/av1/10-bit/data/00000832.ivf",
	"test_vectors/av1/10-bit/data/00000833.ivf",
	"test_vectors/av1/10-bit/data/00000834.ivf",
	"test_vectors/av1/10-bit/data/00000835.ivf",
	"test_vectors/av1/10-bit/data/00000836.ivf",
	"test_vectors/av1/10-bit/data/00000837.ivf",
	"test_vectors/av1/10-bit/data/00000937.ivf",
	"test_vectors/av1/10-bit/data/00000938.ivf",
	"test_vectors/av1/10-bit/data/00000939.ivf",
	"test_vectors/av1/10-bit/data/00000940.ivf",
	"test_vectors/av1/10-bit/data/00000941.ivf",
	"test_vectors/av1/10-bit/data/00000942.ivf",
	"test_vectors/av1/10-bit/data/00000943.ivf",
	"test_vectors/av1/10-bit/data/00000944.ivf",
	"test_vectors/av1/10-bit/film_grain/av1-1-b10-23-film_grain-50.ivf",
	"test_vectors/av1/10-bit/issues/318_tx_4x4.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-00.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-01.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-02.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-03.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-04.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-05.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-06.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-07.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-08.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-09.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-10.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-11.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-12.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-13.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-14.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-15.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-16.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-17.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-18.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-19.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-20.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-21.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-22.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-23.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-24.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-25.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-26.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-27.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-28.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-29.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-30.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-31.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-32.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-33.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-34.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-35.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-36.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-37.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-38.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-39.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-40.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-41.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-42.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-43.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-44.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-45.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-46.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-47.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-48.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-49.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-50.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-51.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-52.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-53.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-54.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-55.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-56.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-57.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-58.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-59.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-60.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-61.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-62.ivf",
	"test_vectors/av1/10-bit/quantizer/av1-1-b10-00-quantizer-63.ivf",
}
