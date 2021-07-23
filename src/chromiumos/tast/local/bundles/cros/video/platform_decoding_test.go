// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/chrome"
)

// NB: If modifying any of the files or test specifications, be sure to
// regenerate the test parameters by running the following in a chroot:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/video

var vaapiAv1Files = []string{
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

// These files come from the WebM test streams and are grouped according to
// (rounded down) level, i.e. "group1" consists of level 1 and 1.1 streams,
// "group2" of level 2 and 2.1, etc. This helps to keep together tests with
// similar amounts of intended behavior/expected stress on devices.
var vp9Files = map[string]map[string]map[string][]string{
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

var vp8Files = map[string][]string{
	"inter_multi_coeff": {
		"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1408.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1409.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1410.ivf",
		//"test_vectors/vp8/inter_multi_coeff/vp80-03-segmentation-1413.ivf", // TODO(b/195789194)
		"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1404.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1405.ivf",
		"test_vectors/vp8/inter_multi_coeff/vp80-04-partitions-1406.ivf",
	},
	"inter_segment": {
		"test_vectors/vp8/inter_segment/vp80-03-segmentation-1407.ivf",
	},
	"inter": {
		"test_vectors/vp8/inter/vp80-02-inter-1402.ivf",
		//"test_vectors/vp8/inter/vp80-02-inter-1412.ivf", // TODO(b/195789194)
		"test_vectors/vp8/inter/vp80-02-inter-1418.ivf",
		"test_vectors/vp8/inter/vp80-02-inter-1424.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1403.ivf",
		//"test_vectors/vp8/inter/vp80-03-segmentation-1425.ivf", //TODO(b/195790894)
		"test_vectors/vp8/inter/vp80-03-segmentation-1426.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1427.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1432.ivf",
		"test_vectors/vp8/inter/vp80-03-segmentation-1435.ivf",
		//"test_vectors/vp8/inter/vp80-03-segmentation-1436.ivf", //TODO(b/195790894)
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
		//"test_vectors/vp8/intra/vp80-01-intra-1411.ivf", // TODO(b/195789194)
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
		//"test_vectors/vp8/vp80-00-comprehensive-006.ivf", // TODO(b/194908118)
		"test_vectors/vp8/vp80-00-comprehensive-007.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-008.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-009.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-010.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-011.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-012.ivf",
		"test_vectors/vp8/vp80-00-comprehensive-013.ivf",
		//"test_vectors/vp8/vp80-00-comprehensive-014.ivf", // TODO(b/194908118)
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
				files := vp9Files[profile][levelGroup][cat]
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
					hardwareDeps = append(hardwareDeps, "hwdep.SkipOnPlatform(\"grunt\", \"zork\")")
				}

				switch levelGroup {
				case "level5_0":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K)
				case "level5_1":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K60)
					hardwareDeps = append(hardwareDeps, "hwdep.MinMemory(4097)")
				default:
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9)
				}

				param.HardwareDeps = strings.Join(hardwareDeps, ", ")
				params = append(params, param)
			}
		}
	}

	// Generate VAAPI AV1 tests.
	params = append(params, paramData{
		Name:       "vaapi_av1",
		Decoder:    filepath.Join(chrome.BinTestDir, "decode_test"),
		CmdBuilder: "av1decodeVAAPIargs",
		Files:      vaapiAv1Files,
		Timeout:    defaultTimeout,
		// These SoftwareDeps do not include the 10 bit version of AV1.
		SoftwareDeps: []string{"vaapi", caps.HWDecodeAV1},
		Metadata:     genExtraData(vaapiAv1Files),
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

	// Generates V4L2 VP9 tests.
	for i, profile := range []string{"profile_0"} {
		for _, levelGroup := range []string{"group1", "group2", "group3", "group4", "level5_0", "level5_1"} {
			for _, cat := range []string{
				"buf", "frm_resize", "gf_dist", "odd_size", "sub8x8", "sub8x8_sf",
			} {
				files := vp9Files[profile][levelGroup][cat]
				param := paramData{
					Name:         fmt.Sprintf("v4l2_vp9_%d_%s_%s", i, levelGroup, cat),
					Decoder:      "v4l2_stateful_decoder",
					CmdBuilder:   "vp9decodeV4L2args",
					Files:        files,
					Timeout:      defaultTimeout,
					SoftwareDeps: []string{"v4l2_codec"},
					Metadata:     genExtraData(files),
					Attr:         []string{"graphics_video_vp9"},
				}
				if extension, ok := vp9GroupExtensions[levelGroup]; ok {
					param.Timeout = extension
				}

				hardwareDeps := []string{"hwdep.Platform(\"trogdor\")"}

				switch levelGroup {
				case "level5_0":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K)
				case "level5_1":
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9_4K60)
					hardwareDeps = append(hardwareDeps, "hwdep.MinMemory(4097)")
				default:
					param.SoftwareDeps = append(param.SoftwareDeps, caps.HWDecodeVP9)
				}

				param.HardwareDeps = strings.Join(hardwareDeps, ", ")
				params = append(params, param)
			}
		}
	}

	// Generate V4L2 VP8 tests.
	for _, testGroup := range []string{"inter", "inter_multi_coeff", "inter_segment", "intra", "intra_multi_coeff", "intra_segment", "comprehensive"} {
		files := vp8Files[testGroup]
		params = append(params, paramData{
			Name:         fmt.Sprintf("v4l2_vp8_%s", testGroup),
			Decoder:      "v4l2_stateful_decoder",
			CmdBuilder:   "vp8decodeV4L2args",
			Files:        files,
			Timeout:      defaultTimeout,
			SoftwareDeps: []string{"v4l2_codec", caps.HWDecodeVP8},
			Metadata:     genExtraData(files),
			Attr:         []string{"graphics_video_vp8"},
		})
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
