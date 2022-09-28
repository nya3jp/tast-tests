// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/chrome"
)

const ffmpegMD5Path = "/usr/local/graphics/ffmpeg_md5sum"

// NB: If modifying any of the files or test specifications, be sure to
// regenerate the test parameters by running the following in a chroot:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/video

var av1Files = []string{
	"test_vectors/av1/8-bit/00000527.ivf",
	"test_vectors/av1/8-bit/00000535.ivf",
	"test_vectors/av1/8-bit/00000548.ivf",
	"test_vectors/av1/8-bit/48_delayed.ivf",
	"test_vectors/av1/8-bit/av1-1-b8-02-allintra.ivf",
	"test_vectors/av1/8-bit/frames_refs_short_signaling.ivf",
	"test_vectors/av1/8-bit/non_uniform_tiling.ivf",
	"test_vectors/av1/8-bit/test-25fps-192x288-only-tile-cols-is-power-of-2.ivf",
	"test_vectors/av1/8-bit/test-25fps-192x288-only-tile-rows-is-power-of-2.ivf",
	"test_vectors/av1/8-bit/test-25fps-192x288-tile-rows-3-tile-cols-3.ivf",
}

var av1AomFiles = map[string]map[string][]string{
	"8bit": {
		"quantizer": {
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-00.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-01.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-02.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-03.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-04.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-05.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-06.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-07.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-08.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-09.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-10.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-11.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-12.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-13.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-14.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-15.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-16.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-17.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-18.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-19.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-20.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-21.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-22.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-23.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-24.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-25.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-26.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-27.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-28.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-29.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-30.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-31.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-32.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-33.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-34.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-35.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-36.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-37.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-38.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-39.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-40.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-41.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-42.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-43.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-44.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-45.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-46.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-47.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-48.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-49.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-50.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-51.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-52.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-53.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-54.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-55.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-56.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-57.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-58.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-59.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-60.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-61.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-62.ivf",
			"test_vectors/av1/aom/av1-1-b8-00-quantizer-63.ivf",
		},
		"size": {
			"test_vectors/av1/aom/av1-1-b8-01-size-16x16.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-16x18.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-16x32.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-16x34.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-16x64.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-16x66.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-18x16.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-18x18.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-18x32.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-18x34.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-18x64.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-18x66.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-32x16.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-32x18.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-32x32.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-32x34.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-32x64.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-32x66.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-34x16.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-34x18.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-34x32.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-34x34.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-34x64.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-34x66.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-64x16.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-64x18.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-64x32.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-64x34.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-64x64.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-64x66.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-66x16.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-66x18.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-66x32.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-66x34.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-66x64.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-66x66.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-196x226.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-198x226.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-200x226.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-202x226.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-208x226.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-210x226.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-224x226.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x196.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x198.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x200.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x202.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x208.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x210.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x224.ivf",
			"test_vectors/av1/aom/av1-1-b8-01-size-226x226.ivf",
		},
		"allintra":  {"test_vectors/av1/aom/av1-1-b8-02-allintra.ivf"},
		"cdfupdate": {"test_vectors/av1/aom/av1-1-b8-04-cdfupdate.ivf"},
		"motionvec": {
			"test_vectors/av1/aom/av1-1-b8-05-mv.ivf",
			"test_vectors/av1/aom/av1-1-b8-06-mfmv.ivf",
		},
		"svc": {
			"test_vectors/av1/aom/av1-1-b8-22-svc-L1T2.ivf",
			"test_vectors/av1/aom/av1-1-b8-22-svc-L2T1.ivf",
			"test_vectors/av1/aom/av1-1-b8-22-svc-L2T2.ivf",
		},
		"filmgrain": {"test_vectors/av1/aom/av1-1-b8-23-film_grain-50.ivf"},
	},
}

var hevcFiles = map[string][]string{
	"main": {
		"test_vectors/hevc/main/AMP_A_Samsung_7.hevc",
		"test_vectors/hevc/main/AMP_B_Samsung_7.hevc",
		"test_vectors/hevc/main/AMP_D_Hisilicon.hevc",
		"test_vectors/hevc/main/AMP_E_Hisilicon.hevc",
		"test_vectors/hevc/main/AMP_F_Hisilicon_3.hevc",
		"test_vectors/hevc/main/AMVP_A_MTK_4.hevc",
		"test_vectors/hevc/main/AMVP_B_MTK_4.hevc",
		"test_vectors/hevc/main/AMVP_C_Samsung_7.hevc",
		"test_vectors/hevc/main/CAINIT_A_SHARP_4.hevc",
		"test_vectors/hevc/main/CAINIT_B_SHARP_4.hevc",
		"test_vectors/hevc/main/CAINIT_C_SHARP_3.hevc",
		"test_vectors/hevc/main/CAINIT_D_SHARP_3.hevc",
		"test_vectors/hevc/main/CAINIT_E_SHARP_3.hevc",
		"test_vectors/hevc/main/CAINIT_F_SHARP_3.hevc",
		"test_vectors/hevc/main/CAINIT_G_SHARP_3.hevc",
		"test_vectors/hevc/main/CAINIT_H_SHARP_3.hevc",
		"test_vectors/hevc/main/CIP_A_Panasonic_3.hevc",
		"test_vectors/hevc/main/cip_B_NEC_3.hevc",
		"test_vectors/hevc/main/CIP_C_Panasonic_2.hevc",
		"test_vectors/hevc/main/DBLK_A_SONY_3.hevc",
		"test_vectors/hevc/main/DBLK_B_SONY_3.hevc",
		"test_vectors/hevc/main/DBLK_C_SONY_3.hevc",
		"test_vectors/hevc/main/DBLK_D_VIXS_2.hevc",
		"test_vectors/hevc/main/DBLK_E_VIXS_2.hevc",
		"test_vectors/hevc/main/DBLK_F_VIXS_2.hevc",
		"test_vectors/hevc/main/DBLK_G_VIXS_2.hevc",
		"test_vectors/hevc/main/DELTAQP_A_BRCM_4.hevc",
		"test_vectors/hevc/main/DELTAQP_B_SONY_3.hevc",
		"test_vectors/hevc/main/DELTAQP_C_SONY_3.hevc",
		"test_vectors/hevc/main/DSLICE_A_HHI_5.hevc",
		"test_vectors/hevc/main/DSLICE_B_HHI_5.hevc",
		"test_vectors/hevc/main/DSLICE_C_HHI_5.hevc",
		"test_vectors/hevc/main/ENTP_A_Qualcomm_1.hevc",
		"test_vectors/hevc/main/ENTP_B_Qualcomm_1.hevc",
		"test_vectors/hevc/main/ENTP_C_Qualcomm_1.hevc",
		"test_vectors/hevc/main/EXT_A_ericsson_4.hevc",
		"test_vectors/hevc/main/FILLER_A_Sony_1.hevc",
		"test_vectors/hevc/main/HRD_A_Fujitsu_3.hevc",
		"test_vectors/hevc/main/INITQP_A_Sony_1.hevc",
		"test_vectors/hevc/main/ipcm_A_NEC_3.hevc",
		"test_vectors/hevc/main/ipcm_B_NEC_3.hevc",
		"test_vectors/hevc/main/ipcm_C_NEC_3.hevc",
		"test_vectors/hevc/main/ipcm_D_NEC_3.hevc",
		"test_vectors/hevc/main/ipcm_E_NEC_2.hevc",
		"test_vectors/hevc/main/IPRED_A_docomo_2.hevc",
		"test_vectors/hevc/main/IPRED_C_Mitsubishi_3.hevc",
		"test_vectors/hevc/main/LS_A_Orange_2.hevc",
		"test_vectors/hevc/main/LS_B_Orange_4.hevc",
		"test_vectors/hevc/main/LTRPSPS_A_Qualcomm_1.hevc",
		"test_vectors/hevc/main/MAXBINS_A_TI_5.hevc",
		"test_vectors/hevc/main/MAXBINS_B_TI_5.hevc",
		"test_vectors/hevc/main/MAXBINS_C_TI_5.hevc",
		"test_vectors/hevc/main/MERGE_A_TI_3.hevc",
		"test_vectors/hevc/main/MERGE_B_TI_3.hevc",
		"test_vectors/hevc/main/MERGE_C_TI_3.hevc",
		"test_vectors/hevc/main/MERGE_D_TI_3.hevc",
		"test_vectors/hevc/main/MERGE_E_TI_3.hevc",
		"test_vectors/hevc/main/MERGE_F_MTK_4.hevc",
		"test_vectors/hevc/main/MERGE_G_HHI_4.hevc",
		"test_vectors/hevc/main/MVCLIP_A_qualcomm_3.hevc",
		"test_vectors/hevc/main/MVDL1ZERO_A_docomo_4.hevc",
		"test_vectors/hevc/main/MVEDGE_A_qualcomm_3.hevc",
		"test_vectors/hevc/main/OPFLAG_A_Qualcomm_1.hevc",
		"test_vectors/hevc/main/OPFLAG_B_Qualcomm_1.hevc",
		"test_vectors/hevc/main/OPFLAG_C_Qualcomm_1.hevc",
		//"test_vectors/hevc/main/PICSIZE_A_Bossen_1.hevc", // Fails to decode on Trogdor - b/229784864
		//"test_vectors/hevc/main/PICSIZE_B_Bossen_1.hevc", // Fails to decode on Trogdor - b/229784864
		//"test_vectors/hevc/main/PICSIZE_C_Bossen_1.hevc", // Fails to decode on Trogdor - b/229784864
		//"test_vectors/hevc/main/PICSIZE_D_Bossen_1.hevc", // Fails to decode on Trogdor - b/229784864
		"test_vectors/hevc/main/PMERGE_A_TI_3.hevc",
		"test_vectors/hevc/main/PMERGE_B_TI_3.hevc",
		"test_vectors/hevc/main/PMERGE_C_TI_3.hevc",
		"test_vectors/hevc/main/PMERGE_D_TI_3.hevc",
		"test_vectors/hevc/main/PMERGE_E_TI_3.hevc",
		"test_vectors/hevc/main/PPS_A_qualcomm_7.hevc",
		"test_vectors/hevc/main/PS_B_VIDYO_3.hevc",
		"test_vectors/hevc/main/RPLM_A_qualcomm_4.hevc",
		"test_vectors/hevc/main/RPS_A_docomo_5.hevc",
		"test_vectors/hevc/main/RPS_B_qualcomm_5.hevc",
		"test_vectors/hevc/main/RPS_E_qualcomm_5.hevc",
		"test_vectors/hevc/main/RPS_F_docomo_2.hevc",
		"test_vectors/hevc/main/RQT_A_HHI_4.hevc",
		"test_vectors/hevc/main/RQT_B_HHI_4.hevc",
		"test_vectors/hevc/main/RQT_C_HHI_4.hevc",
		"test_vectors/hevc/main/RQT_D_HHI_4.hevc",
		"test_vectors/hevc/main/RQT_E_HHI_4.hevc",
		"test_vectors/hevc/main/RQT_F_HHI_4.hevc",
		"test_vectors/hevc/main/RQT_G_HHI_4.hevc",
		"test_vectors/hevc/main/SAO_A_MediaTek_4.hevc",
		"test_vectors/hevc/main/SAO_B_MediaTek_5.hevc",
		"test_vectors/hevc/main/SAO_C_Samsung_5.hevc",
		"test_vectors/hevc/main/SAODBLK_A_MainConcept_4.hevc",
		"test_vectors/hevc/main/SAODBLK_B_MainConcept_4.hevc",
		"test_vectors/hevc/main/SAO_D_Samsung_5.hevc",
		"test_vectors/hevc/main/SAO_E_Canon_4.hevc",
		"test_vectors/hevc/main/SAO_F_Canon_3.hevc",
		"test_vectors/hevc/main/SAO_G_Canon_3.hevc",
		"test_vectors/hevc/main/SAO_H_Parabola_1.hevc",
		"test_vectors/hevc/main/SDH_A_Orange_4.hevc",
		"test_vectors/hevc/main/SLICES_A_Rovi_3.hevc",
		"test_vectors/hevc/main/SLPPLP_A_VIDYO_2.hevc",
		"test_vectors/hevc/main/STRUCT_A_Samsung_7.hevc",
		"test_vectors/hevc/main/STRUCT_B_Samsung_7.hevc",
		"test_vectors/hevc/main/TILES_A_Cisco_2.hevc",
		"test_vectors/hevc/main/TILES_B_Cisco_1.hevc",
		"test_vectors/hevc/main/TMVP_A_MS_3.hevc",
		"test_vectors/hevc/main/TSCL_A_VIDYO_5.hevc",
		"test_vectors/hevc/main/TSCL_B_VIDYO_4.hevc",
		"test_vectors/hevc/main/TSKIP_A_MS_3.hevc",
		"test_vectors/hevc/main/TUSIZE_A_Samsung_1.hevc",
		"test_vectors/hevc/main/VPSID_A_VIDYO_2.hevc",
		"test_vectors/hevc/main/WP_A_Toshiba_3.hevc",
		"test_vectors/hevc/main/WP_B_Toshiba_3.hevc",
		"test_vectors/hevc/main/WPP_A_ericsson_MAIN_2.hevc",
		"test_vectors/hevc/main/WPP_B_ericsson_MAIN_2.hevc",
		"test_vectors/hevc/main/WPP_C_ericsson_MAIN_2.hevc",
		"test_vectors/hevc/main/WPP_D_ericsson_MAIN_2.hevc",
		"test_vectors/hevc/main/WPP_E_ericsson_MAIN_2.hevc",
		"test_vectors/hevc/main/WPP_F_ericsson_MAIN_2.hevc",
		"test_vectors/hevc/main_still_picture/IPRED_B_Nokia_3.hevc",
	},
	"main_10": {
		"test_vectors/hevc/main_10/DBLK_A_MAIN10_VIXS_4.hevc",
		"test_vectors/hevc/main_10/INITQP_B_Main10_Sony_1.hevc",
		"test_vectors/hevc/main_10/WP_A_MAIN10_Toshiba_3.hevc",
		"test_vectors/hevc/main_10/WP_MAIN10_B_Toshiba_3.hevc",
		"test_vectors/hevc/main_10/WPP_A_ericsson_MAIN10_2.hevc",
		"test_vectors/hevc/main_10/WPP_B_ericsson_MAIN10_2.hevc",
		"test_vectors/hevc/main_10/WPP_C_ericsson_MAIN10_2.hevc",
		"test_vectors/hevc/main_10/WPP_D_ericsson_MAIN10_2.hevc",
		"test_vectors/hevc/main_10/WPP_E_ericsson_MAIN10_2.hevc",
		"test_vectors/hevc/main_10/WPP_F_ericsson_MAIN10_2.hevc",
	},
	"3d_hevc": {
		"test_vectors/hevc/3d_hevc/3DHC_C_A.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_C_B.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_C_C.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_A.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_B.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_C.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_D.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_E.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_F.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_G.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D1_H.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D2_A.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_D2_B.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_DT_A.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_DT_B.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_DT_C.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_DT_D.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_T_A.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_T_B.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_T_C.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_TD_A.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_TD_B.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_TD_C.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_TD_D.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_TD_E.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_T_D.hevc",
		"test_vectors/hevc/3d_hevc/3DHC_T_E.hevc",
	},
	"mv_hevc": {
		"test_vectors/hevc/mv_hevc/MVHEVCS_A.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_B.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_C.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_D.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_E.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_F.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_G.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_H.hevc",
		"test_vectors/hevc/mv_hevc/MVHEVCS_I.hevc",
	},
	"rext": {
		"test_vectors/hevc/rext/ADJUST_IPRED_ANGLE_A_RExt_Mitsubishi_2.hevc",
		"test_vectors/hevc/rext/CCP_10bit_RExt_QCOM.hevc",
		"test_vectors/hevc/rext/CCP_12bit_RExt_QCOM.hevc",
		"test_vectors/hevc/rext/CCP_8bit_RExt_QCOM.hevc",
		"test_vectors/hevc/rext/ExplicitRdpcm_A_BBC_1.hevc",
		"test_vectors/hevc/rext/ExplicitRdpcm_B_BBC_2.hevc",
		"test_vectors/hevc/rext/EXTPREC_MAIN_444_16_INTRA_10BIT_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/EXTPREC_MAIN_444_16_INTRA_12BIT_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/EXTPREC_MAIN_444_16_INTRA_8BIT_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_10b_420_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_10b_422_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_10b_444_RExt_Sony_2.hevc",
		"test_vectors/hevc/rext/GENERAL_12b_400_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_12b_420_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_12b_422_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_12b_444_RExt_Sony_2.hevc",
		"test_vectors/hevc/rext/GENERAL_8b_400_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_8b_420_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/GENERAL_8b_444_RExt_Sony_2.hevc",
		"test_vectors/hevc/rext/IPCM_A_RExt_NEC_2.hevc",
		"test_vectors/hevc/rext/IPCM_B_RExt_NEC.hevc",
		"test_vectors/hevc/rext/Main_422_10_A_RExt_Sony_2.hevc",
		"test_vectors/hevc/rext/Main_422_10_B_RExt_Sony_2.hevc",
		"test_vectors/hevc/rext/PERSIST_RPARAM_A_RExt_Sony_3.hevc",
		"test_vectors/hevc/rext/QMATRIX_A_RExt_Sony_1.hevc",
		"test_vectors/hevc/rext/SAO_A_RExt_MediaTek_1.hevc",
		"test_vectors/hevc/rext/TSCTX_10bit_I_RExt_SHARP_1.hevc",
		"test_vectors/hevc/rext/TSCTX_10bit_RExt_SHARP_1.hevc",
		"test_vectors/hevc/rext/TSCTX_12bit_I_RExt_SHARP_1.hevc",
		"test_vectors/hevc/rext/TSCTX_12bit_RExt_SHARP_1.hevc",
		"test_vectors/hevc/rext/TSCTX_8bit_I_RExt_SHARP_1.hevc",
		"test_vectors/hevc/rext/TSCTX_8bit_RExt_SHARP_1.hevc",
	},
	"shvc": {
		"test_vectors/hevc/shvc/8LAYERS_QUALCOMM_1.hevc",
		"test_vectors/hevc/shvc/ADAPTRES_A_ERICSSON_1.hevc",
		"test_vectors/hevc/shvc/ALPHA_A_BBC_1.hevc",
		"test_vectors/hevc/shvc/CGS_A_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_B_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_C_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_D_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_E_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_F_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_G_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_H_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CGS_I_TECHNICOLOR_1.hevc",
		"test_vectors/hevc/shvc/CONFCROP_A_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/CONFCROP_B_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/CONFCROP_C_VIDYO_3.hevc",
		"test_vectors/hevc/shvc/DEPTH_A_NOKIA_1.hevc",
		"test_vectors/hevc/shvc/DISFLAG_A_QUALCOMM_1.hevc",
		"test_vectors/hevc/shvc/DPB_A_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/DPB_B_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/INACTIVE_A_QCOM_1.hevc",
		"test_vectors/hevc/shvc/LAYERID63_A_HHI_1.hevc",
		"test_vectors/hevc/shvc/LAYERID_A_NOKIA_2.hevc",
		"test_vectors/hevc/shvc/MAXTID_A_ETRI_2.hevc",
		"test_vectors/hevc/shvc/MAXTID_B_ETRI_2.hevc",
		"test_vectors/hevc/shvc/MAXTID_C_ETRI_2.hevc",
		"test_vectors/hevc/shvc/MVD_A_IDCC_1.hevc",
		"test_vectors/hevc/shvc/MVD_A_NOKIA_1.hevc",
		"test_vectors/hevc/shvc/NONVUI_A_QUALCOMM_1.hevc",
		"test_vectors/hevc/shvc/NONVUI_B_QUALCOMM_1.hevc",
		"test_vectors/hevc/shvc/NONVUI_C_QUALCOMM_1.hevc",
		"test_vectors/hevc/shvc/OLS_A_NOKIA_1.hevc",
		"test_vectors/hevc/shvc/OLS_B_NOKIA_1.hevc",
		"test_vectors/hevc/shvc/OLS_C_NOKIA_1.hevc",
		"test_vectors/hevc/shvc/POC_A_Ericsson_1.hevc",
		"test_vectors/hevc/shvc/POC_B_Ericsson_1.hevc",
		"test_vectors/hevc/shvc/PPSSLIST_A_Sony_2.hevc",
		"test_vectors/hevc/shvc/PSEXT_A_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/REFLAYER_A_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/REFLAYER_B_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/REFLAYER_C_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/REFLAYER_D_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/REFREGOFF_A_SHARP_1.hevc",
		"test_vectors/hevc/shvc/REPFMT_A_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/REPFMT_B_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/REPFMT_C_VIDYO_2.hevc",
		"test_vectors/hevc/shvc/RESCHANGE_A_VIDYO_1.hevc",
		"test_vectors/hevc/shvc/RESPHASE_A_SAMSUNG_2.hevc",
		"test_vectors/hevc/shvc/SCREFOFF_A_QCOM_1.hevc",
		"test_vectors/hevc/shvc/SIM_A_IDCC_1.hevc",
		"test_vectors/hevc/shvc/SIM_B_IDCC_1.hevc",
		"test_vectors/hevc/shvc/SLLEV_A_VIDYO_1.hevc",
		"test_vectors/hevc/shvc/SNR_A_IDCC_1.hevc",
		"test_vectors/hevc/shvc/SNR_B_IDCC_1.hevc",
		"test_vectors/hevc/shvc/SNR_C_IDCC_1.hevc",
		"test_vectors/hevc/shvc/SPLITFLAG_A_HHI_1.hevc",
		"test_vectors/hevc/shvc/SPSREPFMT_A_Sony_2.hevc",
		"test_vectors/hevc/shvc/SPSSLIST_A_Sony_2.hevc",
		"test_vectors/hevc/shvc/SRATIOS_A_SAMSUNG_3.hevc",
		"test_vectors/hevc/shvc/SRATIOS_B_SAMSUNG_2.hevc",
		"test_vectors/hevc/shvc/SREXT_A_FUJITSU_1.hevc",
		"test_vectors/hevc/shvc/SREXT_B_FUJITSU_1.hevc",
		"test_vectors/hevc/shvc/SREXT_D_FUJITSU_1.hevc",
		"test_vectors/hevc/shvc/SREXT_E_FUJITSU_1.hevc",
		"test_vectors/hevc/shvc/SREXT_F_FUJITSU_1.hevc",
		"test_vectors/hevc/shvc/VUI_A_QUALCOMM_1.hevc",
		"test_vectors/hevc/shvc/VUI_B_QUALCOMM_1.hevc",
		"test_vectors/hevc/shvc/VUI_C_QUALCOMM_1.hevc",
	},
}

// b(242711007): These test vectors are failing for VAAPI, but since we have removed
// them from the hevc Files map, they are no longer being used in V4l2 tests.
var hevcFilesFromBugs = map[string]map[string][]string{
	"main": {
		"239819547": {
			"test_vectors/hevc/main/BUMPING_A_ericsson_1.hevc",
			"test_vectors/hevc/main/NoOutPrior_B_Qualcomm_1.hevc",
		},
		"239927523": {
			"test_vectors/hevc/main/NUT_A_ericsson_5.hevc",
			"test_vectors/hevc/main/RAP_A_docomo_6.hevc",
			"test_vectors/hevc/main/RAP_B_Bossen_2.hevc",
		},
		"239936640": {
			"test_vectors/hevc/main/SLIST_A_Sony_5.hevc",
			"test_vectors/hevc/main/SLIST_B_Sony_9.hevc",
			"test_vectors/hevc/main/SLIST_C_Sony_4.hevc",
			"test_vectors/hevc/main/SLIST_D_Sony_9.hevc",
		},
		"241775056": {
			"test_vectors/hevc/main/POC_A_Bossen_3.hevc",
		},
		"241731431": {
			"test_vectors/hevc/main/RPS_D_ericsson_6.hevc",
		},
		"241733687": {
			"test_vectors/hevc/main/CONFWIN_A_Sony_1.hevc",
		},
		"241727534": {
			"test_vectors/hevc/main/RPLM_B_qualcomm_4.hevc",
		},
		"241731425": {
			"test_vectors/hevc/main/VPSSPSPPS_A_MainConcept_1.hevc",
		},
		"241772308": {
			"test_vectors/hevc/main/NoOutPrior_A_Qualcomm_1.hevc",
		},
		"242708185": {
			"test_vectors/hevc/main/RPS_C_ericsson_5.hevc",
		},
	},
}

// These files come from the WebM test streams and are grouped according to
// (rounded down) level, i.e. "group1" consists of level 1 and 1.1 streams,
// "group2" of level 2 and 2.1, etc. This helps to keep together tests with
// similar amounts of intended behavior/expected stress on devices.
var vp9WebmFiles = map[string]map[string]map[string][]string{
	"profile_0": {
		"group1": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_256X144_fr15_bd8_8buf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_256X144_fr15_bd8_8buf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_256X144_fr15_bd8_8buf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_384X192_fr30_bd8_8buf_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_384X192_fr30_bd8_8buf_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_384X192_fr30_bd8_8buf_l11.ivf",
			},
			"frm_resize": {
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_256X144_fr15_bd8_frm_resize_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_256X144_fr15_bd8_frm_resize_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_256X144_fr15_bd8_frm_resize_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_384X192_fr30_bd8_frm_resize_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_384X192_fr30_bd8_frm_resize_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_384X192_fr30_bd8_frm_resize_l11.ivf",
			},
			"gf_dist": {
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_256X144_fr15_bd8_gf_dist_4_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_256X144_fr15_bd8_gf_dist_4_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_256X144_fr15_bd8_gf_dist_4_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_384X192_fr30_bd8_gf_dist_4_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_384X192_fr30_bd8_gf_dist_4_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_384X192_fr30_bd8_gf_dist_4_l11.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_248X144_fr15_bd8_odd_size_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_248X144_fr15_bd8_odd_size_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_248X144_fr15_bd8_odd_size_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_376X184_fr30_bd8_odd_size_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_376X184_fr30_bd8_odd_size_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_376X184_fr30_bd8_odd_size_l11.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_256X144_fr15_bd8_sub8X8_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_256X144_fr15_bd8_sub8X8_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_256X144_fr15_bd8_sub8X8_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_384X192_fr30_bd8_sub8X8_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_384X192_fr30_bd8_sub8X8_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_384X192_fr30_bd8_sub8X8_l11.ivf",
			},
			"sub8x8_sf": {
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_256X144_fr15_bd8_sub8x8_sf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_256X144_fr15_bd8_sub8x8_sf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_256X144_fr15_bd8_sub8x8_sf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_384X192_fr30_bd8_sub8x8_sf_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_384X192_fr30_bd8_sub8x8_sf_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_384X192_fr30_bd8_sub8x8_sf_l11.ivf",
			},
		},
		"group2": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_480X256_fr30_bd8_8buf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_480X256_fr30_bd8_8buf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_480X256_fr30_bd8_8buf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_640X384_fr30_bd8_8buf_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_640X384_fr30_bd8_8buf_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_640X384_fr30_bd8_8buf_l21.ivf",
			},
			"frm_resize": {
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_480X256_fr30_bd8_frm_resize_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_480X256_fr30_bd8_frm_resize_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_480X256_fr30_bd8_frm_resize_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_640X384_fr30_bd8_frm_resize_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_640X384_fr30_bd8_frm_resize_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_640X384_fr30_bd8_frm_resize_l21.ivf",
			},
			"gf_dist": {
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_480X256_fr30_bd8_gf_dist_4_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_480X256_fr30_bd8_gf_dist_4_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_480X256_fr30_bd8_gf_dist_4_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_640X384_fr30_bd8_gf_dist_4_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_640X384_fr30_bd8_gf_dist_4_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_640X384_fr30_bd8_gf_dist_4_l21.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_472X248_fr30_bd8_odd_size_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_472X248_fr30_bd8_odd_size_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_472X248_fr30_bd8_odd_size_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_632X376_fr30_bd8_odd_size_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_632X376_fr30_bd8_odd_size_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_632X376_fr30_bd8_odd_size_l21.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_480X256_fr30_bd8_sub8X8_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_480X256_fr30_bd8_sub8X8_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_480X256_fr30_bd8_sub8X8_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_640X384_fr30_bd8_sub8X8_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_640X384_fr30_bd8_sub8X8_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_640X384_fr30_bd8_sub8X8_l21.ivf",
			},
			"sub8x8_sf": {
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_480X256_fr30_bd8_sub8x8_sf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_480X256_fr30_bd8_sub8x8_sf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_480X256_fr30_bd8_sub8x8_sf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_640X384_fr30_bd8_sub8x8_sf_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_640X384_fr30_bd8_sub8x8_sf_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_640X384_fr30_bd8_sub8x8_sf_l21.ivf",
			},
		},
		"group3": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_1080X512_fr30_bd8_8buf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_1080X512_fr30_bd8_8buf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_1080X512_fr30_bd8_8buf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_1280X768_fr30_bd8_8buf_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_1280X768_fr30_bd8_8buf_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_1280X768_fr30_bd8_8buf_l31.ivf",
			},
			"frm_resize": {
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_1080X512_fr30_bd8_frm_resize_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_1080X512_fr30_bd8_frm_resize_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_1080X512_fr30_bd8_frm_resize_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_1280X768_fr30_bd8_frm_resize_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_1280X768_fr30_bd8_frm_resize_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_1280X768_fr30_bd8_frm_resize_l31.ivf",
			},
			"gf_dist": {
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_1080X512_fr30_bd8_gf_dist_4_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_1080X512_fr30_bd8_gf_dist_4_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_1080X512_fr30_bd8_gf_dist_4_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_1280X768_fr30_bd8_gf_dist_4_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_1280X768_fr30_bd8_gf_dist_4_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_1280X768_fr30_bd8_gf_dist_4_l31.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_1080X504_fr30_bd8_odd_size_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_1080X504_fr30_bd8_odd_size_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_1080X504_fr30_bd8_odd_size_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_1280X768_fr30_bd8_odd_size_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_1280X768_fr30_bd8_odd_size_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_1280X768_fr30_bd8_odd_size_l31.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_1080X512_fr30_bd8_sub8X8_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_1080X512_fr30_bd8_sub8X8_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_1080X512_fr30_bd8_sub8X8_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_1280X768_fr30_bd8_sub8X8_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_1280X768_fr30_bd8_sub8X8_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_1280X768_fr30_bd8_sub8X8_l31.ivf",
			},
			"sub8x8_sf": {
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_1080X512_fr30_bd8_sub8x8_sf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_1080X512_fr30_bd8_sub8x8_sf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_1080X512_fr30_bd8_sub8x8_sf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_1280X768_fr30_bd8_sub8x8_sf_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_1280X768_fr30_bd8_sub8x8_sf_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_1280X768_fr30_bd8_sub8x8_sf_l31.ivf",
			},
		},
		"group4": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_2048X1088_fr30_bd8_8buf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_2048X1088_fr30_bd8_8buf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_2048X1088_fr30_bd8_8buf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_2048X1088_fr60_bd8_6buf_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_2048X1088_fr60_bd8_6buf_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_2048X1088_fr60_bd8_6buf_l41.ivf",
			},
			"frm_resize": {
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_2048X1088_fr30_bd8_frm_resize_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_2048X1088_fr30_bd8_frm_resize_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_2048X1088_fr30_bd8_frm_resize_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_2048X1088_fr60_bd8_frm_resize_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_2048X1088_fr60_bd8_frm_resize_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_2048X1088_fr60_bd8_frm_resize_l41.ivf",
			},
			"gf_dist": {
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_2048X1088_fr30_bd8_gf_dist_4_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_2048X1088_fr30_bd8_gf_dist_4_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_2048X1088_fr30_bd8_gf_dist_4_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_2048X1088_fr60_bd8_gf_dist_5_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_2048X1088_fr60_bd8_gf_dist_5_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_2048X1088_fr60_bd8_gf_dist_5_l41.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_2040X1080_fr30_bd8_odd_size_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_2040X1080_fr30_bd8_odd_size_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_2040X1080_fr30_bd8_odd_size_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_2040X1080_fr60_bd8_odd_size_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_2040X1080_fr60_bd8_odd_size_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_2040X1080_fr60_bd8_odd_size_l41.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_2048X1088_fr30_bd8_sub8X8_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_2048X1088_fr30_bd8_sub8X8_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_2048X1088_fr30_bd8_sub8X8_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_2048X1088_fr60_bd8_sub8X8_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_2048X1088_fr60_bd8_sub8X8_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_2048X1088_fr60_bd8_sub8X8_l41.ivf",
			},
			"sub8x8_sf": {
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_2048X1088_fr30_bd8_sub8x8_sf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_2048X1088_fr30_bd8_sub8x8_sf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_2048X1088_fr30_bd8_sub8x8_sf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_2048X1088_fr60_bd8_sub8x8_sf_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_2048X1088_fr60_bd8_sub8x8_sf_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_2048X1088_fr60_bd8_sub8x8_sf_l41.ivf",
			},
		},
		// Name this level "5.0" instead of 5 to ensure it runs before 5.1.
		"level5_0": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_4096X2176_fr30_bd8_4buf_l5.ivf",
			},
			"frm_resize": {
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_4096X2176_fr30_bd8_frm_resize_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_4096X2176_fr30_bd8_frm_resize_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_4096X2176_fr30_bd8_frm_resize_l5.ivf",
			},
			"gf_dist": {
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_4096X2176_fr30_bd8_gf_dist_6_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_4096X2176_fr30_bd8_gf_dist_6_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_4096X2176_fr30_bd8_gf_dist_6_l5.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_4088X2168_fr30_bd8_odd_size_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_4088X2168_fr30_bd8_odd_size_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_4088X2168_fr30_bd8_odd_size_l5.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_4096X2176_fr30_bd8_sub8X8_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_4096X2176_fr30_bd8_sub8X8_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_4096X2176_fr30_bd8_sub8X8_l5.ivf",
			},
			"sub8x8_sf": {
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_4096X2176_fr30_bd8_sub8x8_sf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_4096X2176_fr30_bd8_sub8x8_sf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_4096X2176_fr30_bd8_sub8x8_sf_l5.ivf",
			},
		},
		"level5_1": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_4096X2176_fr60_bd8_4buf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_4096X2176_fr60_bd8_4buf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_4096X2176_fr60_bd8_4buf_l51.ivf",
			},
			"frm_resize": {
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_4096X2176_fr60_bd8_frm_resize_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_4096X2176_fr60_bd8_frm_resize_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_4096X2176_fr60_bd8_frm_resize_l51.ivf",
			},
			"gf_dist": {
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_4096X2176_fr60_bd8_gf_dist_10_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_4096X2176_fr60_bd8_gf_dist_10_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_4096X2176_fr60_bd8_gf_dist_10_l51.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_4088X2168_fr60_bd8_odd_size_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_4088X2168_fr60_bd8_odd_size_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_4088X2168_fr60_bd8_odd_size_l51.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_4096X2176_fr60_bd8_sub8X8_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_4096X2176_fr60_bd8_sub8X8_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_4096X2176_fr60_bd8_sub8X8_l51.ivf",
			},
			"sub8x8_sf": {
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_4096X2176_fr60_bd8_sub8X8_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_4096X2176_fr60_bd8_sub8x8_sf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_4096X2176_fr60_bd8_sub8x8_sf_l51.ivf",
			},
		},
	},
}

var vp9SVCFile = "test_vectors/vp9/kSVC/ksvc_3sl_3tl_key100.ivf"

var vp8Files = map[string][]string{
	"inter_multi_coeff": {
		"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1408.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1409.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1410.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1413.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1404.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1405.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1406.ivf",
	},
	"inter_segment": {
		"test_vectors/vp8/inter_segment/vp80-03-segmentation-1407.ivf",
	},
	"inter": {
		"test_vectors/vp8/inter/vp80-02-inter-1402.ivf",
		"test_vectors/vp8/inter/vp80-02-inter-1412.ivf",
		"test_vectors/vp8/inter/vp80-02-inter-1418.ivf",
		"test_vectors/vp8/inter/vp80-02-inter-1424.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1403.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1425.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1426.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1427.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1432.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1435.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1436.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1437.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1441.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1442.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1428.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1429.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1430.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1431.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1433.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1434.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1438.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1439.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1440.ivf",
		"test_vectors/vp8/inter/vp80-05-sharpness-1443.ivf",
	},
	"intra_multi_coeff": {
		"test_vectors/vp8/intra_multi_coeff/vp80-03-segmentation-1414.ivf",
	},
	"intra_segment": {
		"test_vectors/vp8/intra_segment/vp80-03-segmentation-1415.ivf",
	},
	"intra": {
		"test_vectors/vp8/intra/vp80-01-intra-1400.ivf",
		"test_vectors/vp8/intra/vp80-01-intra-1411.ivf",
		"test_vectors/vp8/intra/vp80-01-intra-1416.ivf",
		"test_vectors/vp8/intra/vp80-01-intra-1417.ivf",
		"test_vectors/vp8/intra/vp80-03-segmentation-1401.ivf",
	},
	"comprehensive": {
		"test_vectors/vp8/vp80-00-comprehensive-001.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-002.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-003.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-004.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-005.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-006.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-007.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-008.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-009.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-010.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-011.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-012.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-013.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-014.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-015.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-016.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-017.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-018.ivf",
	},
}

func genExtraData(videoFiles []string) []string {
	tf := make([]string, 0, 2*len(videoFiles))
	for _, file := range videoFiles {
		tf = append(tf, file)
		tf = append(tf, file+".json")
	}
	return tf
}

func TestPlatformDecodingParams(t *testing.T) {
	type paramData struct {
		Name         string
		Decoder      string
		CmdBuilder   string
		Files        []string
		Timeout      time.Duration
		HardwareDeps string
		SoftwareDeps []string
		Metadata     []string
		Attr         []string
	}

	var params []paramData

	// Define timeouts, with extensions for specific groups.
	const defaultTimeout = 10 * time.Minute
	vp9GroupExtensions := map[string]time.Duration{
		"group4":   24 * time.Hour,
		"level5_0": 24 * time.Hour,
		"level5_1": 24 * time.Hour,
	}

	// Generate VAAPI VP9 tests.
	for i, profile := range []string{"profile_0"} {
		for _, levelGroup := range []string{"group1", "group2", "group3", "group4", "level5_0", "level5_1"} {
			for _, cat := range []string{
				"buf", "frm_resize", "gf_dist", "odd_size", "sub8x8", "sub8x8_sf",
			} {
				files := vp9WebmFiles[profile][levelGroup][cat]
				param := paramData{
					Name:         fmt.Sprintf("vaapi_vp9_%d_%s_%s", i, levelGroup, cat),
					Decoder:      filepath.Join(chrome.BinTestDir, "decode_test"),
					CmdBuilder:   "vp9decodeVAAPIargs",
					Files:        files,
					Timeout:      defaultTimeout,
					SoftwareDeps: []string{"vaapi"},
					Metadata:     genExtraData(files),
					Attr:         []string{"graphics_video_vp9"},
				}
				if extension, ok := vp9GroupExtensions[levelGroup]; ok {
					param.Timeout = extension
				}

				var hardwareDeps []string

				// TODO(b/184683272): Reenable everywhere.
				if cat == "frm_resize" || cat == "sub8x8_sf" {
					hardwareDeps = append(hardwareDeps, "hwdep.SkipOnPlatform(\"zork\")")
				}

				switch levelGroup {
				case "level5_0":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K)
				case "level5_1":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K60)
					hardwareDeps = append(hardwareDeps, "hwdep.MinMemory(7169)")
				default:
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9)
				}

				param.HardwareDeps = strings.Join(hardwareDeps, ", ")
				params = append(params, param)
			}
		}
	}

	params = append(params, paramData{
		Name:         fmt.Sprintf("vaapi_vp9_0_svc"),
		Decoder:      filepath.Join(chrome.BinTestDir, "decode_test"),
		CmdBuilder:   "vp9decodeVAAPIargs",
		Files:        []string{vp9SVCFile},
		Timeout:      defaultTimeout,
		SoftwareDeps: []string{"vaapi", caps.HWDecodeVP9},
		Metadata:     genExtraData([]string{vp9SVCFile}),
		Attr:         []string{"graphics_video_vp9"},
	})

	// Generate VAAPI AV1 tests.
	params = append(params, paramData{
		Name:       "vaapi_av1",
		Decoder:    filepath.Join(chrome.BinTestDir, "decode_test"),
		CmdBuilder: "av1decodeVAAPIargs",
		Files:      av1Files,
		Timeout:    defaultTimeout,
		// These SoftwareDeps do not include the 10 bit version of AV1.
		SoftwareDeps: []string{"vaapi", caps.HWDecodeAV1},
		Metadata:     genExtraData(av1Files),
		Attr:         []string{"graphics_video_av1"},
	})

	for _, bit := range []string{"8bit"} {
		for _, cat := range []string{"quantizer", "size", "allintra", "cdfupdate", "motionvec"} {
			files := av1AomFiles[bit][cat]
			param := paramData{
				Name:       fmt.Sprintf("vaapi_av1_%s_%s", bit, cat),
				Decoder:    filepath.Join(chrome.BinTestDir, "decode_test"),
				CmdBuilder: "av1decodeVAAPIargs",
				Files:      files,
				Timeout:    defaultTimeout,
				// These SoftwareDeps do not include the 10 bit version of AV1.
				SoftwareDeps: []string{"vaapi", caps.HWDecodeAV1},
				Metadata:     genExtraData(files),
				Attr:         []string{"graphics_video_av1"},
			}

			params = append(params, param)
		}
	}

	// Generate VAAPI HEVC tests.
	for _, testGroup := range []string{"main"} {
		files := hevcFiles[testGroup]

		params = append(params, paramData{
			Name:         fmt.Sprintf("vaapi_hevc_%s", testGroup),
			Decoder:      filepath.Join(chrome.BinTestDir, "decode_test"),
			CmdBuilder:   "hevcdecodeVAAPIargs",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"vaapi", caps.HWDecodeHEVC},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_hevc"},
		})
	}

	// Generate VAAPI HEVC tests from bugs.
	for _, testGroup := range []string{"main"} {
		bugIDs := make([]string, 0, len(hevcFilesFromBugs[testGroup]))
		// Sort the keys so the order of output of the tests is deterministic.
		for k := range hevcFilesFromBugs[testGroup] {
			bugIDs = append(bugIDs, k)
		}
		sort.Strings(bugIDs)
		for _, bugID := range bugIDs {
			files := hevcFilesFromBugs[testGroup][bugID]

			params = append(params, paramData{
				Name:         fmt.Sprintf("vaapi_hevc_%s_bug_%s", testGroup, bugID),
				Decoder:      filepath.Join(chrome.BinTestDir, "decode_test"),
				CmdBuilder:   "hevcdecodeVAAPIargs",
				Files:        files,
				Timeout:      time.Minute,
				SoftwareDeps: []string{"vaapi", caps.HWDecodeHEVC},
				Metadata:     genExtraData(files),
				Attr:         []string{"graphics_video_hevc"},
			})
		}
	}

	// Generate VAAPI VP8 tests.
	for _, testGroup := range []string{"inter", "inter_multi_coeff", "inter_segment", "intra", "intra_multi_coeff", "intra_segment", "comprehensive"} {
		files := vp8Files[testGroup]

		params = append(params, paramData{
			Name:         fmt.Sprintf("vaapi_vp8_%s", testGroup),
			Decoder:      filepath.Join(chrome.BinTestDir, "decode_test"),
			CmdBuilder:   "vp8decodeVAAPIargs",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"vaapi", caps.HWDecodeVP8},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_vp8"},
		})
	}

	// Generates V4L2 H264 tests.
	for _, group := range []string{"baseline", "main", "first_mb_in_slice"} {
		files := h264Files[group]

		param := paramData{
			Name:         fmt.Sprintf("v4l2_h264_%s", group),
			Decoder:      "v4l2_stateful_decoder",
			CmdBuilder:   "v4l2StatefulDecodeArgs",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"v4l2_codec", caps.HWDecodeH264},
			HardwareDeps: "hwdep.SupportsV4L2StatefulVideoDecoding(), ",
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_h264"},
		}
		params = append(params, param)
	}

	// Generates VAAPI H264 tests.
	for _, group := range []string{"baseline", "main", "first_mb_in_slice"} {
		files := h264Files[group]

		param := paramData{
			Name:         fmt.Sprintf("vaapi_h264_%s", group),
			Decoder:      filepath.Join(chrome.BinTestDir, "decode_test"),
			CmdBuilder:   "h264decodeVAAPIargs",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"vaapi", caps.HWDecodeH264},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_h264"},
		}
		params = append(params, param)
	}

	// Generate V4L2 HEVC tests.
	for _, testGroup := range []string{"main"} {
		files := hevcFiles[testGroup]

		// TODO(b/232255167): Remove hwdep.Model in favor of SoftwareDeps: caps.HWDecodeHEVC
		hardwareDeps := []string{"hwdep.SupportsV4L2StatefulVideoDecoding()",
			"hwdep.Model(\"coachz\", \"homestar\", \"quackingstick\", \"wormdingler\", \"kingoftown\", \"lazor\", \"limozeen\", \"pazquel\", \"pompom\")"}
		params = append(params, paramData{
			Name:         fmt.Sprintf("v4l2_hevc_%s", testGroup),
			Decoder:      "v4l2_stateful_decoder",
			CmdBuilder:   "v4l2StatefulDecodeArgs",
			Files:        files,
			Timeout:      defaultTimeout,
			HardwareDeps: strings.Join(hardwareDeps, ", "),
			SoftwareDeps: []string{"v4l2_codec"},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_hevc"},
		})
	}

	// Generates V4L2 VP9 tests.
	for i, profile := range []string{"profile_0"} {
		for _, levelGroup := range []string{"group1", "group2", "group3", "group4", "level5_0", "level5_1"} {
			for _, cat := range []string{
				"buf", "frm_resize", "gf_dist", "odd_size", "sub8x8", "sub8x8_sf",
			} {
				files := vp9WebmFiles[profile][levelGroup][cat]
				param := paramData{
					Name:         fmt.Sprintf("v4l2_vp9_%d_%s_%s", i, levelGroup, cat),
					Decoder:      "v4l2_stateful_decoder",
					CmdBuilder:   "v4l2StatefulDecodeArgs",
					Files:        files,
					Timeout:      defaultTimeout,
					SoftwareDeps: []string{"v4l2_codec"},
					Metadata:     genExtraData(files),
					Attr:         []string{"graphics_video_vp9"},
				}
				if extension, ok := vp9GroupExtensions[levelGroup]; ok {
					param.Timeout = extension
				}

				hardwareDeps := []string{"hwdep.SupportsV4L2StatefulVideoDecoding()"}

				switch levelGroup {
				case "level5_0":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K)
				case "level5_1":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K60)
					hardwareDeps = append(hardwareDeps, "hwdep.MinMemory(7169)")
				default:
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9)
				}

				param.HardwareDeps = strings.Join(hardwareDeps, ", ")
				params = append(params, param)
			}
		}
	}

	// Generates V4L2 Stateless VP9 tests.
	for i, profile := range []string{"profile_0"} {
		for _, levelGroup := range []string{"group1", "group2", "group3", "group4", "level5_0", "level5_1"} {
			for _, cat := range []string{
				"buf", "frm_resize", "gf_dist", "odd_size", "sub8x8", "sub8x8_sf",
			} {
				files := vp9WebmFiles[profile][levelGroup][cat]
				param := paramData{
					Name:         fmt.Sprintf("v4l2_stateless_vp9_%d_%s_%s", i, levelGroup, cat),
					Decoder:      filepath.Join(chrome.BinTestDir, "v4l2_stateless_decoder"),
					CmdBuilder:   "v4l2StatelessDecodeArgs",
					Files:        files,
					Timeout:      defaultTimeout,
					SoftwareDeps: []string{"v4l2_codec"},
					Metadata:     genExtraData(files),
					Attr:         []string{"graphics_video_vp9"},
				}
				if extension, ok := vp9GroupExtensions[levelGroup]; ok {
					param.Timeout = extension
				}

				hardwareDeps := []string{"hwdep.SupportsV4L2StatelessVideoDecoding()"}

				switch levelGroup {
				case "level5_0":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K)
				case "level5_1":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K60)
					hardwareDeps = append(hardwareDeps, "hwdep.MinMemory(7169)")
				default:
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9)
				}

				// TODO(b/227480076): re-enable on RockChip devices (bob, gru, kevin) if needed in the future.
				hardwareDeps = append(hardwareDeps, "hwdep.SkipOnPlatform(\"bob\", \"gru\", \"kevin\")")

				param.HardwareDeps = strings.Join(hardwareDeps, ", ")
				params = append(params, param)
			}
		}
	}

	params = append(params, paramData{
		Name:         fmt.Sprintf("v4l2_vp9_0_svc"),
		Decoder:      "v4l2_stateful_decoder",
		CmdBuilder:   "v4l2StatefulDecodeArgs",
		Files:        []string{vp9SVCFile},
		Timeout:      defaultTimeout,
		SoftwareDeps: []string{"v4l2_codec", caps.HWDecodeVP9},
		HardwareDeps: "hwdep.SupportsV4L2StatefulVideoDecoding()",
		Metadata:     genExtraData([]string{vp9SVCFile}),
		Attr:         []string{"graphics_video_vp9"},
	})

	// Generate V4L2 VP8 tests.
	for _, testGroup := range []string{"inter", "inter_multi_coeff", "inter_segment", "intra", "intra_multi_coeff", "intra_segment", "comprehensive"} {
		files := vp8Files[testGroup]

		// TODO(nhebert) Use a to-be-created hardware dependency for V4L2 stateful decode
		hardwareDeps := []string{"hwdep.SupportsV4L2StatefulVideoDecoding()"}
		params = append(params, paramData{
			Name:         fmt.Sprintf("v4l2_vp8_%s", testGroup),
			Decoder:      "v4l2_stateful_decoder",
			CmdBuilder:   "v4l2StatefulDecodeArgs",
			Files:        files,
			Timeout:      defaultTimeout,
			HardwareDeps: strings.Join(hardwareDeps, ", "),
			SoftwareDeps: []string{"v4l2_codec", caps.HWDecodeVP8},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_vp8"},
		})
	}

	// Generate ffmpeg VAAPI VP9 tests.
	for i, profile := range []string{"profile_0"} {
		for _, levelGroup := range []string{"group1", "group2", "group3", "group4", "level5_0", "level5_1"} {
			for _, cat := range []string{
				"buf", "frm_resize", "gf_dist", "odd_size", "sub8x8", "sub8x8_sf",
			} {
				// Disabled due to <1% pass rate over 30 days. See b/246820265
				if fmt.Sprintf("ffmpeg_vaapi_vp9_%d_%s_%s", i, levelGroup, cat) == "ffmpeg_vaapi_vp9_0_group1_frm_resize" {
					continue
				}
				// Disabled due to <1% pass rate over 30 days. See b/246820265
				if fmt.Sprintf("ffmpeg_vaapi_vp9_%d_%s_%s", i, levelGroup, cat) == "ffmpeg_vaapi_vp9_0_group1_sub8x8_sf" {
					continue
				}
				files := vp9WebmFiles[profile][levelGroup][cat]
				param := paramData{
					Name:         fmt.Sprintf("ffmpeg_vaapi_vp9_%d_%s_%s", i, levelGroup, cat),
					Decoder:      ffmpegMD5Path,
					CmdBuilder:   "ffmpegMD5VAAPIargs",
					Files:        files,
					Timeout:      defaultTimeout,
					SoftwareDeps: []string{"vaapi"},
					Metadata:     genExtraData(files),
					Attr:         []string{"graphics_video_vp9"},
				}
				if extension, ok := vp9GroupExtensions[levelGroup]; ok {
					param.Timeout = extension
				}

				var hardwareDeps []string

				switch levelGroup {
				case "level5_0":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K)
				case "level5_1":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K60)
					hardwareDeps = append(hardwareDeps, "hwdep.MinMemory(7169)")
				default:
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9)
				}

				param.HardwareDeps = strings.Join(hardwareDeps, ", ")
				params = append(params, param)
			}
		}
	}

	// Generate ffmpeg VAAPI AV1 tests.
	params = append(params, paramData{
		Name:       "ffmpeg_vaapi_av1",
		Decoder:    filepath.Join(chrome.BinTestDir, "decode_test"),
		CmdBuilder: "av1decodeVAAPIargs",
		Files:      av1Files,
		Timeout:    defaultTimeout,
		// These SoftwareDeps do not include the 10 bit version of AV1.
		SoftwareDeps: []string{"vaapi", caps.HWDecodeAV1},
		Metadata:     genExtraData(av1Files),
		Attr:         []string{"graphics_video_av1"},
	})

	for _, bit := range []string{"8bit"} {
		for _, cat := range []string{"quantizer", "size", "allintra", "cdfupdate", "motionvec"} {
			files := av1AomFiles[bit][cat]
			param := paramData{
				Name:       fmt.Sprintf("ffmpeg_vaapi_av1_%s_%s", bit, cat),
				Decoder:    filepath.Join(chrome.BinTestDir, "decode_test"),
				CmdBuilder: "av1decodeVAAPIargs",
				Files:      files,
				Timeout:    defaultTimeout,
				// These SoftwareDeps do not include the 10 bit version of AV1.
				SoftwareDeps: []string{"vaapi", caps.HWDecodeAV1},
				Metadata:     genExtraData(files),
				Attr:         []string{"graphics_video_av1"},
			}

			params = append(params, param)
		}
	}

	// Generate ffmpeg VP8 tests.
	for _, testGroup := range []string{"inter", "inter_multi_coeff", "inter_segment", "intra", "intra_multi_coeff", "intra_segment", "comprehensive"} {
		files := vp8Files[testGroup]

		params = append(params, paramData{
			Name:         fmt.Sprintf("ffmpeg_vaapi_vp8_%s", testGroup),
			Decoder:      ffmpegMD5Path,
			CmdBuilder:   "ffmpegMD5VAAPIargs",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"vaapi", caps.HWDecodeVP8},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_vp8"},
		})
	}

	// Generate ffmpeg H264 tests.
	for _, group := range []string{"baseline", "main", "first_mb_in_slice"} {
		files := h264Files[group]

		param := paramData{
			Name:         fmt.Sprintf("ffmpeg_vaapi_h264_%s", group),
			Decoder:      ffmpegMD5Path,
			CmdBuilder:   "ffmpegMD5VAAPIargs",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"vaapi", caps.HWDecodeVP8},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_h264"},
		}
		params = append(params, param)
	}

	// Generate ffmpeg HEVC tests.
	for _, group := range []string{"main"} {
		files := hevcFiles[group]

		param := paramData{
			Name:         fmt.Sprintf("ffmpeg_vaapi_hevc_%s", group),
			Decoder:      ffmpegMD5Path,
			CmdBuilder:   "ffmpegMD5VAAPIargs",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"vaapi", caps.HWDecodeHEVC},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_hevc"},
		}
		params = append(params, param)
	}

	// Generate ffmpeg HEVC tests from bugs.
	for _, testGroup := range []string{"main"} {
		bugIDs := make([]string, 0, len(hevcFilesFromBugs[testGroup]))
		// Sort the keys so the order of output of the tests is deterministic.
		for k := range hevcFilesFromBugs[testGroup] {
			bugIDs = append(bugIDs, k)
		}
		sort.Strings(bugIDs)
		for _, bugID := range bugIDs {
			files := hevcFilesFromBugs[testGroup][bugID]

			params = append(params, paramData{
				Name:         fmt.Sprintf("ffmpeg_vaapi_hevc_%s_bug_%s", testGroup, bugID),
				Decoder:      ffmpegMD5Path,
				CmdBuilder:   "ffmpegMD5VAAPIargs",
				Files:        files,
				Timeout:      time.Minute,
				SoftwareDeps: []string{"vaapi", caps.HWDecodeHEVC},
				Metadata:     genExtraData(files),
				Attr:         []string{"graphics_video_hevc"},
			})
		}
	}

	// Generate V4L2 AV1 tests.
	params = append(params, paramData{
		Name:         "v4l2_stateless_av1",
		Decoder:      filepath.Join(chrome.BinTestDir, "v4l2_stateless_decoder"),
		CmdBuilder:   "v4l2StatelessDecodeArgs",
		Files:        av1Files,
		Timeout:      defaultTimeout,
		SoftwareDeps: []string{"v4l2_codec"},
		// TODO(b/242075797): use HW capabilities
		HardwareDeps: "hwdep.SupportsV4L2StatelessVideoDecoding(), hwdep.Model(\"tomato\", \"dojo\")",
		Metadata:     genExtraData(av1Files),
		Attr:         []string{"graphics_video_av1"},
	})

	for _, bit := range []string{"8bit"} {
		for _, cat := range []string{"quantizer", "size", "allintra", "cdfupdate", "motionvec", "svc"} {
			files := av1AomFiles[bit][cat]
			param := paramData{
				Name:         fmt.Sprintf("v4l2_stateless_av1_%s_%s", bit, cat),
				Decoder:      filepath.Join(chrome.BinTestDir, "v4l2_stateless_decoder"),
				CmdBuilder:   "v4l2StatelessDecodeArgs",
				Files:        files,
				Timeout:      defaultTimeout,
				SoftwareDeps: []string{"v4l2_codec"},
				// TODO(b/242075797): use HW capabilities
				HardwareDeps: "hwdep.SupportsV4L2StatelessVideoDecoding(), hwdep.Model(\"tomato\", \"dojo\")",
				Metadata:     genExtraData(files),
				Attr:         []string{"graphics_video_av1"},
			}

			params = append(params, param)
		}
	}

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val:  platformDecodingParams{
			filenames: {{ .Files | fmt }},
			decoder: {{ .Decoder | fmt }},
			commandBuilder: {{ .CmdBuilder }},
		},
		Timeout: {{ .Timeout | fmt }},
		{{ if .HardwareDeps }}
		ExtraHardwareDeps: hwdep.D({{ .HardwareDeps }}),
		{{ end }}
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
		ExtraData: {{ .Metadata | fmt }},
		{{ if .Attr }}
		ExtraAttr: {{ .Attr | fmt }},
		{{ end }}
	},
	{{ end }}`, params)
	genparams.Ensure(t, "platform_decoding.go", code)
}
