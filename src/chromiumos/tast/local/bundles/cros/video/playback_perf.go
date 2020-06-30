// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/playback"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

type playbackPerfParams struct {
	fileName    string
	decoderType playback.DecoderType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackPerf,
		Desc:         "Measures video playback performance in Chrome browser with/without HW acceleration",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{decode.ChromeMediaInternalsUtilsJSFile},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
		// "chrome_internal" is needed for H.264 videos because H.264 is a proprietary codec.
		Params: []testing.Param{{
			Name: "h264_144p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "144p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"144p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_240p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "240p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"240p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_360p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "360p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"360p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "chrome_internal"},
			ExtraData:         []string{"1080p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "chrome_internal"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "2160p_30fps_300frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K, "chrome_internal"},
			ExtraData:         []string{"2160p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "2160p_60fps_600frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_4K60, "chrome_internal"},
			ExtraData:         []string{"2160p_60fps_600frames.h264.mp4"},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_144p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "144p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"144p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_240p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "240p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"240p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_360p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "360p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"360p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"1080p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "2160p_30fps_300frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"2160p_30fps_300frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp8_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "2160p_60fps_600frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"2160p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_4K60},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_144p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "144p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"144p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_240p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "240p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"240p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_360p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "360p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"360p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_480p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_720p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_1080p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"1080p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_1080p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_2160p_30fps_hw",
			Val: playbackPerfParams{
				fileName:    "2160p_30fps_300frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"2160p_30fps_300frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "vp9_2160p_60fps_hw",
			Val: playbackPerfParams{
				fileName:    "2160p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60},
			Pre:               pre.ChromeVideo(),
		}, {
			Name: "h264_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.h264.mp4",
				decoderType: playback.Software,
			},
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"480p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "h264_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.h264.mp4",
				decoderType: playback.Software,
			},
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraAttr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData:         []string{"720p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "h264_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.h264.mp4",
				decoderType: playback.Software,
			},
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"1080p_30fps_300frames.h264.mp4"},
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "h264_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.h264.mp4",
				decoderType: playback.Software,
			},
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
			Pre:               pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp8_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.vp8.webm",
				decoderType: playback.Software,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"480p_30fps_300frames.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp8_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.vp8.webm",
				decoderType: playback.Software,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"720p_30fps_300frames.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp8_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.vp8.webm",
				decoderType: playback.Software,
			},
			ExtraData: []string{"1080p_30fps_300frames.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp8_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.vp8.webm",
				decoderType: playback.Software,
			},
			ExtraData: []string{"1080p_60fps_600frames.vp8.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp9_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.vp9.webm",
				decoderType: playback.Software,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"480p_30fps_300frames.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp9_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.vp9.webm",
				decoderType: playback.Software,
			},
			ExtraAttr: []string{"group:graphics", "graphics_video", "graphics_nightly"},
			ExtraData: []string{"720p_30fps_300frames.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp9_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.vp9.webm",
				decoderType: playback.Software,
			},
			ExtraData: []string{"1080p_30fps_300frames.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "vp9_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.vp9.webm",
				decoderType: playback.Software,
			},
			ExtraData: []string{"1080p_60fps_600frames.vp9.webm"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "av1_480p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.av1.mp4",
				decoderType: playback.Software,
			},
			ExtraData: []string{"480p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "av1_720p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.av1.mp4",
				decoderType: playback.Software,
			},
			ExtraData: []string{"720p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "av1_720p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "720p_60fps_600frames.av1.mp4",
				decoderType: playback.Software,
			},
			ExtraData: []string{"720p_60fps_600frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "av1_1080p_30fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.av1.mp4",
				decoderType: playback.Software,
			},
			ExtraData: []string{"1080p_30fps_300frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "av1_1080p_60fps_sw",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.av1.mp4",
				decoderType: playback.Software,
			},
			ExtraData: []string{"1080p_60fps_600frames.av1.mp4"},
			Pre:       pre.ChromeVideoWithSWDecoding(),
		}, {
			Name: "av1_480p_30fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "480p_30fps_300frames.av1.mp4",
				decoderType: playback.LibGAV1,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"480p_30fps_300frames.av1.mp4"},
			Pre:               pre.ChromeVideoWithSWDecodingAndLibGAV1(),
		}, {
			Name: "av1_720p_30fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "720p_30fps_300frames.av1.mp4",
				decoderType: playback.LibGAV1,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"720p_30fps_300frames.av1.mp4"},
			Pre:               pre.ChromeVideoWithSWDecodingAndLibGAV1(),
		}, {
			Name: "av1_720p_60fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "720p_60fps_600frames.av1.mp4",
				decoderType: playback.LibGAV1,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"720p_60fps_600frames.av1.mp4"},
			Pre:               pre.ChromeVideoWithSWDecodingAndLibGAV1(),
		}, {
			Name: "av1_1080p_30fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "1080p_30fps_300frames.av1.mp4",
				decoderType: playback.LibGAV1,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"1080p_30fps_300frames.av1.mp4"},
			Pre:               pre.ChromeVideoWithSWDecodingAndLibGAV1(),
		}, {
			Name: "av1_1080p_60fps_sw_gav1",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.av1.mp4",
				decoderType: playback.LibGAV1,
			},
			ExtraSoftwareDeps: []string{"arm"},
			ExtraData:         []string{"1080p_60fps_600frames.av1.mp4"},
			Pre:               pre.ChromeVideoWithSWDecodingAndLibGAV1(),
		}, {
			Name: "h264_1080p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.h264.mp4",
				decoderType: playback.Hardware,
			},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{caps.HWDecodeH264_60, "chrome_internal"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name: "vp8_1080p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.vp8.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"1080p_60fps_600frames.vp8.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP8_60},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name: "vp9_1080p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "1080p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"1080p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_60},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}, {
			Name: "vp9_2160p_60fps_hw_alt",
			Val: playbackPerfParams{
				fileName:    "2160p_60fps_600frames.vp9.webm",
				decoderType: playback.Hardware,
			},
			ExtraData:         []string{"2160p_60fps_600frames.vp9.webm"},
			ExtraSoftwareDeps: []string{caps.HWDecodeVP9_4K60},
			Pre:               pre.ChromeAlternateVideoDecoder(),
		}},
	})
}

// PlaybackPerf plays a video in the Chrome browser and measures the performance with or without
// HW decode acceleration as per DecoderType. The values are reported to the performance dashboard.
func PlaybackPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(playbackPerfParams)
	playback.RunTest(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.fileName, testOpt.decoderType)
}
