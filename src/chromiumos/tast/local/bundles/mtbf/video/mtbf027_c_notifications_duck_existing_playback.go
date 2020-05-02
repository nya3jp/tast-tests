// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/video/common"
	"chromiumos/tast/local/bundles/mtbf/video/youtube"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF027CNotificationsDuckExistingPlayback,
		Desc:         "NotificationsDuckExistingPlayback(MTBF027): notifications should duck the existing playback",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"video.youtubeVideo"},
		Pre:          chrome.LoginReuse(),
	})
}

// MTBF027CNotificationsDuckExistingPlayback notifications should duck the existing playback
func MTBF027CNotificationsDuckExistingPlayback(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	videoURL := common.GetVar(ctx, s, "video.youtubeVideo")
	gmailURL := "https://mail.google.com/mail/u/0/#inbox"

	videoConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(youtube.Add1SecondForURL(videoURL)))
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExistTarget, err, videoURL))
	}
	defer videoConn.CloseTarget(ctx)

	gmailConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(gmailURL))
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeExistTarget, err, gmailURL))
	}
	defer gmailConn.CloseTarget(ctx)
}
