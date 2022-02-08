// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/bundles/cros/video/playback"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
)

type playbackPerfParams struct {
	fileName    string
	decoderType playback.DecoderType
	browserType browser.Type
	gridSize    int // Creates a layout of |gridSize| x |gridSize| videos for playback.
}

func createPlaybackParams(fileName string, decoderType playback.DecoderType, browserType browser.Type, gridSize int) playbackPerfParams {
	params := playbackPerfParams{}
	params.fileName = fileName
	params.decoderType = decoderType
	params.browserType = browserType
	params.gridSize = gridSize

	return params
}

func createPlaybackParamsForHwOnAsh(fileName string) playbackPerfParams {
	return createPlaybackParams(fileName, playback.Hardware, browser.TypeAsh, 1)
}

func createPlaybackParamsForSwOnAsh(fileName string) playbackPerfParams {
	return createPlaybackParams(fileName, playback.Software, browser.TypeAsh, 1)
}

func createPlaybackParamsWithDecoderType(fileName string, decoderType playback.DecoderType, browserType browser.Type) playbackPerfParams {
	return createPlaybackParams(fileName, decoderType, browserType, 1)
}

func createPlaybackParamsWithGridSize(fileName string, gridSize int) playbackPerfParams {
	return createPlaybackParams(fileName, playback.Hardware, browser.TypeAsh, gridSize)
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures video playback performance in Chrome browser with/without HW acceleration",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         playback.DataFiles(),
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name:              "h264_144p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("144p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"144p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_240p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("240p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"240p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_360p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("360p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"360p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_480p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("480p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_720p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("720p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_720p_30fps_hw_lacros",
			Val:               createPlaybackParamsWithDecoderType("720p_30fps_300frames.h264.mp4", playback.Hardware, browser.TypeLacros),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "lacros"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name:              "h264_720p_30fps_hw_3x3",
			Val:               createPlaybackParamsWithGridSize("720p_30fps_300frames.h264.mp4", 3),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_1080p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"1080p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_1080p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "proprietary_codecs"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_2160p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "proprietary_codecs"},
			ExtraData:         []string{"2160p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "h264_2160p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_60fps_600frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "proprietary_codecs"},
			ExtraData:         []string{"2160p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_144p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("144p_30fps_300frames.vp8.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"144p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_240p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("240p_30fps_300frames.vp8.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"240p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_360p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("360p_30fps_300frames.vp8.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"360p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_480p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("480p_30fps_300frames.vp8.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_720p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("720p_30fps_300frames.vp8.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_720p_30fps_hw_3x3",
			Val:               createPlaybackParamsWithGridSize("720p_30fps_300frames.vp8.webm", 3),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_1080p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_30fps_300frames.vp8.webm"),
			ExtraData:         []string{"1080p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_1080p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.vp8.webm"),
			ExtraData:         []string{"1080p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_2160p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_30fps_300frames.vp8.webm"),
			ExtraData:         []string{"2160p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp8_2160p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_60fps_600frames.vp8.webm"),
			ExtraData:         []string{"2160p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_144p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("144p_30fps_300frames.vp9.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"144p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_240p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("240p_30fps_300frames.vp9.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"240p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_360p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("360p_30fps_300frames.vp9.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"360p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_480p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("480p_30fps_300frames.vp9.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_720p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("720p_30fps_300frames.vp9.webm"),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_720p_30fps_hw_lacros",
			Val:               createPlaybackParamsWithDecoderType("720p_30fps_300frames.vp9.webm", playback.Hardware, browser.TypeLacros),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "lacros"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name:              "vp9_720p_30fps_hw_3x3",
			Val:               createPlaybackParamsWithGridSize("720p_30fps_300frames.vp9.webm", 3),
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_1080p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_30fps_300frames.vp9.webm"),
			ExtraData:         []string{"1080p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_1080p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.vp9.webm"),
			ExtraData:         []string{"1080p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_2160p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_30fps_300frames.vp9.webm"),
			ExtraData:         []string{"2160p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K},
			Fixture:           "chromeVideo",
		}, {
			Name:              "vp9_2160p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_60fps_600frames.vp9.webm"),
			ExtraData:         []string{"2160p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60},
			Fixture:           "chromeVideo",
		}, {
			Name:              "av1_480p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("480p_30fps_300frames.av1.mp4"),
			ExtraData:         []string{"480p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "av1_720p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("720p_30fps_300frames.av1.mp4"),
			ExtraData:         []string{"720p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "av1_720p_30fps_hw_3x3",
			Val:               createPlaybackParamsWithGridSize("720p_30fps_300frames.av1.mp4", 3),
			ExtraData:         []string{"720p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "av1_720p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("720p_60fps_600frames.av1.mp4"),
			ExtraData:         []string{"720p_60fps_600frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_60},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "av1_1080p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_30fps_300frames.av1.mp4"),
			ExtraData:         []string{"1080p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "av1_1080p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.av1.mp4"),
			ExtraData:         []string{"1080p_60fps_600frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_60},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "av1_2160p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_30fps_300frames.av1.mp4"),
			ExtraData:         []string{"2160p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_4K},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "av1_2160p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_60fps_600frames.av1.mp4"),
			ExtraData:         []string{"2160p_60fps_600frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_4K60},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name:              "hevc_144p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("144p_30fps_300frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"144p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_240p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("240p_30fps_300frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"240p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_360p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("360p_30fps_300frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"360p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_480p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("480p_30fps_300frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_720p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("720p_30fps_300frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_1080p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_30fps_300frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"1080p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_1080p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC60, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"1080p_60fps_600frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_2160p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_30fps_300frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"2160p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc_2160p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_60fps_600frames.hevc.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K60, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"2160p_60fps_600frames.hevc.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc10_2160p_30fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_30fps_300frames.hevc10.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K10BPP, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"2160p_30fps_300frames.hevc10.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "hevc10_2160p_60fps_hw",
			Val:               createPlaybackParamsForHwOnAsh("2160p_60fps_600frames.hevc10.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K60_10BPP, "proprietary_codecs", "protected_content"},
			ExtraData:         []string{"2160p_60fps_600frames.hevc10.mp4"},
			Fixture:           "chromeVideoWithClearHEVCHWDecoding",
		}, {
			Name:              "h264_480p_30fps_sw",
			Val:               createPlaybackParamsForSwOnAsh("480p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name:              "h264_720p_30fps_sw",
			Val:               createPlaybackParamsForSwOnAsh("720p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name:              "h264_1080p_30fps_sw",
			Val:               createPlaybackParamsForSwOnAsh("1080p_30fps_300frames.h264.mp4"),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraData:         []string{"1080p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name:              "h264_1080p_60fps_sw",
			Val:               createPlaybackParamsForSwOnAsh("1080p_60fps_600frames.h264.mp4"),
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp8_480p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("480p_30fps_300frames.vp8.webm"),
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"480p_30fps_300frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp8_720p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("720p_30fps_300frames.vp8.webm"),
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"720p_30fps_300frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp8_1080p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("1080p_30fps_300frames.vp8.webm"),
			ExtraData: []string{"1080p_30fps_300frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp8_1080p_60fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("1080p_60fps_600frames.vp8.webm"),
			ExtraData: []string{"1080p_60fps_600frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp9_480p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("480p_30fps_300frames.vp9.webm"),
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"480p_30fps_300frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp9_720p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("720p_30fps_300frames.vp9.webm"),
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"720p_30fps_300frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp9_1080p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("1080p_30fps_300frames.vp9.webm"),
			ExtraData: []string{"1080p_30fps_300frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "vp9_1080p_60fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("1080p_60fps_600frames.vp9.webm"),
			ExtraData: []string{"1080p_60fps_600frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "av1_480p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("480p_30fps_300frames.av1.mp4"),
			ExtraData: []string{"480p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "av1_720p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("720p_30fps_300frames.av1.mp4"),
			ExtraData: []string{"720p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "av1_720p_60fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("720p_60fps_600frames.av1.mp4"),
			ExtraData: []string{"720p_60fps_600frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "av1_1080p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("1080p_30fps_300frames.av1.mp4"),
			ExtraData: []string{"1080p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "av1_1080p_60fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("1080p_60fps_600frames.av1.mp4"),
			ExtraData: []string{"1080p_60fps_600frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "av1_2160p_30fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("2160p_30fps_300frames.av1.mp4"),
			ExtraData: []string{"2160p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:      "av1_2160p_60fps_sw",
			Val:       createPlaybackParamsForSwOnAsh("2160p_60fps_600frames.av1.mp4"),
			ExtraData: []string{"2160p_60fps_600frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name:              "av1_480p_30fps_sw_gav1",
			Val:               createPlaybackParamsWithDecoderType("480p_30fps_300frames.av1.mp4", playback.LibGAV1, browser.TypeAsh),
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"480p_30fps_300frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name:              "av1_720p_30fps_sw_gav1",
			Val:               createPlaybackParamsWithDecoderType("720p_30fps_300frames.av1.mp4", playback.LibGAV1, browser.TypeAsh),
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"720p_30fps_300frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name:              "av1_720p_60fps_sw_gav1",
			Val:               createPlaybackParamsWithDecoderType("720p_60fps_600frames.av1.mp4", playback.LibGAV1, browser.TypeAsh),
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"720p_60fps_600frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name:              "av1_1080p_30fps_sw_gav1",
			Val:               createPlaybackParamsWithDecoderType("1080p_30fps_300frames.av1.mp4", playback.LibGAV1, browser.TypeAsh),
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"1080p_30fps_300frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name:              "av1_1080p_60fps_sw_gav1",
			Val:               createPlaybackParamsWithDecoderType("1080p_60fps_600frames.av1.mp4", playback.LibGAV1, browser.TypeAsh),
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"1080p_60fps_600frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name:              "h264_1080p_60fps_hw_alt",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.h264.mp4"),
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "video_decoder_legacy_supported", "proprietary_codecs"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name:              "vp8_1080p_60fps_hw_alt",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.vp8.webm"),
			ExtraData:         []string{"1080p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name:              "vp9_1080p_60fps_hw_alt",
			Val:               createPlaybackParamsForHwOnAsh("1080p_60fps_600frames.vp9.webm"),
			ExtraData:         []string{"1080p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name:              "vp9_2160p_60fps_hw_alt",
			Val:               createPlaybackParamsForHwOnAsh("2160p_60fps_600frames.vp9.webm"),
			ExtraData:         []string{"2160p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}},
	})
}

// PlaybackPerf plays a video in the Chrome browser and measures the performance with or without
// HW decode acceleration as per DecoderType. The values are reported to the performance dashboard.
func PlaybackPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(playbackPerfParams)

	_, l, cs, err := lacros.Setup(ctx, s.FixtValue(), testOpt.browserType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(ctx, l)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	playback.RunTest(ctx, s, cs, cr, testOpt.fileName, testOpt.decoderType, testOpt.gridSize)
}
