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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/debug"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

const videoPlayerAppID = "jcgeabjmjgoblfofpppfkcoakmfobdko"

// VideoPlayer is a video player object that can play/pause.
type VideoPlayer struct {
	tconn *chrome.Conn
	files *filesapp.FilesApp
}

// NewVideoPlayer return a new video player.
func NewVideoPlayer(ctx context.Context, cr *chrome.Chrome) (player *VideoPlayer, err error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		return nil, mtbferr
	}

	player = &VideoPlayer{files: files, tconn: tconn}
	return player, nil
}

// StartToPlay plays video file by given name in download folder.
func (p *VideoPlayer) StartToPlay(ctx context.Context, filename string, fullFilename string) (err error) {
	videoFileLocation := filepath.Join(filesapp.DownloadPath, filename)
	if err := fsutil.CopyFile(fullFilename, videoFileLocation); err != nil {
		return mtbferrors.New(mtbferrors.VideoCopy, err, filename, videoFileLocation)
	}

	if err := p.files.OpenDownloads(ctx); err != nil {
		return mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads")
	}

	if err := p.files.WaitForElement(ctx, filesapp.RoleStaticText, filename, time.Second*10); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, filename)
	}
	// Make sure UI is stable before clicking
	testing.Sleep(ctx, 2*time.Second)
	if err := p.files.ClickElement(ctx, filesapp.RoleStaticText, filename); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, filename)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()
	testing.Sleep(ctx, 2*time.Second)

	if err := kb.Accel(ctx, "Enter"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
	}
	testing.Sleep(ctx, 1*time.Second)

	if err := p.files.WaitForElement(ctx, filesapp.RoleRootWebArea, filename, time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.ChromeOpenVideoPlayer, err)
	}
	return nil
}

// Play plays video file
func (p *VideoPlayer) Play(ctx context.Context) error {
	if err := p.files.WaitForElement(ctx, "button", "play", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.VideoWaitPlayButton, err)
	}

	if err := p.files.ClickElement(ctx, "button", "play"); err != nil {
		return mtbferrors.New(mtbferrors.VideoClickPlayButton, err)
	}
	return nil
}

// Pause pauses playing video file
func (p *VideoPlayer) Pause(ctx context.Context) error {
	if err := p.files.WaitForElement(ctx, "button", "pause", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.VideoWaitPauseButton, err)
	}

	if err := p.files.ClickElement(ctx, "button", "pause"); err != nil {
		return mtbferrors.New(mtbferrors.VideoClickPauseButton, err)
	}
	return nil
}

// Close closes video player
func (p *VideoPlayer) Close(ctx context.Context) error {
	defer p.tconn.Close()
	defer filesapp.Close(ctx, p.tconn)
	if err := apps.Close(ctx, p.tconn, videoPlayerAppID); err != nil {
		return mtbferrors.New(mtbferrors.ChromeCloseApp, err, "Video Player")
	}
	return nil
}
