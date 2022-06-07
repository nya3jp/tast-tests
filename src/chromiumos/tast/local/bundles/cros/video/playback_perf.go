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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type playbackPerfParams struct {
	fileName    string
	decoderType playback.DecoderType
	browserType browser.Type
	// Creates a layout of |gridSize| x |gridSize| videos for playback. Values
	// less than 1 are clamped to a grid of 1x1.
	gridSize int
	// If set, uses a longer video sequence which allows for measuring Media
	// Devtools "playback roughness".
	measureRoughness bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures video playback performance in Chrome browser with/without HW acceleration",
		Contacts: []string{
			"mcasas@chromium.org",
			"hiroh@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"video.html", "playback.js"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_144p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/144p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/144p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_240p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/240p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/240p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_360p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/360p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/360p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/480p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/480p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/720p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_720p_30fps_hw_lacros",
			Val: playbackPerfParams{
				fileName:    "perf/h264/720p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs", "lacros"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name: "h264_720p_30fps_hw_3x3",
			Val: playbackPerfParams{
				fileName:    "perf/h264/720p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
				gridSize:    3,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/1080p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"perf/h264/1080p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/1080p_60fps_600frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "proprietary_codecs"},
			ExtraData:         []string{"perf/h264/1080p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/2160p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "proprietary_codecs"},
			ExtraData:         []string{"perf/h264/2160p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/2160p_60fps_600frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "proprietary_codecs"},
			ExtraData:         []string{"perf/h264/2160p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_144p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/144p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp8/144p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_240p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/240p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp8/240p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_360p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/360p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp8/360p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/480p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp8/480p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/720p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp8/720p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_720p_30fps_hw_3x3",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/720p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
				gridSize:    3,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp8/720p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/1080p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp8/1080p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/1080p_60fps_600frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp8/1080p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/2160p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp8/2160p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/2160p_60fps_600frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp8/2160p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_144p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/144p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp9/144p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_240p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/240p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp9/240p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_360p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/360p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp9/360p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/480p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp9/480p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/720p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_720p_30fps_hw_lacros",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/720p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeLacros,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "lacros"},
			Fixture:           "chromeVideoLacros",
		}, {
			Name: "vp9_720p_30fps_hw_3x3",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/720p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
				gridSize:    3,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/1080p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp9/1080p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/1080p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp9/1080p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/2160p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp9/2160p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K},
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/2160p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp9/2160p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60},
			Fixture:           "chromeVideo",
		}, {
			Name: "av1_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/480p_30fps_300frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/av1/480p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/720p_30fps_300frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/av1/720p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_720p_30fps_hw_3x3",
			Val: playbackPerfParams{
				fileName:    "perf/av1/720p_30fps_300frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
				gridSize:    3,
			},
			ExtraData:         []string{"perf/av1/720p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_720p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/720p_60fps_600frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/av1/720p_60fps_600frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_60},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/1080p_30fps_300frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/av1/1080p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/1080p_60fps_600frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/av1/1080p_60fps_600frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_60},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/2160p_30fps_300frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/av1/2160p_30fps_300frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_4K},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "av1_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/2160p_60fps_600frames.av1.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/av1/2160p_60fps_600frames.av1.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1_4K60},
			Fixture:           "chromeVideoWithHWAV1Decoding",
		}, {
			Name: "hevc_144p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/144p_30fps_300frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/hevc/144p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_240p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/240p_30fps_300frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/hevc/240p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_360p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/360p_30fps_300frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/hevc/360p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/480p_30fps_300frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/hevc/480p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/720p_30fps_300frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/hevc/720p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/1080p_30fps_300frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraData:         []string{"perf/hevc/1080p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/1080p_60fps_600frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC60, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraData:         []string{"perf/hevc/1080p_60fps_600frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/2160p_30fps_300frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraData:         []string{"perf/hevc/2160p_30fps_300frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc/2160p_60fps_600frames.hevc.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K60, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraData:         []string{"perf/hevc/2160p_60fps_600frames.hevc.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc10_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc10/2160p_30fps_300frames.hevc10.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K10BPP, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraData:         []string{"perf/hevc10/2160p_30fps_300frames.hevc10.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "hevc10_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "perf/hevc10/2160p_60fps_600frames.hevc10.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeHEVC4K60_10BPP, "proprietary_codecs"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("trogdor", "strongbad")), // TODO(b/232255167): re-enable when HEVC decoding has been enabled on QC devices
			ExtraData:         []string{"perf/hevc10/2160p_60fps_600frames.hevc10.mp4"},
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/480p_30fps_300frames.h264.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/480p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "h264_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/720p_30fps_300frames.h264.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "h264_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/1080p_30fps_300frames.h264.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraData:         []string{"perf/h264/1080p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "h264_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/h264/1080p_60fps_600frames.h264.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"proprietary_codecs"},
			ExtraData:         []string{"perf/h264/1080p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "vp8_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/480p_30fps_300frames.vp8.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"perf/vp8/480p_30fps_300frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp8_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/720p_30fps_300frames.vp8.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"perf/vp8/720p_30fps_300frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp8_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/1080p_30fps_300frames.vp8.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/vp8/1080p_30fps_300frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp8_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/1080p_60fps_600frames.vp8.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/vp8/1080p_60fps_600frames.vp8.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/480p_30fps_300frames.vp9.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"perf/vp9/480p_30fps_300frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/720p_30fps_300frames.vp9.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/1080p_30fps_300frames.vp9.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/vp9/1080p_30fps_300frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/1080p_60fps_600frames.vp9.webm",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/vp9/1080p_60fps_600frames.vp9.webm"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/480p_30fps_300frames.av1.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/av1/480p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/720p_30fps_300frames.av1.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/av1/720p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_720p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/720p_60fps_600frames.av1.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/av1/720p_60fps_600frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/1080p_30fps_300frames.av1.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/av1/1080p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/1080p_60fps_600frames.av1.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/av1/1080p_60fps_600frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_2160p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/2160p_30fps_300frames.av1.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/av1/2160p_30fps_300frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_2160p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "perf/av1/2160p_60fps_600frames.av1.mp4",
				decoderType: playback.Software,
				browserType: browser.TypeAsh,
			},
			ExtraData: []string{"perf/av1/2160p_60fps_600frames.av1.mp4"},
			Fixture:   "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_480p_30fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "perf/av1/480p_30fps_300frames.av1.mp4",
				decoderType: playback.LibGAV1,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"perf/av1/480p_30fps_300frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name: "av1_720p_30fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "perf/av1/720p_30fps_300frames.av1.mp4",
				decoderType: playback.LibGAV1,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"perf/av1/720p_30fps_300frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name: "av1_720p_60fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "perf/av1/720p_60fps_600frames.av1.mp4",
				decoderType: playback.LibGAV1,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"perf/av1/720p_60fps_600frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name: "av1_1080p_30fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "perf/av1/1080p_30fps_300frames.av1.mp4",
				decoderType: playback.LibGAV1,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"perf/av1/1080p_30fps_300frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name: "av1_1080p_60fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "perf/av1/1080p_60fps_600frames.av1.mp4",
				decoderType: playback.LibGAV1,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"perf/av1/1080p_60fps_600frames.av1.mp4"},
			Fixture:           "chromeVideoWithSWDecodingAndLibGAV1",
		}, {
			Name: "h264_1080p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "perf/h264/1080p_60fps_600frames.h264.mp4",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "video_decoder_legacy_supported", "proprietary_codecs"},
			ExtraData:         []string{"perf/h264/1080p_60fps_600frames.h264.mp4"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp8_1080p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "perf/vp8/1080p_60fps_600frames.vp8.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp8/1080p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp9_1080p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/1080p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp9/1080p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp9_2160p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "perf/vp9/2160p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
				browserType: browser.TypeAsh,
			},
			ExtraData:         []string{"perf/vp9/2160p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60, "video_decoder_legacy_supported"},
			Fixture:           "chromeAlternateVideoDecoder",
		}, {
			Name: "vp8_1080p_30fps_hw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/1080_vp8.webm",
				decoderType:      playback.Hardware,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/1080_vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8, "drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideo",
		}, {
			Name: "vp8_1080p_30fps_sw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/1080_vp8.webm",
				decoderType:      playback.Software,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/1080_vp8.webm"},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "vp9_1080p_30fps_hw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/1080.webm",
				decoderType:      playback.Hardware,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/1080.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9, "drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideo",
		}, {
			Name: "vp9_1080p_30fps_sw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/1080.webm",
				decoderType:      playback.Software,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/1080.webm"},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "h264_1080p_30fps_hw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/1080.mp4",
				decoderType:      playback.Hardware,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/1080.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideo",
		}, {
			Name: "h264_1080p_30fps_sw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/1080.mp4",
				decoderType:      playback.Software,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/1080.mp4"},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideoWithSWDecoding",
		}, {
			Name: "av1_1080p_30fps_hw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/av1_1080p_30fps.mp4",
				decoderType:      playback.Hardware,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/av1_1080p_30fps.mp4"},
			ExtraSoftwareDeps: []string{caps.HWDecodeAV1, "drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideo",
		}, {
			Name: "av1_1080p_30fps_sw_long",
			Val: playbackPerfParams{
				fileName:         "crosvideo/av1_1080p_30fps.mp4",
				decoderType:      playback.Software,
				browserType:      browser.TypeAsh,
				measureRoughness: true,
			},
			ExtraData:         []string{"crosvideo/av1_1080p_30fps.mp4"},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			// Test produces no Media DevTools roughness on MT8173, nor on chromeboxes
			// with external displays, see b/171913706.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("hana", "elm"), hwdep.InternalDisplay()),
			Fixture:           "chromeVideoWithSWDecoding",
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
	}
	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorAlwaysAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
	}
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	playback.RunTest(ctx, s, cs, cr, testOpt.fileName, testOpt.decoderType, testOpt.gridSize, testOpt.measureRoughness)
}
