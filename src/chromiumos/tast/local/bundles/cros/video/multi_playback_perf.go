// Copyright 2022 The ChromiumOS Authors.
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

type multiPlaybackPerfParam struct {
	fileName   string
	gridWidth  int
	gridHeight int
	numThreads int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiPlaybackPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures multiple videos playback performance in Chrome browser with/without HW acceleration",
		Contacts: []string{
			"mcasas@chromium.org",
			"hiroh@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_video", "graphics_nightly"},
		SoftwareDeps: []string{"chrome", "lacros_stable"},
		Data:         []string{"video.html", "playback.js"},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_720px1_1threads",
			Val: multiPlaybackPerfParam{
				fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
				gridWidth:  1,
				gridHeight: 1,
				numThreads: 1,
			},
			ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
			ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
			Fixture:           "chromeVideoWithDecoderThreads1",
		},
			{
				Name: "h264_720px2_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  2,
					gridHeight: 1,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "h264_720px8_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  4,
					gridHeight: 2,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "h264_720px8_4threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  4,
					gridHeight: 2,
					numThreads: 4,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads4",
			},
			{
				Name: "h264_720px8_8threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  4,
					gridHeight: 2,
					numThreads: 8,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads8",
			},
			{
				Name: "h264_720px16_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "h264_720px16_4threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 4,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads4",
			},
			{
				Name: "h264_720px16_8threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 8,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads8",
			},
			{
				Name: "h264_720px16_16threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 16,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads16",
			},
			{
				Name: "h264_720px49_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "h264_720px49_4threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 4,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads4",
			},
			{
				Name: "h264_720px49_8threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 8,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads8",
			},
			{
				Name: "h264_720px49_16threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 16,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads16",
			},
			{
				Name: "h264_720px49_49threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/h264/720p_30fps_300frames.h264.mp4",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 49,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeH264, "proprietary_codecs"},
				ExtraData:         []string{"perf/h264/720p_30fps_300frames.h264.mp4"},
				Fixture:           "chromeVideoWithDecoderThreads49",
			},
			{
				Name: "vp9_720px1_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  1,
					gridHeight: 1,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "vp9_720px2_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  2,
					gridHeight: 1,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "vp9_720px8_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  4,
					gridHeight: 2,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "vp9_720px8_4threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  4,
					gridHeight: 2,
					numThreads: 4,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads4",
			},
			{
				Name: "vp9_720px8_8threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  4,
					gridHeight: 2,
					numThreads: 8,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads8",
			},
			{
				Name: "vp9_720px16_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "vp9_720px16_4threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 4,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads4",
			},
			{
				Name: "vp9_720px16_8threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 8,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads8",
			},
			{
				Name: "vp9_720px16_16threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  4,
					gridHeight: 4,
					numThreads: 16,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads16",
			},
			{
				Name: "vp9_720px49_1threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 1,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads1",
			},
			{
				Name: "vp9_720px49_4threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 4,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads4",
			},
			{
				Name: "vp9_720px49_8threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 8,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads8",
			},
			{
				Name: "vp9_720px49_16threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 16,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads16",
			},
			{
				Name: "vp9_720px49_49threads",
				Val: multiPlaybackPerfParam{
					fileName:   "perf/vp9/720p_30fps_300frames.vp9.webm",
					gridWidth:  7,
					gridHeight: 7,
					numThreads: 49,
				},
				ExtraSoftwareDeps: []string{caps.HWDecodeVP9},
				ExtraData:         []string{"perf/vp9/720p_30fps_300frames.vp9.webm"},
				Fixture:           "chromeVideoWithDecoderThreads49",
			},
		},
	})
}

func MultiPlaybackPerf(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(multiPlaybackPerfParam)
	_, l, cs, err := lacros.Setup(ctx, s.FixtValue(), browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(ctx, l)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	playback.RunTest(ctx, s, cs, cr, testOpt.fileName, playback.Hardware, testOpt.gridWidth, testOpt.gridHeight, false)
}
