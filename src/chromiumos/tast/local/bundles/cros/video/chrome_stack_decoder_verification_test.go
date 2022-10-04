// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"sort"
	"testing"

	"chromiumos/tast/common/genparams"
)

// To regenerate the test parameters by running the following in a chroot:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/video
var h264FilesFromBugs = map[string]string{
	"149068426": "test_vectors/h264/files_from_bugs/b_149068426_invalid_video_layout_mtk_8183_with_direct_videodecoder.h264",
	"172838252": "test_vectors/h264/files_from_bugs/b_172838252_pixelated_video_on_rk3399.h264",
	"174733646": "test_vectors/h264/files_from_bugs/b_174733646_video_with_out_of_order_frames_mtk_8173.h264",
	"210895987": "test_vectors/h264/files_from_bugs/b_210895987_still-colors-360p.h264",
}

var h2644kFilesFromBugs = map[string]string{
	"22704778": "test_vectors/h264/files_from_bugs/b_227047778_mtk_8195_artifacts.h264",
}

type paramData struct {
	Name         string
	SoftwareDeps string
	HardwareDeps string
	Data         []string
	Attr         []string
	Comment      string

	VideoFiles    string
	ValidatorType string
	MustFail      bool
}

// genFilesFromBugs generates multiple test cases for each files in the filesFromBugs map. The key of filesFromBugs would be appended in the test name and value will be assigned to VideoFiles.
func genFilesFromBugs(defaultParam paramData, filesFromBugs map[string]string) []paramData {
	var result []paramData
	// Iterate the map in order
	keys := make([]string, 0)
	for k := range filesFromBugs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		data := defaultParam
		data.Name = data.Name + "_" + key
		data.VideoFiles = "[]string{\"" + filesFromBugs[key] + "\"}"
		result = append(result, data)
	}
	return result
}

func TestChromeStackDecoderVerificationParams(t *testing.T) {
	perBuildAttrs := []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_chromestackdecoding"}
	params := []paramData{
		{
			Name:          "av1_files_from_bugs",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeAV1}`,
			VideoFiles:    "av1FilesFromBugs",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "av1_common",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeAV1}`,
			VideoFiles:    "av1CommonFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "av1_film_grain",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeAV1}`,
			Comment:       "Different decoders may use different film grain synthesis methods while producing a visually correct output (AV1 spec 7.2). Thus we validate the decoding of film-grain streams using SSIM.",
			VideoFiles:    "av1FilmGrainFiles",
			ValidatorType: "decoding.SSIM",
		}, {
			Name:          "av1_10bit_common",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeAV1_10BPP}`,
			VideoFiles:    "av110BitCommonFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "av1_10bit_film_grain",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeAV1_10BPP}`,
			Comment:       "Different decoders may use different film grain synthesis methods while producing a visually correct output (AV1 spec 7.2). Thus, for volteer, don't validate the decoding of film-grain streams using MD5. Instead, validate them using SSIM (see the av1_10bit_ssim test).",
			VideoFiles:    "av110BitFilmGrainFiles",
			ValidatorType: "decoding.SSIM",
		}, {
			Name:          "h264_invalid_bitstreams",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeH264, "proprietary_codecs"}`,
			VideoFiles:    "h264InvalidBitstreams",
			ValidatorType: "decoding.MD5",
			MustFail:      true,
		}, {
			Name:          "h264_baseline",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeH264, "proprietary_codecs"}`,
			VideoFiles:    "h264Files[\"baseline\"]",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "h264_main",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeH264, "proprietary_codecs"}`,
			VideoFiles:    "h264Files[\"main\"]",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "h264_first_mb_in_slice",
			Attr:          perBuildAttrs,
			HardwareDeps:  "hwdep.D(hwdep.SupportsV4L2StatefulVideoDecoding())",
			SoftwareDeps:  `[]string{caps.HWDecodeH264, "proprietary_codecs"}`,
			VideoFiles:    "h264Files[\"first_mb_in_slice\"]",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp8_comprehensive",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP8}`,
			VideoFiles:    "vp8ComprehensiveFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp8_inter",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP8}`,
			VideoFiles:    "vp8InterFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp8_inter_multi_coeff",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP8}`,
			VideoFiles:    "vp8InterMultiCoeffFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp8_inter_segment",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP8}`,
			VideoFiles:    "vp8InterSegmentFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp8_intra",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP8}`,
			VideoFiles:    "vp8IntraFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp8_intra_multi_coeff",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP8}`,
			VideoFiles:    "vp8IntraMultiCoeffSegmentFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp8_intra_segment",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP8}`,
			VideoFiles:    "vp8IntraSegmentFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp9_files_from_bugs",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp9FilesFromBugs",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp9_0_group1_buf",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp90Group1Buf",
			ValidatorType: "decoding.MD5",
		}, {
			Name: "vp9_0_group1_frm_resize",
			// TODO(b/207057398): Reenable when VideoDecoder supports resolution changes in non keyframes.
			//Attr:         perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp90Group1FrmResize",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp9_0_group1_gf_dist",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp90Group1GfDist",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp9_0_group1_odd_size",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp90Group1OddSize",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "vp9_0_group1_sub8x8",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp90Group1Sub8x8",
			ValidatorType: "decoding.MD5",
		}, {
			Name: "vp9_0_group1_sub8x8_sf",
			// TODO(b/207057398): Reenable when VideoDecoder supports resolution changes in non keyframes."
			//Attr:         []string{"group:mainline", "informational"},
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp90Group1Sub8x8Sf",
			ValidatorType: "decoding.MD5",
		}, {
			Name: "vp9_0_svc",
			// TODO(b/210167476): Reenable when it's not failing everywhere.
			//Attr:         perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeVP9}`,
			VideoFiles:    "vp9SVCFiles",
			ValidatorType: "decoding.MD5",
		}, {
			Name:          "hevc_main",
			Attr:          perBuildAttrs,
			SoftwareDeps:  `[]string{caps.HWDecodeHEVC}`,
			VideoFiles:    "hevcCommonFiles",
			ValidatorType: "decoding.MD5",
		},
	}

	// h264 is the only codec that has a strong need for specific faulty cases so we create one test case per bug.
	params = append(params, genFilesFromBugs(paramData{
		Name:          "h264_files_from_bugs",
		Attr:          perBuildAttrs,
		SoftwareDeps:  `[]string{caps.HWDecodeH264, "proprietary_codecs"}`,
		ValidatorType: "decoding.MD5",
	}, h264FilesFromBugs)...)
	params = append(params, genFilesFromBugs(paramData{
		Name:          "h264_4k_files_from_bugs",
		Attr:          perBuildAttrs,
		SoftwareDeps:  `[]string{caps.HWDecodeH264_4K, "proprietary_codecs"}`,
		ValidatorType: "decoding.MD5",
	}, h2644kFilesFromBugs)...)

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
        {{ if .Comment }}
        // {{ .Comment }}
        {{ end }}
		{{ if .Attr }}
		ExtraAttr: {{ .Attr | fmt }},
		{{ end }}
		{{ if .HardwareDeps }}
		ExtraHardwareDeps: {{ .HardwareDeps }},
		{{ end }}
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps }},
		{{ end }}
		ExtraData: appendJSONFiles({{ .VideoFiles }}),
		Val:  chromeStackDecoderVerificationTestParam{
            videoFiles: {{ .VideoFiles  }},
            validatorType: {{ .ValidatorType }},
            mustFail: {{ .MustFail | fmt }},
		},
	},
	{{ end }}`, params)
	genparams.Ensure(t, "chrome_stack_decoder_verification.go", code)
}
