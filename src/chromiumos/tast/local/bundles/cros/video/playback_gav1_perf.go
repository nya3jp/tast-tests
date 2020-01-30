// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func:         PlaybackGav1Perf,
		Desc:         "Measures video playback performance in Chrome browser with/without HW acceleration",
		Contacts:     []string{"hiroh@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arm"},
		Data:         []string{decode.ChromeMediaInternalsUtilsJSFile},
		// Default timeout (i.e. 2 minutes) is not enough for low-end devices.
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
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

// PlaybackGav1Perf plays a video in the Chrome browser and measures the performance with VideoDecoder using libgav1, called Gav1VideoDecoder.
// The values are reported to the performance dashboard.
func PlaybackGav1Perf(ctx context.Context, s *testing.State) {
	playback.RunTest(ctx, s, s.Param().(string), playback.DefaultPerfDisabled, playback.GAV1)
}
