// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/common"
	"chromiumos/tast/local/bundles/mtbf/video/youtube"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF027BNotificationsDuckExistingPlayback,
		Desc:         "NotificationsDuckExistingPlayback(MTBF027): notifications should duck the existing playback",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"video.youtubeVideo"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF027BNotificationsDuckExistingPlayback notifications should duck the existing playback
func MTBF027BNotificationsDuckExistingPlayback(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := common.GetVar(ctx, s, "video.youtubeVideo")
	gmailURL := "https://mail.google.com/mail/u/0/#inbox"

	videoConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(youtube.Add1SecondForURL(videoURL)))
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExistTarget, err, videoURL))
	}

	gmailConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(gmailURL))
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExistTarget, err, gmailURL))
	}

	// Verify message received
	if !isChatRoomExists(ctx, gmailConn) {
		s.Fatal(mtbferrors.New(mtbferrors.AudioNoMsg, nil))
	}
	// Verify video continues playing
	if mtbferr := youtube.IsPlaying(ctx, videoConn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}

func isChatRoomExists(ctx context.Context, conn *chrome.Conn) bool {
	var chatRoomCount int
	script := "document.querySelector('%s').childElementCount"
	chatRoomParent := "body > div.dw > div > div > div > div.no > div:nth-child(3)"
	query := fmt.Sprintf(script, chatRoomParent)
	conn.Eval(ctx, query, &chatRoomCount)
	return chatRoomCount != 0
}
