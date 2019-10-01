// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/local/bundles/cros/video/play"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PlayH264,
		Desc: "Checks H264 video playback is working",
		Contacts: []string{
			"acourbot@chromium.org",
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
			"chromeos-video-eng@google.com",
		},
		// "chrome_internal" is needed because H.264 is a proprietary codec.
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Pre:          pre.ChromeVideo(),
		Data:         []string{"720_h264.mp4", "video.html", "chrome_media_internals_utils.js"},
		Attr:         []string{"group:mainline"},
	})
}

// PlayH264 plays 720_h264.mp4 with Chrome.
func PlayH264(ctx context.Context, s *testing.State) {
	extraChromeMediaInternalsUtilsJS, err :=
		ioutil.ReadFile(s.DataPath("chrome_media_internals_utils.js"))
	if err != nil {
		s.Fatal("Failed to read chrome://media-internals JS: ", err)
	}

	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
		"720_h264.mp4", play.NormalVideo, string(extraChromeMediaInternalsUtilsJS))
}
