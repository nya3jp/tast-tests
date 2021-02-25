// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"fmt"
	"path/filepath"
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
)

// NB: If modifying any of the files or test specifications, be sure to
// regenerate the test parameters by running the following in a chroot:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/video

var v4l2Vp9Files = []string{"1080p_30fps_300frames.vp9.ivf"}

var vaapiVp9Files = map[string]map[string]map[string][]string{
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
		// TODO(jchinlee): enable levels 4 and above when we have a better
		// understanding of runtime on slower devices.
		/*"group4": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_2048X1088_fr30_bd8_8buf_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_2048X1088_fr30_bd8_8buf_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_2048X1088_fr30_bd8_8buf_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_2048X1088_fr60_bd8_6buf_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_2048X1088_fr60_bd8_6buf_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_2048X1088_fr60_bd8_6buf_l4-l411.ivf",
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
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_2048X1088_fr30_bd8_gf_dist_4_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_2048X1088_fr30_bd8_gf_dist_4_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_2048X1088_fr30_bd8_gf_dist_4_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_2048X1088_fr60_bd8_gf_dist_5_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_2048X1088_fr60_bd8_gf_dist_5_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_2048X1088_fr60_bd8_gf_dist_5_l4-l411.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_2040X1080_fr30_bd8_odd_size_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_2040X1080_fr30_bd8_odd_size_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_2040X1080_fr30_bd8_odd_size_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_2040X1080_fr60_bd8_odd_size_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_2040X1080_fr60_bd8_odd_size_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_2040X1080_fr60_bd8_odd_size_l4-l411.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_2048X1088_fr30_bd8_sub8X8_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_2048X1088_fr30_bd8_sub8X8_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_2048X1088_fr30_bd8_sub8X8_l4-l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_2048X1088_fr60_bd8_sub8X8_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_2048X1088_fr60_bd8_sub8X8_l4-l411.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_2048X1088_fr60_bd8_sub8X8_l4-l411.ivf",
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
		"group5": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_4096X2176_fr60_bd8_4buf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_4096X2176_fr60_bd8_4buf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_4096X2176_fr60_bd8_4buf_l51.ivf",
			},
			"frm_resize": {
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_4096X2176_fr30_bd8_frm_resize_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_4096X2176_fr30_bd8_frm_resize_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_4096X2176_fr30_bd8_frm_resize_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/crowd_run_4096X2176_fr60_bd8_frm_resize_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/grass_1_4096X2176_fr60_bd8_frm_resize_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/frm_resize/street1_1_4096X2176_fr60_bd8_frm_resize_l51.ivf",
			},
			"gf_dist": {
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_4096X2176_fr30_bd8_gf_dist_6_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_4096X2176_fr30_bd8_gf_dist_6_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_4096X2176_fr30_bd8_gf_dist_6_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/crowd_run_4096X2176_fr60_bd8_gf_dist_10_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/grass_1_4096X2176_fr60_bd8_gf_dist_10_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/gf_dist/street1_1_4096X2176_fr60_bd8_gf_dist_10_l51.ivf",
			},
			"odd_size": {
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_4088X2168_fr30_bd8_odd_size_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_4088X2168_fr30_bd8_odd_size_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_4088X2168_fr30_bd8_odd_size_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/crowd_run_4088X2168_fr60_bd8_odd_size_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/grass_1_4088X2168_fr60_bd8_odd_size_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/odd_size/street1_1_4088X2168_fr60_bd8_odd_size_l51.ivf",
			},
			"sub8x8": {
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_4096X2176_fr30_bd8_sub8X8_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_4096X2176_fr30_bd8_sub8X8_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_4096X2176_fr30_bd8_sub8X8_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/crowd_run_4096X2176_fr60_bd8_sub8X8_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/grass_1_4096X2176_fr60_bd8_sub8X8_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8X8/street1_1_4096X2176_fr60_bd8_sub8X8_l51.ivf",
			},
			"sub8x8_sf": {
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_4096X2176_fr30_bd8_sub8x8_sf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_4096X2176_fr30_bd8_sub8x8_sf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_4096X2176_fr30_bd8_sub8x8_sf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/crowd_run_4096X2176_fr60_bd8_sub8X8_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/grass_1_4096X2176_fr60_bd8_sub8x8_sf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/sub8x8_sf/street1_1_4096X2176_fr60_bd8_sub8x8_sf_l51.ivf",
			},
		},*/
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
		HardwareDeps string
		SoftwareDeps []string
		Metadata     []string
	}

	var params []paramData

	// Generate VAAPI VP9 tests.
	for i, profile := range []string{"profile_0"} {
		for _, levelGroup := range []string{"group1", "group2", "group3"} {
			for _, cat := range []string{
				"buf", "frm_resize", "gf_dist", "odd_size", "sub8x8", "sub8x8_sf",
			} {
				files := vaapiVp9Files[profile][levelGroup][cat]
				params = append(params, paramData{
					Name:         fmt.Sprintf("vaapi_vp9_%d_%s_%s", i, levelGroup, cat),
					Decoder:      filepath.Join(chrome.BinTestDir, "decode_test"),
					CmdBuilder:   "vp9decodeVAAPIargs",
					Files:        files,
					SoftwareDeps: []string{"vaapi", caps.HWDecodeVP9},
					Metadata:     genExtraData(files),
				})
			}
		}
	}

	// Generate V4L2 VP9 tests.
	params = append(params, paramData{
		Name:         "v4l2_vp9",
		Decoder:      "v4l2_stateful_decoder",
		CmdBuilder:   "vp9decodeV4L2args",
		Files:        v4l2Vp9Files,
		HardwareDeps: "hwdep.D(hwdep.Platform(\"trogdor\"))",
		SoftwareDeps: []string{"v4l2_codec", caps.HWDecodeVP9},
		Metadata:     genExtraData(v4l2Vp9Files),
	})

	code := genparams.Template(t, `{{ range . }}{
		Name: {{ .Name | fmt }},
		Val:  platformDecodingParams{
			filenames: {{ .Files | fmt }},
			decoder: {{ .Decoder |fmt }},
			commandBuilder: {{ .CmdBuilder }},
		},
		{{ if .HardwareDeps }}
		ExtraHardwareDeps: {{ .HardwareDeps }},
		{{ end }}
		{{ if .SoftwareDeps }}
		ExtraSoftwareDeps: {{ .SoftwareDeps | fmt }},
		{{ end }}
		ExtraData: {{ .Metadata | fmt }},
		Timeout: 10 * time.Minute,
	},
	{{ end }}`, params)
	genparams.Ensure(t, "platform_decoding.go", code)
}
