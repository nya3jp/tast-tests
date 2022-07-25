// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"sort"
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/media/decoding"
)

// To regenerate the test parameters by running the following in a chroot:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/video

var av1FilesFromBugs = []string{
	"test_vectors/av1/files_from_bugs/b_235138734_test-25fps-one-to-four-tiles.av1.ivf",
}

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

var vp9FilesFromBugs = []string{
	"test_vectors/vp9/files_from_bugs/b_177839888__rk3399_vp9_artifacts_with_video_decoder_japanews24.ivf",
}

var vp90Group1Buf = []string{
	"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_256X144_fr15_bd8_8buf_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/buf/grass_1_256X144_fr15_bd8_8buf_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/buf/street1_1_256X144_fr15_bd8_8buf_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_384X192_fr30_bd8_8buf_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/buf/grass_1_384X192_fr30_bd8_8buf_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/buf/street1_1_384X192_fr30_bd8_8buf_l11.ivf",
}

var vp90Group1FrmResize = []string{
	"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_256X144_fr15_bd8_frm_resize_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_256X144_fr15_bd8_frm_resize_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_256X144_fr15_bd8_frm_resize_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_384X192_fr30_bd8_frm_resize_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_384X192_fr30_bd8_frm_resize_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_384X192_fr30_bd8_frm_resize_l11.ivf",
}

var vp90Group1GfDist = []string{
	"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_256X144_fr15_bd8_gf_dist_4_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_256X144_fr15_bd8_gf_dist_4_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_256X144_fr15_bd8_gf_dist_4_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_384X192_fr30_bd8_gf_dist_4_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_384X192_fr30_bd8_gf_dist_4_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_384X192_fr30_bd8_gf_dist_4_l11.ivf",
}

var vp90Group1OddSize = []string{
	"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_248X144_fr15_bd8_odd_size_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_248X144_fr15_bd8_odd_size_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_248X144_fr15_bd8_odd_size_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_376X184_fr30_bd8_odd_size_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_376X184_fr30_bd8_odd_size_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_376X184_fr30_bd8_odd_size_l11.ivf",
}

var vp90Group1Sub8x8 = []string{
	"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_256X144_fr15_bd8_sub8X8_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_256X144_fr15_bd8_sub8X8_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_256X144_fr15_bd8_sub8X8_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_384X192_fr30_bd8_sub8X8_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_384X192_fr30_bd8_sub8X8_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_384X192_fr30_bd8_sub8X8_l11.ivf",
}

var vp90Group1Sub8x8Sf = []string{
	"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_256X144_fr15_bd8_sub8x8_sf_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_256X144_fr15_bd8_sub8x8_sf_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_256X144_fr15_bd8_sub8x8_sf_l1.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_384X192_fr30_bd8_sub8x8_sf_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_384X192_fr30_bd8_sub8x8_sf_l11.ivf",
	"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_384X192_fr30_bd8_sub8x8_sf_l11.ivf",
}
var vp9SVCFiles = []string{
	"test_vectors/vp9/kSVC/ksvc_3sl_3tl_key100.ivf",
}

// These are invalid bistreams one way or another that are decoded correctly
// (no artifacts) with a software decoder but not when using certain hardware
// decoder implementations. These tests are expected to fail long-term, but it's
// interesting to have them to verify that e.g. the driver doesn't crash.
var h264InvalidBitstreams = []string{
	"test_vectors/h264/files_from_bugs/b_234651916_big_buck_bunny_artifacts_rk3399.h264",
	"test_vectors/h264/files_from_bugs/b_184041918_Webex_out_of_order_h264_frames.h264",
}

var h264Files = map[string][]string{
	"baseline": {
		"test_vectors/h264/baseline/AUD_MW_E.h264",
		"test_vectors/h264/baseline/BA1_Sony_D.h264",
		"test_vectors/h264/baseline/BA2_Sony_F.h264",
		"test_vectors/h264/baseline/BAMQ1_JVC_C.h264",
		"test_vectors/h264/baseline/BAMQ2_JVC_C.h264",
		"test_vectors/h264/baseline/BANM_MW_D.h264",
		"test_vectors/h264/baseline/BA_MW_D.h264",
		"test_vectors/h264/baseline/CI_MW_D.h264",
		"test_vectors/h264/baseline/CVSE2_Sony_B.h264",
		"test_vectors/h264/baseline/HCBP1_HHI_A.h264",
		"test_vectors/h264/baseline/HCBP2_HHI_A.h264",
		"test_vectors/h264/baseline/LS_SVA_D.h264",
		"test_vectors/h264/baseline/MIDR_MW_D.h264",
		"test_vectors/h264/baseline/MPS_MW_A.h264",
		"test_vectors/h264/baseline/MR1_MW_A.h264",
		"test_vectors/h264/baseline/MR2_MW_A.h264",
		"test_vectors/h264/baseline/NL1_Sony_D.h264",
		"test_vectors/h264/baseline/NL2_Sony_H.h264",
		"test_vectors/h264/baseline/NLMQ1_JVC_C.h264",
		"test_vectors/h264/baseline/NLMQ2_JVC_C.h264",
		"test_vectors/h264/baseline/NRF_MW_E.h264",
		"test_vectors/h264/baseline/SVA_BA1_B.h264",
		"test_vectors/h264/baseline/SVA_BA2_D.h264",
		"test_vectors/h264/baseline/SVA_NL1_B.h264",
		"test_vectors/h264/baseline/SVA_NL2_E.h264",

		// The following test vectors are disabled because they don't verify that
		// |max_num_reorder_frames| is smaller or equal to the DPB size, see
		// b/216179527.
		//"test_vectors/h264/baseline/MR2_TANDBERG_E.h264",
		//"test_vectors/h264/baseline/MR3_TANDBERG_B.h264",
		//"test_vectors/h264/baseline/MR4_TANDBERG_C.h264",
		//"test_vectors/h264/baseline/MR5_TANDBERG_C.h264",
	},
	"main": {
		"test_vectors/h264/main/CABA1_SVA_B.h264",
		"test_vectors/h264/main/CABA1_Sony_D.h264",
		"test_vectors/h264/main/CABA2_SVA_B.h264",
		"test_vectors/h264/main/CABA2_Sony_E.h264",
		"test_vectors/h264/main/CABA3_SVA_B.h264",
		"test_vectors/h264/main/CABA3_Sony_C.h264",
		"test_vectors/h264/main/CABA3_TOSHIBA_E.h264",
		"test_vectors/h264/main/CACQP3_Sony_D.h264",
		"test_vectors/h264/main/CANL1_SVA_B.h264",
		"test_vectors/h264/main/CANL1_Sony_E.h264",
		"test_vectors/h264/main/CANL1_TOSHIBA_G.h264",
		"test_vectors/h264/main/CANL2_SVA_B.h264",
		"test_vectors/h264/main/CANL2_Sony_E.h264",
		"test_vectors/h264/main/CANL3_SVA_B.h264",
		"test_vectors/h264/main/CANL3_Sony_C.h264",
		"test_vectors/h264/main/CANL4_SVA_B.h264",
		"test_vectors/h264/main/CAPCM1_Sand_E.h264",
		"test_vectors/h264/main/CAPCMNL1_Sand_E.h264",
		"test_vectors/h264/main/CAPM3_Sony_D.h264",
		"test_vectors/h264/main/CAQP1_Sony_B.h264",
		"test_vectors/h264/main/CAWP1_TOSHIBA_E.h264",
		"test_vectors/h264/main/CAWP5_TOSHIBA_E.h264",
		"test_vectors/h264/main/CVBS3_Sony_C.h264",
		"test_vectors/h264/main/CVPCMNL1_SVA_C.h264",
		"test_vectors/h264/main/CVPCMNL2_SVA_C.h264",
		"test_vectors/h264/main/CVSE3_Sony_H.h264",
		"test_vectors/h264/main/CVSEFDFT3_Sony_E.h264",
		"test_vectors/h264/main/CVWP1_TOSHIBA_E.h264",
		"test_vectors/h264/main/CVWP2_TOSHIBA_E.h264",
		"test_vectors/h264/main/CVWP3_TOSHIBA_E.h264",
		"test_vectors/h264/main/CVWP5_TOSHIBA_E.h264",
		"test_vectors/h264/main/NL3_SVA_E.h264",
		"test_vectors/h264/main/camp_mot_frm0_full.h264",
		"test_vectors/h264/main/cvmp_mot_frm0_full_B.h264",
		"test_vectors/h264/main/src19td.IBP.h264",
		"test_vectors/h264/main/HCMP1_HHI_A.h264",

		// The following test vectors are disabled because they don't verify the
		// SPS's |frame_mbs_only_flag|, i.e. they contain interlaced macroblocks
		// which are not supported, see b/216319263.
		//"test_vectors/h264/main/CAMA1_Sony_C.h264",
		//"test_vectors/h264/main/CAMA1_TOSHIBA_B.h264",
		//"test_vectors/h264/main/CAMA3_Sand_E.h264",
		//"test_vectors/h264/main/CAMACI3_Sony_C.h264",
		//"test_vectors/h264/main/CAMANL1_TOSHIBA_B.h264",
		//"test_vectors/h264/main/CAMANL2_TOSHIBA_B.h264",
		//"test_vectors/h264/main/CAMANL3_Sand_E.h264",
		//"test_vectors/h264/main/CAMASL3_Sony_B.h264",
		//"test_vectors/h264/main/CAMP_MOT_MBAFF_L30.h264",
		//"test_vectors/h264/main/CAMP_MOT_MBAFF_L31.h264",
		//"test_vectors/h264/main/CANLMA2_Sony_C.h264",
		//"test_vectors/h264/main/CANLMA3_Sony_C.h264",
		//"test_vectors/h264/main/CVCANLMA2_Sony_C.h264",
		//"test_vectors/h264/main/CVMA1_Sony_D.h264",
		//"test_vectors/h264/main/CVMA1_TOSHIBA_B.h264",
		//"test_vectors/h264/main/CVMANL1_TOSHIBA_B.h264",
		//"test_vectors/h264/main/CVMANL2_TOSHIBA_B.h264",
		//"test_vectors/h264/main/CVMAQP2_Sony_G.h264",
		//"test_vectors/h264/main/CVMAQP3_Sony_D.h264",
		//"test_vectors/h264/main/camp_mot_mbaff0_full.h264",
		//"test_vectors/h264/main/cvmp_mot_mbaff0_full_B.h264",
	},
	// The following test vectors are separated because they don't verify that all
	// slice header's |first_mb_in_slice| is zero, which is not supported by
	// Chromium's parsers (see b/216179527). Stateful decoders, who have their own
	// H.264 parsers, might support them, though.
	"first_mb_in_slice": {
		"test_vectors/h264/baseline/BA1_FT_C.h264",
		"test_vectors/h264/baseline/BASQP1_Sony_C.h264",
		"test_vectors/h264/baseline/CI1_FT_B.h264",
		"test_vectors/h264/baseline/SVA_Base_B.h264",
		"test_vectors/h264/baseline/SVA_CL1_E.h264",
		"test_vectors/h264/baseline/SVA_FM1_E.h264",
		"test_vectors/h264/baseline/MR1_BT_A.h264",
		"test_vectors/h264/main/CABACI3_Sony_B.h264",
		"test_vectors/h264/main/CABAST3_Sony_E.h264",
		"test_vectors/h264/main/CABASTBR3_Sony_B.h264",
		"test_vectors/h264/main/SL1_SVA_B.h264",
	},
}

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
	SoftwareDeps []string
	HardwareDeps string
	Data         []string
	Attr         []string
	Comment      string

	VideoFiles    []string
	ValidatorType decoding.ValidatorType
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
		data.VideoFiles = []string{filesFromBugs[key]}
		result = append(result, data)
	}
	return result
}

// fillData fills the Data field of the params with their respecting json files.
func fillData(params []paramData) []paramData {
	appendJSONFiles := func(videoFiles []string) []string {
		var tf []string
		for _, file := range videoFiles {
			tf = append(tf, file)
			tf = append(tf, file+".json")
		}
		return tf
	}
	for i, param := range params {
		params[i].Data = appendJSONFiles(param.VideoFiles)
	}
	return params
}

func TestChromeStackDecoderVerificationParams(t *testing.T) {

	perBuildAttrs := []string{"group:graphics", "graphics_video", "graphics_perbuild", "graphics_video_chromestackdecoding"}
	params := []paramData{
		{
			Name:          "av1_files_from_bugs",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeAV1},
			VideoFiles:    av1FilesFromBugs,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "av1_common",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeAV1},
			VideoFiles:    av1CommonFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "av1_film_grain",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeAV1},
			Comment:       "Different decoders may use different film grain synthesis methods while producing a visually correct output (AV1 spec 7.2). Thus we validate the decoding of film-grain streams using SSIM.",
			VideoFiles:    av1FilmGrainFiles,
			ValidatorType: decoding.SSIM,
		}, {
			Name:          "av1_10bit_common",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeAV1_10BPP},
			VideoFiles:    av110BitCommonFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "av1_10bit_film_grain",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeAV1_10BPP},
			Comment:       "Different decoders may use different film grain synthesis methods while producing a visually correct output (AV1 spec 7.2). Thus, for volteer, don't validate the decoding of film-grain streams using MD5. Instead, validate them using SSIM (see the av1_10bit_ssim test).",
			VideoFiles:    av110BitFilmGrainFiles,
			ValidatorType: decoding.SSIM,
		}, {
			Name:          "h264_invalid_bitstreams",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeH264, "proprietary_codecs"},
			VideoFiles:    h264InvalidBitstreams,
			ValidatorType: decoding.MD5,
			MustFail:      true,
		}, {
			Name:          "h264_baseline",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeH264, "proprietary_codecs"},
			VideoFiles:    h264Files["baseline"],
			ValidatorType: decoding.MD5,
		}, {
			Name:          "h264_main",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeH264, "proprietary_codecs"},
			VideoFiles:    h264Files["main"],
			ValidatorType: decoding.MD5,
		}, {
			Name:          "h264_first_mb_in_slice",
			Attr:          perBuildAttrs,
			HardwareDeps:  "hwdep.D(hwdep.SupportsV4L2StatefulVideoDecoding())",
			SoftwareDeps:  []string{caps.HWDecodeH264, "proprietary_codecs"},
			VideoFiles:    h264Files["first_mb_in_slice"],
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp8_comprehensive",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP8},
			VideoFiles:    vp8ComprehensiveFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp8_inter",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP8},
			VideoFiles:    vp8InterFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp8_inter_multi_coeff",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP8},
			VideoFiles:    vp8InterMultiCoeffFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp8_inter_segment",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP8},
			VideoFiles:    vp8InterSegmentFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp8_intra",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP8},
			VideoFiles:    vp8IntraFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp8_intra_multi_coeff",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP8},
			VideoFiles:    vp8IntraMultiCoeffSegmentFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp8_intra_segment",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP8},
			VideoFiles:    vp8IntraSegmentFiles,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp9_files_from_bugs",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp9FilesFromBugs,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp9_0_group1_buf",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp90Group1Buf,
			ValidatorType: decoding.MD5,
		}, {
			Name: "vp9_0_group1_frm_resize",
			// TODO(b/207057398): Reenable when VideoDecoder supports resolution changes in non keyframes.
			//Attr:         []string{"group:mainline", "informational"},
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp90Group1FrmResize,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp9_0_group1_gf_dist",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp90Group1GfDist,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp9_0_group1_odd_size",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp90Group1OddSize,
			ValidatorType: decoding.MD5,
		}, {
			Name:          "vp9_0_group1_sub8x8",
			Attr:          perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp90Group1Sub8x8,
			ValidatorType: decoding.MD5,
		}, {
			Name: "vp9_0_group1_sub8x8_sf",
			// TODO(b/207057398): Reenable when VideoDecoder supports resolution changes in non keyframes."
			// ExtraAttr:         []string{"group:mainline", "informational"},
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp90Group1Sub8x8Sf,
			ValidatorType: decoding.MD5,
		}, {
			Name: "vp9_0_svc",
			// TODO(b/210167476): Reenable when it's not failing everywhere.
			//ExtraAttr:         perBuildAttrs,
			SoftwareDeps:  []string{caps.HWDecodeVP9},
			VideoFiles:    vp9SVCFiles,
			ValidatorType: decoding.MD5,
		},
	}

	// h264 is the only codec that has a strong need for specific faulty cases so we create one test case per bug.
	params = append(params, genFilesFromBugs(paramData{
		Name:          "h264_files_from_bugs",
		Attr:          perBuildAttrs,
		SoftwareDeps:  []string{caps.HWDecodeH264, "proprietary_codecs"},
		ValidatorType: decoding.MD5,
	}, h264FilesFromBugs)...)
	params = append(params, genFilesFromBugs(paramData{
		Name:          "h264_4k_files_from_bugs",
		Attr:          perBuildAttrs,
		SoftwareDeps:  []string{caps.HWDecodeH264_4K, "proprietary_codecs"},
		ValidatorType: decoding.MD5,
	}, h2644kFilesFromBugs)...)

	params = fillData(params)
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
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
        {{ if .Data }}
		ExtraData: {{ .Data | fmt }},
        {{ end }}
		Val:  chromeStackDecoderVerificationTestParam{
            videoFiles: {{ .VideoFiles | fmt }},
            validatorType: {{ .ValidatorType }},
            mustFail: {{ .MustFail | fmt }},
		},
	},
	{{ end }}`, params)
	genparams.Ensure(t, "chrome_stack_decoder_verification.go", code)
}
