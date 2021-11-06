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

var h264FilesFromBugs = []string{
	"test_vectors/h264/b_149068426_invalid_video_layout_mtk_8183_with_direct_videodecoder.h264",
	"test_vectors/h264/b_172838252_pixelated_video_on_rk3399.h264",
	"test_vectors/h264/b_174733646_video_with_out_of_order_frames_mtk_8173.h264",
}

var vp8ComprehensiveFiles = []string{
	"test_vectors/vp8/vp80-00-comprehensive-001.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-002.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-003.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-004.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-005.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-007.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-008.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-009.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-010.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-011.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-012.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-013.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-015.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-016.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-017.ivf",
	"test_vectors/vp8/vp80-00-comprehensive-018.ivf",
}

var vp8InterFiles = []string{
	"test_vectors/vp8/inter/vp80-02-inter-1402.ivf",
	"test_vectors/vp8/inter/vp80-02-inter-1424.ivf",
	"test_vectors/vp8/inter/vp80-02-inter-1418.ivf",
	"test_vectors/vp8/inter/vp80-02-inter-1412.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1442.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1436.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1432.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1427.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1426.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1435.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1403.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1425.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1441.ivf",
	"test_vectors/vp8/inter/vp80-03-segmentation-1437.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1434.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1430.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1443.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1439.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1428.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1438.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1431.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1440.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1433.ivf",
	"test_vectors/vp8/inter/vp80-05-sharpness-1429.ivf",
}

var vp8InterMultiCoeffFiles = []string{
	"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1409.ivf",
	"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1408.ivf",
	"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1410.ivf",
	"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1413.ivf",
	"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1405.ivf",
	"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1406.ivf",
	"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1404.ivf",
}

var vp8InterSegmentFiles = []string{
	"test_vectors/vp8/inter_segment/vp80-03-segmentation-1407.ivf",
}

var vp8IntraFiles = []string{
	"test_vectors/vp8/intra/vp80-01-intra-1416.ivf",
	"test_vectors/vp8/intra/vp80-01-intra-1417.ivf",
	"test_vectors/vp8/intra/vp80-01-intra-1411.ivf",
	"test_vectors/vp8/intra/vp80-01-intra-1400.ivf",
	"test_vectors/vp8/intra/vp80-03-segmentation-1401.ivf",
}

var vp8IntraMultiCoeffSegmentFiles = []string{
	"test_vectors/vp8/intra_multi_coeff/vp80-03-segmentation-1414.ivf",
}

var vp8IntraSegmentFiles = []string{
	"test_vectors/vp8/intra_segment/vp80-03-segmentation-1415.ivf",
}

func appendJSONFiles(videoFiles []string) []string {
	var tf []string
	for _, file := range videoFiles {
		tf = append(tf, file)
		tf = append(tf, file+".json")
	}
	return tf
}

// chromeStackDecodingTestParam is used to describe the options used to run each test.
type chromeStackDecodingTestParam struct {
	videoFiles    []string               // The paths of video files to be tested.
	validatorType decoding.ValidatorType // The frame validation type of video_decode_accelerator_tests.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeStackDecoding,
		Desc:         "Verifies video decoding using Chrome's stack (via the video_decode_accelerator_tests binary) and either MD5 or SSIM criteria",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-gfx-video@google.com"},
		SoftwareDeps: []string{"chrome", "video_decoder_direct"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "av1_common",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         appendJSONFiles(av1CommonFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    av1CommonFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_film_grain",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			// Different decoders may use different film grain synthesis methods while
			// producing a visually correct output (AV1 spec 7.2). Thus, for volteer,
			// don't validate the decoding of film-grain streams using MD5. Instead,
			// validate them using SSIM (see the av1_ssim test).
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("volteer")),
			ExtraData:         appendJSONFiles(av1FilmGrainFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    av1FilmGrainFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_ssim",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			ExtraData:         appendJSONFiles(av1Files),
			Val: chromeStackDecodingTestParam{
				videoFiles:    av1Files,
				validatorType: decoding.SSIM,
			},
		}, {
			Name:              "av1_10bit_common",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
			ExtraData:         appendJSONFiles(av110BitCommonFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    av110BitCommonFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_10bit_film_grain",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
			// Different decoders may use different film grain synthesis methods while
			// producing a visually correct output (AV1 spec 7.2). Thus, for volteer,
			// don't validate the decoding of film-grain streams using MD5. Instead,
			// validate them using SSIM (see the av1_10bit_ssim test).
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("volteer")),
			ExtraData:         appendJSONFiles(av110BitFilmGrainFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    av110BitFilmGrainFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "av1_10bit_ssim",
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_10BPP},
			ExtraData:         appendJSONFiles(av110BitFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    av110BitFiles,
				validatorType: decoding.SSIM,
			},
		}, {
			Name:              "h264",
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         appendJSONFiles(h264FilesFromBugs),
			Val: chromeStackDecodingTestParam{
				videoFiles:    h264FilesFromBugs,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "vp8_comprehensive",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         appendJSONFiles(vp8ComprehensiveFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    vp8ComprehensiveFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "vp8_inter",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         appendJSONFiles(vp8InterFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    vp8InterFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "vp8_inter_multi_coeff",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         appendJSONFiles(vp8InterMultiCoeffFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    vp8InterMultiCoeffFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "vp8_inter_segment",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         appendJSONFiles(vp8InterSegmentFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    vp8InterSegmentFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "vp8_intra",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         appendJSONFiles(vp8IntraFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    vp8IntraFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "vp8_intra_multi_coeff",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         appendJSONFiles(vp8IntraMultiCoeffSegmentFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    vp8IntraMultiCoeffSegmentFiles,
				validatorType: decoding.MD5,
			},
		}, {
			Name:              "vp8_intra_segment",
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			ExtraData:         appendJSONFiles(vp8IntraSegmentFiles),
			Val: chromeStackDecodingTestParam{
				videoFiles:    vp8IntraSegmentFiles,
				validatorType: decoding.MD5,
			},
		}},
	})
}

func ChromeStackDecoding(ctx context.Context, s *testing.State) {
	var tv []string
	param := s.Param().(chromeStackDecodingTestParam)
	for _, file := range param.videoFiles {
		tv = append(tv, s.DataPath(file))
	}

	if err := decoding.RunAccelVideoTestWithTestVectors(ctx, s.OutDir(), tv, param.validatorType); err != nil {
		s.Fatal("test failed: ", err)
	}
}
