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
		Func:         SeekVP9,
		Desc:         "Verifies that VP9 non-resolution-changing seek works in Chrome",
		Contacts:     []string{"acourbot@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
		Pre:          chrome.LoggedInVideo(),
		Data:         []string{"shaka_720.webm", "video.html"},
	})
}

// TODO this test should not be performed on Nyan due to crbug.com/699260.

// SeekVP9 plays a non-resolution-changing VP9 file with
// Chrome and checks that it can safely be seeked into.
func SeekVP9(ctx context.Context, s *testing.State) {
	play.TestSeek(ctx, s, s.PreValue().(*chrome.Chrome), "shaka_720.webm")
}
