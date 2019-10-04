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
		Func:     PlayVDVP8,
		Desc:     "Checks whether VP8 video playback is working when using a media::VideoDecoder (see go/vd-migration)",
		Contacts: []string{"dstaessens@chromium.org", "akahuang@chromium.org", "chromeos-video-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO(b/137916185): Remove dependency on android capability. It's used here
		// to guarantee import-mode support, which is required by the new VD's.
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          pre.ChromeVideoVD(),
		Data:         []string{"720_vp8.webm", "video.html"},
	})
}

func PlayVDVP8(ctx context.Context, s *testing.State) {
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"720_vp8.webm", play.NormalVideo, play.NoVerifyHWAcceleratorUsed)
}
