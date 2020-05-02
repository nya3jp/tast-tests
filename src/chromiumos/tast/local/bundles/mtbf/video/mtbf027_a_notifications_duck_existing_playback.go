// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/common"
	"chromiumos/tast/local/bundles/mtbf/video/youtube"
	"chromiumos/tast/local/chrome"
	ch "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF027ANotificationsDuckExistingPlayback,
		Desc:         "NotificationsDuckExistingPlayback(MTBF027): notifications should duck the existing playback",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"video.youtubeVideo", "video.shortDuckingAudioURL"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF027ANotificationsDuckExistingPlayback notifications should duck the existing playback
func MTBF027ANotificationsDuckExistingPlayback(ctx context.Context, s *testing.State) {
	videoURL := common.GetVar(ctx, s, "video.youtubeVideo")
	shortDuckingAudioURL := common.GetVar(ctx, s, "video.shortDuckingAudioURL")
	gmailURL := "https://mail.google.com/"
	cr := s.PreValue().(*chrome.Chrome)

	// Start playing audio/video from browser or default player Ex: YouTube video.
	s.Log("Open video URL for playing")
	videoConn, mtbferr := ch.NewConn(ctx, cr, youtube.Add1SecondForURL(videoURL))
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	testing.Sleep(ctx, 3*time.Second)

	// Play any short (<5 seconds) ducking audio
	s.Log("Open corrupted media file URL")
	conn, mtbferr := ch.NewConn(ctx, cr, shortDuckingAudioURL)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Wait document ready")
	if mtbferr = dom.WaitForDocumentReady(ctx, conn); mtbferr != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoDocLoad, nil, shortDuckingAudioURL))
	}

	s.Log("Try to play short ducking audio")
	if mtbferr := dom.PlayElement(ctx, conn, "body > audio"); mtbferr != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoPlayingEle, nil, shortDuckingAudioURL))
	}

	// Observe behavior.
	testing.Sleep(ctx, 5*time.Second)
	conn.CloseTarget(ctx)
	// Verify video continues playing
	if mtbferr = youtube.IsPlaying(ctx, videoConn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	// Let YouTube video play and open Gmail in another page
	gmailConn, mtbferr := ch.NewConn(ctx, cr, gmailURL)
	s.Log("Open Gmail page:", gmailConn)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
