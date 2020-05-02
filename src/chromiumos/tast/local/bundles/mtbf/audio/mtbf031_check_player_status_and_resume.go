// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF031CheckPlayerStatusAndResume,
		Desc:         "Check if player is paused and resume",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"audio.m4a"},
	})
}

func MTBF031CheckPlayerStatusAndResume(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()
	defer tconn.CloseTarget(ctx)
	defer audio.Close(ctx, tconn)

	s.Log("Open the Files App")
	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn) // Launch files app to use its API.
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer filesapp.Close(ctx, tconn)

	s.Log("Switch to audio player")
	if err := files.ClickElement(ctx, filesapp.RoleButton, "Audio Player"); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeClickItem, err, "Audio Player"))
	}
	testing.Sleep(ctx, 2*time.Second) // For possible UI update delay.

	s.Log("Verify audio player is paused")
	if mtbferr := audio.IsPausing(ctx, tconn, 5*time.Second); err != nil {
		s.Fatal(mtbferr)
	}

	s.Log("Resume what's audio player is playing")
	if err := audio.Play(ctx, tconn); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioPlaying, err))
	}
	testing.Sleep(ctx, time.Second)
	s.Log("Verify audio player is playing")
	if mtbferr := audio.IsPlaying(ctx, tconn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
