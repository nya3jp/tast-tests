// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SeekVP8,
		Desc:         "Verifies that vp8 non-resolution-changing seek works in Chrome",
		Contacts:     []string{"acourbot@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Pre:          chrome.LoggedInVideo(),
		Data:         []string{"video_seek.webm", "video.html"},
	})
}

// SeekVP8 plays a non-resolution-changing VP8 file with
// Chrome and checks that it can safely be seeked into.
func SeekVP8(ctx context.Context, s *testing.State) {
	play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), "video_seek.webm")
}
