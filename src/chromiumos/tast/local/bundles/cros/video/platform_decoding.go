// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type failExpectedFn func(stdout, stderr []byte) bool

type platformDecodingParams struct {
	filenames    []string
	failExpected failExpectedFn
}

var vp9Files = map[string]map[string]map[string][]string{
	"profile_0": {
		"l1ish": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_256X144_fr15_bd8_8buf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_256X144_fr15_bd8_8buf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_256X144_fr15_bd8_8buf_l1.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_384X192_fr30_bd8_8buf_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_384X192_fr30_bd8_8buf_l11.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_384X192_fr30_bd8_8buf_l11.ivf",
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
		},
		"l2ish": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_480X256_fr30_bd8_8buf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_480X256_fr30_bd8_8buf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_480X256_fr30_bd8_8buf_l2.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_640X384_fr30_bd8_8buf_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_640X384_fr30_bd8_8buf_l21.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_640X384_fr30_bd8_8buf_l21.ivf",
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
		},
		"l3ish": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_1080X512_fr30_bd8_8buf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_1080X512_fr30_bd8_8buf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_1080X512_fr30_bd8_8buf_l3.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_1280X768_fr30_bd8_8buf_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_1280X768_fr30_bd8_8buf_l31.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_1280X768_fr30_bd8_8buf_l31.ivf",
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
		},
		// Omit l4 and above until we have a better understanding of runtime on
		// slower devices.
		/*"l4ish": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_2048X1088_fr30_bd8_8buf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_2048X1088_fr30_bd8_8buf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_2048X1088_fr30_bd8_8buf_l4.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_2048X1088_fr60_bd8_6buf_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_2048X1088_fr60_bd8_6buf_l41.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_2048X1088_fr60_bd8_6buf_l41.ivf",
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
		},
		"l5ish": {
			"buf": {
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_4096X2176_fr30_bd8_4buf_l5.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/crowd_run_4096X2176_fr60_bd8_4buf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/grass_1_4096X2176_fr60_bd8_4buf_l51.ivf",
				"test_vectors/vp9/Profile_0_8bit/buf/street1_1_4096X2176_fr60_bd8_4buf_l51.ivf",
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

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformDecoding,
		Desc: "Smoke tests libva decoding by running the media/gpu/vaapi/test:decode_test binary",
		Contacts: []string{
			"jchinlee@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"vaapi"},
		Params: []testing.Param{{
			Name: "vp9_0_l1ish_buf",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l1ish"]["buf"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l1ish"]["buf"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l1ish_gf_dist",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l1ish"]["gf_dist"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l1ish"]["gf_dist"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l1ish_odd_size",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l1ish"]["odd_size"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l1ish"]["odd_size"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l1ish_sub8x8",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l1ish"]["sub8X8"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l1ish"]["sub8X8"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l2ish_buf",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l2ish"]["buf"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l2ish"]["buf"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l2ish_gf_dist",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l2ish"]["gf_dist"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l2ish"]["gf_dist"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l2ish_odd_size",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l2ish"]["odd_size"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l2ish"]["odd_size"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l2ish_sub8x8",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l2ish"]["sub8X8"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l2ish"]["sub8X8"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l3ish_buf",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l3ish"]["buf"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l3ish"]["buf"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l3ish_gf_dist",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l3ish"]["gf_dist"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l3ish"]["gf_dist"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l3ish_odd_size",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l3ish"]["odd_size"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l3ish"]["odd_size"]),
			Timeout:           10 * time.Minute,
		}, {
			Name: "vp9_0_l3ish_sub8x8",
			Val: platformDecodingParams{
				filenames:    vp9Files["profile_0"]["l3ish"]["sub8X8"],
				failExpected: nil,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			ExtraData:         genExtraData(vp9Files["profile_0"]["l3ish"]["sub8X8"]),
			Timeout:           10 * time.Minute,
		}, {
			// TODO(jchinlee): enable these when we have a better understanding of
			// runtime on slower devices.
			/*Name: "vp9_0_l4ish_buf",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l4ish"]["buf"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l4ish"]["buf"]),
				Timeout:           10 * time.Minute,
			}, {
				Name: "vp9_0_l4ish_gf_dist",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l4ish"]["gf_dist"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l4ish"]["gf_dist"]),
				Timeout:           10 * time.Minute,
			}, {
				Name: "vp9_0_l4ish_odd_size",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l4ish"]["odd_size"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l4ish"]["odd_size"]),
				Timeout:           10 * time.Minute,
			}, {
				Name: "vp9_0_l4ish_sub8x8",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l4ish"]["sub8X8"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l4ish"]["sub8X8"]),
				Timeout:           10 * time.Minute,
			}, {
				Name: "vp9_0_l5ish_buf",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l5ish"]["buf"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l5ish"]["buf"]),
				Timeout:           10 * time.Minute,
			}, {
				Name: "vp9_0_l5ish_gf_dist",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l5ish"]["gf_dist"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l5ish"]["gf_dist"]),
				Timeout:           10 * time.Minute,
			}, {
				Name: "vp9_0_l5ish_odd_size",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l5ish"]["odd_size"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l5ish"]["odd_size"]),
				Timeout:           10 * time.Minute,
			}, {
				Name: "vp9_0_l5ish_sub8x8",
				Val: platformDecodingParams{
					filenames:    vp9Files["profile_0"]["l5ish"]["sub8X8"],
					failExpected: nil,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         genExtraData(vp9Files["profile_0"]["l5ish"]["sub8X8"]),
				Timeout:           10 * time.Minute,
			}, {*/
			// Attempt to decode an unsupported codec to ensure that the binary is not
			// unconditionally succeeding, i.e. not crashing even when expected to.
			Name: "unsupported_codec_fail",
			Val: platformDecodingParams{
				filenames: []string{"resolution_change_500frames.vp8.ivf"},
				failExpected: func(stdout, stderr []byte) bool {
					return strings.Contains(string(stderr), "Codec VP80 not supported.")
				},
			},
			ExtraData: []string{"resolution_change_500frames.vp8.ivf", "resolution_change_500frames.vp8.ivf.json"},
		}},
	})
}

func verifyContent(expectedHashesPath, actualOutput string) error {
	// Read expected hashes from metadata json.
	metadataJSONBytes, err := ioutil.ReadFile(expectedHashesPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read metadata file at %s", expectedHashesPath)
	}

	var meta map[string]interface{}
	if err = json.Unmarshal(metadataJSONBytes, &meta); err != nil {
		return errors.Wrapf(err, "failed to read json from metadata file at %s", expectedHashesPath)
	}
	expected, ok := meta["md5_checksums"].([]interface{})
	if !ok {
		return errors.Errorf("`md5_checksums` in metadata at %s not a slice; got %v", expectedHashesPath, meta["md5_checksums"])
	}

	// Compare expected hashes to actual hashes.
	actual := strings.Split(strings.TrimSpace(actualOutput), "\n")
	if len(expected) != len(actual) {
		return errors.Errorf("expected and actual number of frames mismatched (%d != %d)", len(expected), len(actual))
	}

	var mismatched []string
	for i, ex := range expected {
		if _, ok := ex.(string); !ok {
			return errors.Errorf("failed to cast expected hash %v of type %T to string", ex, ex)
		}
		if wanted, got := strings.TrimSpace(ex.(string)), strings.TrimSpace(actual[i]); wanted != got {
			mismatched = append(mismatched, fmt.Sprintf("frame %d (%s != %s)", i, wanted, got))
		}
	}

	if mismatched != nil {
		return errors.Wrap(errors.New("mismatched hashes"), strings.Join(mismatched, "\n"))
	}

	return nil
}

// PlatformDecoding runs the media/gpu/vaapi/test:decode_test binary on the
// file specified in the testing state. The test fails if any of the VAAPI calls
// fail (or if the test is incorrectly invoked): notably, the binary does not
// check for correctness of decoded output. This test is motivated by instances
// in which libva uprevs may introduce regressions and cause decoding to break
// for reasons unrelated to Chrome.
func PlatformDecoding(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(platformDecodingParams)
	const cleanupTime = 90 * time.Second

	// Reserve time to restart the ui job and perform cleanup at the end of the test.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	vl, err := logging.NewVideoLogger()
	if err != nil {
		s.Fatal("Failed to create new video logger: ", err)
	}
	defer vl.Close()

	// Only a single process can have access to the GPU. We do not strictly need
	// to `stop ui` to run the binary, but still do so to shut down the browser
	// and improve benchmarking accuracy.
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(cleanupCtx, "ui")

	// Run the decode_test binary on all files, failing at the first error.
	// The decode_test binary fails if the VAAPI calls themselves error, the
	// binary is called on unsupported inputs or could not open the DRI render
	// node, or the binary otherwise crashes.
	// The test may also fail to verify the decode results (MD5 comparison).
	const exec = "decode_test"
	for _, filename := range testOpt.filenames {
		testing.ContextLogf(ctx, "Running %s on %s", exec, filename)
		stdout, stderr, err := testexec.CommandContext(
			ctx,
			filepath.Join(chrome.BinTestDir, exec),
			"--video="+s.DataPath(filename),
			"--visible",
			"--md5",
		).SeparatedOutput(testexec.DumpLogOnError)

		if err != nil && (testOpt.failExpected == nil || !testOpt.failExpected(stdout, stderr)) {
			output := append(stdout, stderr...)
			s.Fatalf("%v failed unexpectedly: %v", exec, errors.Wrap(err, string(output)))
		}
		if err == nil && testOpt.failExpected != nil {
			s.Fatalf("%v passed on %s when expected to fail", exec, filename)
		}
		if testOpt.failExpected != nil && testOpt.failExpected(stdout, stderr) {
			continue
		}

		if err := verifyContent(s.DataPath(filename+".json"), string(stdout)); err != nil {
			s.Fatalf("%v failed to verify content: %v", exec, errors.Wrap(err, filename))
		}
	}
}
