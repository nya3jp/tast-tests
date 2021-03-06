// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

// seekTest is used to describe the config used to run each Seek test.
type seekTest struct {
	filename   string // File name to play back.
	numSeeks   int    // Amount of times to seek into the <video>.
	chromeType lacros.ChromeType
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Seek,
		Desc: "Verifies that seeking works in Chrome, either with or without resolution changes",
		Contacts: []string{
			"acourbot@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"video.html"},
		Params: []testing.Param{{
			Name: "av1",
			Val: seekTest{
				filename:   "720_av1.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "h264",
			Val: seekTest{
				filename:   "720_h264.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_lacros",
			Val: seekTest{
				filename:   "720_h264.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_h264.mp4", launcher.DataArtifact},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "lacros"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name: "hevc",
			Val: seekTest{
				filename:   "720_hevc.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_hevc.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name: "vp8",
			Val: seekTest{
				filename:   "720_vp8.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9",
			Val: seekTest{
				filename:   "720_vp9.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_lacros",
			Val: seekTest{
				filename:   "720_vp9.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_vp9.webm", launcher.DataArtifact},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "lacros"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name: "switch_av1",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.av1.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.av1.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "switch_h264",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.h264.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Fixture:           "chromeVideo",
		}, {
			Name: "switch_hevc",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.hevc.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.hevc.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name: "switch_vp8",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.vp8.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "switch_vp9",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.vp9.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "stress_av1",
			Val: seekTest{
				filename:   "720_av1.mp4",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "stress_vp8",
			Val: seekTest{
				filename:   "720_vp8.webm",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeVideo",
		}, {
			Name: "stress_vp9",
			Val: seekTest{
				filename:   "720_vp9.webm",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeVideo",
		}, {
			Name: "stress_h264",
			Val: seekTest{
				filename:   "720_h264.mp4",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeVideo",
		}, {
			Name: "stress_hevc",
			Val: seekTest{
				filename:   "720_hevc.mp4",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_hevc.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name: "h264_alt",
			Val: seekTest{
				filename:   "720_h264.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp8_alt",
			Val: seekTest{
				filename:   "720_vp8.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp9_alt",
			Val: seekTest{
				filename:   "720_vp9.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"720_vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "switch_h264_alt",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.h264.mp4",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "switch_vp8_alt",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.vp8.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "switch_vp9_alt",
			Val: seekTest{
				filename:   "smpte_bars_resolution_ladder.vp9.webm",
				numSeeks:   25,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
			ExtraData:         []string{"smpte_bars_resolution_ladder.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "stress_vp8_alt",
			Val: seekTest{
				filename:   "720_vp8.webm",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "video_decoder_legacy_supported"},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "stress_vp9_alt",
			Val: seekTest{
				filename:   "720_vp9.webm",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "video_decoder_legacy_supported"},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "stress_h264_alt",
			Val: seekTest{
				filename:   "720_h264.mp4",
				numSeeks:   1000,
				chromeType: lacros.ChromeTypeChromeOS,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_weekly"},
			ExtraData:         []string{"720_h264.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "video_decoder_legacy_supported", "proprietary_codecs"},
			Timeout:           20 * time.Minute,
			Fixture:           "chromeAlternateVideoDecoder",
		}},
	})
}

// Seek plays a file with Chrome and checks that it can safely be seeked into.
func Seek(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(seekTest)

	// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
	var artifactPath string
	if testOpt.chromeType == lacros.ChromeTypeLacros {
		if !strings.Contains(s.TestName(), "lacros") {
			s.Fatal("Non-lacros test run with lacros chrome-type")
		}
		artifactPath = s.DataPath(launcher.DataArtifact)
	}
	_, l, cs, err := lacros.Setup(ctx, s.FixtValue(), artifactPath, testOpt.chromeType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacrosChrome(ctx, l)

	if err := play.TestSeek(ctx, http.FileServer(s.DataFileSystem()), cs, testOpt.filename, testOpt.numSeeks); err != nil {
		s.Fatal("TestSeek failed: ", err)
	}
}
