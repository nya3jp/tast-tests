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
		Func:         PlayVDVP9,
		Desc:         "Checks whether VP9 video playback is working when using a media::VideoDecoder (see go/vd-migration)",
		Contacts:     []string{"dstaessens@chromium.org", "akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.ChromeVideoVD(),
		Data:         []string{"720_vp9.webm", "video.html"},
	})
}

func PlayVDVP9(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"720_vp9.webm", play.NormalVideo, play.NoCheckHistogram)
}
