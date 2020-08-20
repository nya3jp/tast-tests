// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/debug"
	mtbfui "chromiumos/tast/local/mtbf/ui"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/testing"
)

const videoPlayerAppID = "jcgeabjmjgoblfofpppfkcoakmfobdko"

// Player is a video player object that can play/pause.
type Player struct {
	tconn *chrome.TestConn
	files *mtbfFilesapp.MTBFFilesApp
	kb    *input.KeyboardEventWriter
}

// NewPlayer return a new video player.
func NewPlayer(ctx context.Context, cr *chrome.Chrome) (player *Player, err error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		return nil, mtbferr
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.KbInit, err)
	}

	player = &Player{files: files, tconn: tconn, kb: kb}
	return player, nil
}

// StartToPlay plays video file by given name in download folder.
func (p *Player) StartToPlay(ctx context.Context, filename, fullFilename string) (err error) {
	videoFileLocation := filepath.Join(mtbfFilesapp.VideoFolderPath, filename)
	if _, err := os.Stat(videoFileLocation); os.IsNotExist(err) {
		os.Mkdir(mtbfFilesapp.VideoFolderPath, 0755)
	}

	if err := fsutil.CopyFile(fullFilename, videoFileLocation); err != nil {
		return mtbferrors.New(mtbferrors.VideoCopy, err, filename, videoFileLocation)
	}

	if err := p.files.OpenDownloads(ctx); err != nil {
		return mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads")
	}

	if mtbferr := p.files.EnterFolderPath(ctx, []string{"videos"}); mtbferr != nil {
		return mtbferr
	}

	// Make filesApp enter full screen mode (list more files)
	if err := p.kb.Accel(ctx, "F3"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "F3")
	}

	if err := p.files.WaitForFile(ctx, filename, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, filename)
	}
	// Make sure UI is stable before clicking
	testing.Sleep(ctx, 2*time.Second)
	if err := p.files.SelectFile(ctx, filename); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, filename)
	}

	testing.Sleep(ctx, 2*time.Second)

	testing.ContextLogf(ctx, "Open Video Player to play %s", filename)
	if err := p.kb.Accel(ctx, "Enter"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
	}
	testing.Sleep(ctx, 1*time.Second)

	if err := mtbfui.WaitForElement(ctx, p.tconn, ui.RoleTypeWindow, filename, time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.ChromeOpenVideoPlayer, err)
	}

	if err := p.kb.Accel(ctx, "Alt+="); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Alt+=")
	}
	return nil
}

// Play plays video file
func (p *Player) Play(ctx context.Context) error {
	if err := mtbfui.WaitForElement(ctx, p.tconn, ui.RoleTypeButton, "play", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.VideoWaitPlayButton, err)
	}

	if err := mtbfui.ClickElement(ctx, p.tconn, ui.RoleTypeButton, "play"); err != nil {
		return mtbferrors.New(mtbferrors.VideoClickPlayButton, err)
	}
	return nil
}

// Pause pauses playing video file
func (p *Player) Pause(ctx context.Context) error {
	if err := mtbfui.WaitForElement(ctx, p.tconn, ui.RoleTypeButton, "pause", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.VideoWaitPauseButton, err)
	}

	if err := mtbfui.ClickElement(ctx, p.tconn, ui.RoleTypeButton, "pause"); err != nil {
		return mtbferrors.New(mtbferrors.VideoClickPauseButton, err)
	}
	return nil
}

// Close closes video player
func (p *Player) Close(ctx context.Context) error {
	defer p.tconn.Close()
	defer p.tconn.CloseTarget(ctx)
	defer p.files.Close(ctx)

	// Minimize video player
	if err := p.kb.Accel(ctx, "Alt+="); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Alt+=")
	}

	if err := apps.Close(ctx, p.tconn, videoPlayerAppID); err != nil {
		return mtbferrors.New(mtbferrors.ChromeCloseApp, err, "Video Player")
	}

	// Make filesApp escape full screen mode
	if err := p.kb.Accel(ctx, "F3"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Alt+=")
	}

	return nil
}

func parseDuration(t string) (float64, error) {
	f := ""
	ts := strings.Split(t, ":")
	switch len(ts) {
	case 1:
		f = fmt.Sprintf("%ss", ts[0])
	case 2:
		f = fmt.Sprintf("%sm%ss", ts[0], ts[1])
	case 3:
		f = fmt.Sprintf("%sh%sm%ss", ts[0], ts[1], ts[2])
	}

	d, err := time.ParseDuration(f)
	if err != nil {
		return -1, err
	}

	return d.Seconds(), nil
}

// GetPlayingTime returns audio player playing time.
func (p *Player) GetPlayingTime(ctx context.Context) (playtime string, err error) {
	const javascript = `new Promise((resolve, reject) => {
		let playTime;
		const recursive = root => {
			if (root.children && root.children.length > 0) {
				root.children.forEach(child => recursive(child));
			} else if (root.role === 'inlineTextBox' && /\d*:\d*/.test(root.name)) {
				playTime = root.name;
			}
		};
		chrome.automation.getDesktop(root => recursive(root));
		if (playTime) {
			resolve(playTime);
		} else {
			reject("none time has been found.");
		}
	})`

	if err = p.tconn.EvalPromise(ctx, javascript, &playtime); err != nil {
		return
	}
	return
}

// IsPlaying verify video is still playing.
func (p *Player) IsPlaying(ctx context.Context, timeout time.Duration) (err error) {
	var currentTimeStr, previousTimeStr string
	if previousTimeStr, err = p.GetPlayingTime(ctx); err != nil {
		return
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}
	if currentTimeStr, err = p.GetPlayingTime(ctx); err != nil {
		return
	}

	currentTime, err := parseDuration(currentTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, currentTimeStr)
	}

	previousTime, err := parseDuration(previousTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, previousTimeStr)
	}

	if previousTime >= currentTime {
		return mtbferrors.New(mtbferrors.VideoPlay, nil, currentTime, previousTime, timeout.Seconds())
	}

	return nil
}

// IsPausing verify audio is now pausing.
func (p *Player) IsPausing(ctx context.Context, timeout time.Duration) (err error) {
	var currentTimeStr, previousTimeStr string
	if previousTimeStr, err = p.GetPlayingTime(ctx); err != nil {
		return
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}
	if currentTimeStr, err = p.GetPlayingTime(ctx); err != nil {
		return
	}

	currentTime, err := parseDuration(currentTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, currentTimeStr)
	}

	previousTime, err := parseDuration(previousTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, previousTimeStr)
	}

	if currentTime > previousTime {
		return mtbferrors.New(mtbferrors.VideoPause, nil, currentTime, previousTime, timeout.Seconds())
	}

	return nil
}
