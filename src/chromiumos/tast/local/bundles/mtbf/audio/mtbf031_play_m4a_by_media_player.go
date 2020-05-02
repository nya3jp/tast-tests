// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/audio"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF031PlayM4aByMediaPlayer,
		Desc:         "Play a m4a file by native media player",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoginReuse(),
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Data:         []string{"audio.m4a"},
	})
}

func MTBF031PlayM4aByMediaPlayer(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	s.Log("Preparing m4a to play")
	audioFile := "audio.m4a"
	audioFileLocation := filepath.Join(filesapp.DownloadPath, audioFile)
	if err := fsutil.CopyFile(s.DataPath(audioFile), audioFileLocation); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoCopy, err, audioFile, audioFileLocation))
	}

	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer tconn.Close()
	defer tconn.CloseTarget(ctx)

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeGetKeyboard, err))
	}
	defer kb.Close()

	s.Log("Open the Files App")
	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		s.Fatal(mtbferr)
	}
	defer filesapp.Close(ctx, tconn)

	s.Log("Open the Downloads folder")
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads"))
	}

	s.Log("Wait for audio file shows on files app")
	// To make sure it's visible in case of item not found,
	// so we have to sort files by modified date in descending order.
	if mtbferr := mtbfFilesapp.SortFilesByModifiedDateInDescendingOrder(ctx, files); mtbferr != nil {
		s.Fatal(mtbferr)
	}
	if mtbferr := mtbfFilesapp.WaitAndClickElement(ctx, files, filesapp.RoleStaticText, audioFile); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	testing.Sleep(ctx, 2*time.Second)
	s.Log("Open Audio Player to play m4a")
	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter"))
	}
	if err := files.WaitForElement(ctx, filesapp.RoleRootWebArea, "Audio Player", time.Minute); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeOpenAudioPlayer, err))
	}
	s.Log("Verify audio player is playing")
	if mtbferr := audio.IsPlaying(ctx, tconn, 5*time.Second); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
