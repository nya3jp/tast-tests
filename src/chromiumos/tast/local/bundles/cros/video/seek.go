// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

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
		Pre:          pre.ChromeVideo(),
		Data:         []string{"video.html"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:      "h264",
			Val:       "video_seek.mp4",
			ExtraData: []string{"video_seek.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name:      "vp8",
			Val:       "video_seek.webm",
			ExtraData: []string{"video_seek.webm"},
		}, {
			Name:      "vp9",
			Val:       "shaka_720.webm",
			ExtraData: []string{"shaka_720.webm"},
		}, {
			Name:      "switch_h264",
			Val:       "switch_1080p_720p.mp4",
			ExtraData: []string{"switch_1080p_720p.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name:      "switch_vp8",
			Val:       "frame_size_change.webm",
			ExtraData: []string{"frame_size_change.webm"},
		}},
	})
}

// Seek plays a file with Chrome and checks that it can safely be seeked into.
func Seek(ctx context.Context, s *testing.State) {
	play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), s.Param().(string))
}
