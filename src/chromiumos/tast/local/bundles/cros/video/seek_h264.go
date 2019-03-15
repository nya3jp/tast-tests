// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/lib/pre"
	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     SeekH264,
		Desc:     "Verifies that H.264 non-resolution-changing seek works in Chrome",
		Contacts: []string{"acourbot@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"informational"},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome_login", "chrome_internal"},
		Pre:          pre.LoggedInVideo(),
		Data:         []string{"video_seek.mp4", "video.html"},
	})
}

// SeekH264 plays a non-resolution-changing H264 file with
// Chrome and checks that it can safely be seeked into.
func SeekH264(ctx context.Context, s *testing.State) {
	play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), "video_seek.mp4")
}
