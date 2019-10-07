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
		Func:     SeekVDH264,
		Desc:     "Verifies that H.264 non-resolution-changing seek works in Chrome when using a media::VideoDecoder",
		Contacts: []string{"acourbot@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          pre.ChromeVideoVD(),
		Data:         []string{"video_seek.mp4", "video.html"},
	})
}

// SeekVDH264 plays a non-resolution-changing H264 video in the Chrome browser
// and checks that it can safely be seeked into while using a media::VideoDecoder
// (see go/vd-migration).
func SeekVDH264(ctx context.Context, s *testing.State) {
	play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), "video_seek.mp4")
}
