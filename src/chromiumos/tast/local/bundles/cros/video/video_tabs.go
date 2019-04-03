// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
        "context"
        "time"
        "chromiumos/tast/local/bundles/cros/video/play"
        "chromiumos/tast/local/chrome"
        "chromiumos/tast/testing"
)

func init() {

        testing.AddTest(&testing.Test{
                Func:         VideoTabs,
                Desc:         "Checks video playback in three tab i.e local file-google drive video file, youtube and one from the server",
                Contacts:     []string{"keiichiw@chromium.org", "chromeos-video-eng@google.com"},
                Attr:         []string{"informational"},
                SoftwareDeps: []string{"chrome_login"},
                Pre:          chrome.LoggedIn(),
                Data:         []string{"bear-320x240.vp9.webm","bear.mjpeg.external", "video.html","youtube.html"},
        })
}

// Checks video playback in three tab i.e local file-google drive video file, youtube and one from the server
func VideoTabs(ctx context.Context, s *testing.State) {
        time.Sleep(20 * time.Second)
        play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
                "bear-320x240.vp9.webm", play.NormalVideo,play.NoCheckHistogram)
	play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),"", play.YoutubeVideo,play.NoCheckHistogram)
        play.TestPlay(ctx, s, s.PreValue().(*chrome.Chrome),
                "https://drive.google.com/uc?export=download&id=1Aho653IGvXZWkLmn0utDsYlxAYDVENM2", play.NormalVideo,play.NoCheckHistogram)
        }


