// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/bundles/cros/video/playback"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackPerf,
		Desc:         "Measures video playback performance in Chrome browser with/without HW acceleration",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{decode.ChromeMediaInternalsUtilsJSFile},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
		// "chrome_internal" is needed for H.264 videos because H.264 is a proprietary codec.
		Params: []testing.Param{{
			Name:              "h264_1080p_30fps",
			Val:               "1080p_30fps_300frames.h264.mp4",
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"1080p_30fps_300frames.h264.mp4"},
		}, {
			Name:              "h264_1080p_60fps",
			Val:               "1080p_60fps_600frames.h264.mp4",
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
		}, {
			Name:              "h264_2160p_30fps",
			Val:               "2160p_30fps_300frames.h264.mp4",
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"2160p_30fps_300frames.h264.mp4"},
		}, {
			Name:              "h264_2160p_60fps",
			Val:               "2160p_60fps_600frames.h264.mp4",
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"2160p_60fps_600frames.h264.mp4"},
		}, {
			Name:      "vp8_1080p_30fps",
			Val:       "1080p_30fps_300frames.vp8.webm",
			ExtraData: []string{"1080p_30fps_300frames.vp8.webm"},
		}, {
			Name:      "vp8_1080p_60fps",
			Val:       "1080p_60fps_600frames.vp8.webm",
			ExtraData: []string{"1080p_60fps_600frames.vp8.webm"},
		}, {
			Name:      "vp8_2160p_30fps",
			Val:       "2160p_30fps_300frames.vp8.webm",
			ExtraData: []string{"2160p_30fps_300frames.vp8.webm"},
		}, {
			Name:      "vp8_2160p_60fps",
			Val:       "2160p_60fps_600frames.vp8.webm",
			ExtraData: []string{"2160p_60fps_600frames.vp8.webm"},
		}, {
			Name:      "vp9_1080p_30fps",
			Val:       "1080p_30fps_300frames.vp9.webm",
			ExtraData: []string{"1080p_30fps_300frames.vp9.webm"},
		}, {
			Name:      "vp9_1080p_60fps",
			Val:       "1080p_60fps_600frames.vp9.webm",
			ExtraData: []string{"1080p_60fps_600frames.vp9.webm"},
		}, {
			Name:      "vp9_2160p_30fps",
			Val:       "2160p_30fps_300frames.vp9.webm",
			ExtraData: []string{"2160p_30fps_300frames.vp9.webm"},
		}, {
			Name:      "vp9_2160p_60fps",
			Val:       "2160p_60fps_600frames.vp9.webm",
			ExtraData: []string{"2160p_60fps_600frames.vp9.webm"},
		}, {
			Name:      "av1_480p_30fps",
			Val:       "480p_30fps_300frames.av1.mp4",
			ExtraData: []string{"480p_30fps_300frames.av1.mp4"},
		}, {
			Name:      "av1_720p_30fps",
			Val:       "720p_30fps_300frames.av1.mp4",
			ExtraData: []string{"720p_30fps_300frames.av1.mp4"},
		}, {
			Name:      "av1_720p_60fps",
			Val:       "720p_60fps_600frames.av1.mp4",
			ExtraData: []string{"720p_60fps_600frames.av1.mp4"},
		}, {
			Name:      "av1_1080p_30fps",
			Val:       "1080p_30fps_300frames.av1.mp4",
			ExtraData: []string{"1080p_30fps_300frames.av1.mp4"},
		}, {
			Name:      "av1_1080p_60fps",
			Val:       "1080p_60fps_600frames.av1.mp4",
			ExtraData: []string{"1080p_60fps_600frames.av1.mp4"},
		}},
	})
}

// PlaybackPerf plays a video in the Chrome browser and measures the performance with and without
// HW decode acceleration if available. The values are reported to the performance dashboard.
func PlaybackPerf(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, s.Param().(string), playback.DefaultPerfDisabled, playback.VDA)
}
