// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package player

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/debug"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

// Player is a audio player object that can play/pause.
type Player struct {
	s        *testing.State
	filename string
}

// New return a new audio player.
func New(s *testing.State, filename string, fullFilename string) (player *Player, err error) {
	audioFileLocation := filepath.Join(filesapp.DownloadPath, filename)
	if err := fsutil.CopyFile(fullFilename, audioFileLocation); err != nil {
		return nil, mtbferrors.New(mtbferrors.VideoCopy, err, filename, audioFileLocation)
	}
	player = &Player{s: s, filename: filename}
	s.Logf("Your file(%s) should be ready at download folder", filename)
	return player, nil
}

// StartToPlay plays audio file by given name in download folder.
func (p *Player) StartToPlay(ctx context.Context, files *filesapp.FilesApp) (err error) {
	p.s.Log("Open the Downloads folder")
	if err := files.OpenDownloads(ctx); err != nil {
		return mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads")
	}

	p.s.Log("Wait for audio file shows on files app")
	if err := files.WaitForElement(ctx, filesapp.RoleStaticText, p.filename, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, p.filename)
	}
	// Make sure UI is stable before clicking
	testing.Sleep(ctx, 2*time.Second)
	if err := files.ClickElement(ctx, filesapp.RoleStaticText, p.filename); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, p.filename)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()
	testing.Sleep(ctx, 2*time.Second)

	p.s.Logf("Open Audio Player to play %s.", p.filename)
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
	}
	testing.Sleep(ctx, 1*time.Second)
	debug.TakeScreenshot(ctx)
	if err := files.WaitForElement(ctx, filesapp.RoleRootWebArea, "Audio Player", time.Minute); err != nil {
		return mtbferrors.New(mtbferrors.ChromeOpenAudioPlayer, err)
	}
	return nil
}
