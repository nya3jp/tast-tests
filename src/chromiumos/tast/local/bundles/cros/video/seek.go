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

// seekTest is used to describe the config used to run each Seek test.
type seekTest struct {
	filename string // File name to play back.
	numSeeks int    // Amount of times to seek into the <video>.
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
		Pre:          pre.ChromeVideo(),
		Data:         []string{"video.html"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:      "h264",
			Val:       seekTest{filename: "720_h264.mp4", numSeeks: 25},
			ExtraData: []string{"720_h264.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name:      "vp8",
			Val:       seekTest{filename: "720_vp8.webm", numSeeks: 25},
			ExtraData: []string{"720_vp8.webm"},
		}, {
			Name:      "vp9",
			Val:       seekTest{filename: "720_vp9.webm", numSeeks: 25},
			ExtraData: []string{"720_vp9.webm"},
		}, {
			Name:      "switch_h264",
			Val:       seekTest{filename: "switch_1080p_720p.mp4", numSeeks: 25},
			ExtraData: []string{"switch_1080p_720p.mp4"},
			// "chrome_internal" is needed because H.264 is a proprietary codec.
			ExtraSoftwareDeps: []string{"chrome_internal"},
		}, {
			Name:      "switch_vp8",
			Val:       seekTest{filename: "frame_size_change.webm", numSeeks: 25},
			ExtraData: []string{"frame_size_change.webm"},
		}},
	})
}

// Seek plays a file with Chrome and checks that it can safely be seeked into.
func Seek(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(seekTest)
	play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), testOpt.filename, testOpt.numSeeks)
}
