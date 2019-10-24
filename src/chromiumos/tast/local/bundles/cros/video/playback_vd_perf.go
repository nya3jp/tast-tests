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
		Func:         PlaybackVDPerf,
		Desc:         "Measures video playback performance with/without HW acceleration using a media::VideoDecoder",
		Contacts:     []string{"akahuang@chromium.org", "dstaessens@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"cros_video_decoder", "chrome"},
		Data:         []string{decode.ChromeMediaInternalsUtilsJSFile},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Name: "h264_1080p_60fps",
			Val:  "1080p_60fps_600frames.h264.mp4",
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
			ExtraData:         []string{"1080p_60fps_600frames.h264.mp4"},
		}, {
			Name:      "vp8_1080p_60fps",
			Val:       "1080p_60fps_600frames.vp8.webm",
			ExtraData: []string{"1080p_60fps_600frames.vp8.webm"},
		}, {
			Name:      "vp9_1080p_60fps",
			Val:       "1080p_60fps_600frames.vp9.webm",
			ExtraData: []string{"1080p_60fps_600frames.vp9.webm"},
		}, {
			Name:      "vp9_2160p_60fps",
			Val:       "2160p_60fps_600frames.vp9.webm",
			ExtraData: []string{"2160p_60fps_600frames.vp9.webm"},
		}},
	})
}

// PlaybackVDPerf plays a video in the Chrome browser and collects various performance metrics with
// HW video decode acceleration disabled/enabled if available, while using a media::VideoDecoder
// (see go/vd-migration). The values are reported to the performance dashboard.
func PlaybackVDPerf(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, s.Param().(string), playback.DefaultPerfDisabled, playback.VD)
}
