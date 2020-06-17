// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type chromePIPEnergyAndPowerTestParams struct {
	tabletMode    bool
	bigPIP        bool
	videoFileName string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePIPEnergyAndPower,
		Desc:         "Measures energy and power usage of Chrome PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		Data:         []string{"pip.html"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:      "clamshell_small_av1",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "clamshell_small_h264",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "clamshell_small_vp8",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "clamshell_small_vp9",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "clamshell_small_vp9_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:              "clamshell_small_h264_sw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_small_vp8_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_small_vp9_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_small_vp9_2_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData: []string{"bear-320x240.vp9.2.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:              "clamshell_small_vp9_sw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithSWDecodingAndHDRScreen(),
		}, {
			Name:              "clamshell_small_h264_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_small_vp8_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_small_vp9_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_small_vp9_2_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_small_vp9_hw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:      "clamshell_small_av1_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "clamshell_small_h264_guest",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "clamshell_small_vp8_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "clamshell_small_vp9_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "clamshell_small_h264_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "clamshell_small_vp8_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP8},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "clamshell_small_vp9_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP9},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:      "tablet_small_av1",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "tablet_small_h264",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "tablet_small_vp8",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "tablet_small_vp9",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "tablet_small_vp9_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:              "tablet_small_h264_sw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_small_vp8_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_small_vp9_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_small_vp9_2_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData: []string{"bear-320x240.vp9.2.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:              "tablet_small_vp9_sw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithSWDecodingAndHDRScreen(),
		}, {
			Name:              "tablet_small_h264_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_small_vp8_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_small_vp9_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_small_vp9_2_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_small_vp9_hw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:      "tablet_small_av1_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "tablet_small_h264_guest",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "tablet_small_vp8_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "tablet_small_vp9_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "tablet_small_h264_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "tablet_small_vp8_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP8},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "tablet_small_vp9_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP9},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:      "clamshell_big_av1",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "clamshell_big_h264",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "clamshell_big_vp8",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "clamshell_big_vp9",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "clamshell_big_vp9_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:              "clamshell_big_h264_sw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_big_vp8_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_big_vp9_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "clamshell_big_vp9_2_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData: []string{"bear-320x240.vp9.2.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:              "clamshell_big_vp9_sw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithSWDecodingAndHDRScreen(),
		}, {
			Name:              "clamshell_big_h264_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_big_vp8_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_big_vp9_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_big_vp9_2_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "clamshell_big_vp9_hw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:      "clamshell_big_av1_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "clamshell_big_h264_guest",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "clamshell_big_vp8_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "clamshell_big_vp9_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "clamshell_big_h264_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "clamshell_big_vp8_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP8},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "clamshell_big_vp9_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP9},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:      "tablet_big_av1",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "tablet_big_h264",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:      "tablet_big_vp8",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:      "tablet_big_vp9",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideo(),
		}, {
			Name:              "tablet_big_vp9_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:              "tablet_big_h264_sw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_big_vp8_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_big_vp9_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:      "tablet_big_vp9_2_sw",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData: []string{"bear-320x240.vp9.2.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name:              "tablet_big_vp9_sw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			Pre:               pre.ChromeVideoWithSWDecodingAndHDRScreen(),
		}, {
			Name:              "tablet_big_h264_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_big_vp8_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_big_vp9_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_big_vp9_2_hw",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.2.webm"},
			ExtraData:         []string{"bear-320x240.vp9.2.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideo(),
		}, {
			Name:              "tablet_big_vp9_hw_hdr",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "peru.8k.cut.hdr.vp9.webm"},
			ExtraData:         []string{"peru.8k.cut.hdr.vp9.webm"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kohaku")),
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_2},
			Pre:               pre.ChromeVideoWithHDRScreen(),
		}, {
			Name:      "tablet_big_av1_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "360p_30fps_300frames.av1.mp4"},
			ExtraData: []string{"360p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "tablet_big_h264_guest",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "tablet_big_vp8_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData: []string{"bear-320x240.vp8.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:      "tablet_big_vp9_guest",
			Val:       chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData: []string{"bear-320x240.vp9.webm"},
			Pre:       pre.ChromeVideoWithGuestLogin(),
		}, {
			Name:              "tablet_big_h264_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.h264.mp4"},
			ExtraData:         []string{"bear-320x240.h264.mp4"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeH264, "chrome_internal"}, // "chrome_internal" is needed because H.264 is a proprietary codec.
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "tablet_big_vp8_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp8.webm"},
			ExtraData:         []string{"bear-320x240.vp8.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP8},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name:              "tablet_big_vp9_hw_alt",
			Val:               chromePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true, videoFileName: "bear-320x240.vp9.webm"},
			ExtraData:         []string{"bear-320x240.vp9.webm"},
			ExtraSoftwareDeps: []string{"cros_video_decoder", caps.HWDecodeVP9},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}},
	})
}

func ChromePIPEnergyAndPower(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	params := s.Param().(chromePIPEnergyAndPowerTestParams)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet/clamshell mode: ", err)
	}
	defer cleanup(ctx)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	var pointerController pointer.Controller
	if params.tabletMode {
		pointerController, err = pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create touch controller: ", err)
		}
	} else {
		pointerController = pointer.NewMouseController(tconn)
	}
	defer pointerController.Close()

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for low CPU usage: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cr.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer conn.Close()

	if err := conn.Call(ctx, nil, "startVideo", params.videoFileName); err != nil {
		s.Fatal("Failed to start video: ", err)
	}

	var pipButtonCenterString string
	if err := conn.Call(ctx, &pipButtonCenterString, "getPIPButtonCenter"); err != nil {
		s.Fatal("Failed to get center of PIP button: ", err)
	}

	var pipButtonCenterInWebContents coords.Point
	if n, err := fmt.Sscanf(pipButtonCenterString, "%v,%v", &pipButtonCenterInWebContents.X, &pipButtonCenterInWebContents.Y); err != nil {
		s.Fatalf("Failed to parse center of PIP button (successfully parsed %v of 2 tokens): %v", n, err)
	}

	webContentsView, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: "WebContentsViewAura"})
	if err != nil {
		s.Fatal("Failed to get web contents view: ", err)
	}
	defer webContentsView.Release(ctx)

	if err := pointer.Click(ctx, pointerController, webContentsView.Location.TopLeft().Add(pipButtonCenterInWebContents)); err != nil {
		s.Fatal("Failed to click/tap PIP button: ", err)
	}

	pipWindowFindParams := chromeui.FindParams{Name: "Picture in picture", ClassName: "PictureInPictureWindow"}
	if err := chromeui.WaitUntilExists(ctx, tconn, pipWindowFindParams, time.Minute); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	if params.tabletMode {
		// Tap the PIP window in preparation for the resizing swipe. Otherwise, that
		// swipe will move the PIP window instead of resizing it.
		pipWindow, err := chromeui.Find(ctx, tconn, pipWindowFindParams)
		if err != nil {
			s.Fatal("Failed to get PIP window: ", err)
		}
		defer pipWindow.Release(ctx)
		if err := pointer.Click(ctx, pointerController, pipWindow.Location.CenterPoint()); err != nil {
			s.Fatal("Failed to tap center of PIP window: ", err)
		}
	}

	resizeHandle, err := chromeui.Find(ctx, tconn, chromeui.FindParams{Name: "Resize", ClassName: "ImageButton"})
	if err != nil {
		s.Fatal("Failed to get PIP resize handle: ", err)
	}
	defer resizeHandle.Release(ctx)

	var resizeDragEnd coords.Point
	if params.bigPIP {
		resizeDragEnd = info.WorkArea.TopLeft()
	} else {
		resizeDragEnd = info.WorkArea.BottomRight()
	}
	if err := pointer.Drag(ctx, pointerController, resizeHandle.Location.CenterPoint(), resizeDragEnd, time.Second); err != nil {
		s.Fatal("Failed to drag PIP resize handle: ", err)
	}

	if params.tabletMode {
		// Ensure that the PIP window has no controls (like Pause) showing.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait five seconds: ", err)
		}
	} else {
		// Ensure that the PIP window has no resize shadows showing.
		if err := pointerController.Move(ctx, resizeDragEnd, info.WorkArea.TopLeft().Add(coords.Point{X: 20, Y: 20}), time.Second); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}
	}

	extraConn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer extraConn.Close()

	if err := webutil.WaitForQuiescence(ctx, extraConn, time.Minute); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	if err := timeline.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}
	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}
	if err := testing.Sleep(ctx, time.Minute); err != nil {
		s.Fatal("Failed to wait a minute: ", err)
	}
	pv, err := timeline.StopRecording()
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
