// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// AudioPlayerName indicates the name of automation node that audio player is using.
const AudioPlayerName = "Audio Player"
const audioPlayerAppID = "cjbfomnbifhcdnihkgipgfcihmgjfhbf"

// Player repersent audio player data model.
type Player struct {
	tconn *chrome.TestConn
	files *mtbfFilesapp.MTBFFilesApp
}

// NewPlayer returns a new audio player by open file apps.
func NewPlayer(ctx context.Context, cr *chrome.Chrome) (*Player, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	files, mtbferr := mtbfFilesapp.Launch(ctx, tconn)
	if mtbferr != nil {
		return nil, mtbferr
	}

	player := &Player{files: files, tconn: tconn}
	return player, nil
}

// startToPlayBySorting by copying audio file to Downloads folder and click enter on it.
// if sorting is enable then will do an sorting before open audio file.
func (p *Player) startToPlayBySorting(ctx context.Context, filename, fullFilename string, isSorting bool) error {
	audioFileLocation := filepath.Join(mtbfFilesapp.AudioFolderPath, filename)

	if _, err := os.Stat(audioFileLocation); os.IsNotExist(err) {
		os.Mkdir(mtbfFilesapp.AudioFolderPath, 0755)
	}

	if err := fsutil.CopyFile(fullFilename, audioFileLocation); err != nil {
		return mtbferrors.New(mtbferrors.VideoCopy, err, filename, audioFileLocation)
	}

	if err := p.files.OpenDownloads(ctx); err != nil {
		return mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads")
	}

	if mtbferr := p.files.EnterFolderPath(ctx, []string{"audios"}); mtbferr != nil {
		return mtbferr
	}

	if isSorting {
		if mtbferr := p.files.SortFilesByModifiedDateInDescendingOrder(ctx); mtbferr != nil {
			return mtbferr
		}
	}

	if err := p.files.WaitForFile(ctx, filename, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, filename)
	}
	// Make sure UI is stable before clicking
	testing.Sleep(ctx, 2*time.Second)
	if err := p.files.SelectFile(ctx, filename); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, filename)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()
	testing.Sleep(ctx, 2*time.Second)

	testing.ContextLogf(ctx, "Open Audio Player to play %s", filename)
	if err := kb.Accel(ctx, "Enter"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeKeyPress, err, "Enter")
	}
	testing.Sleep(ctx, 1*time.Second)
	debug.TakeScreenshot(ctx)
	if err := mtbfui.WaitForElement(ctx, p.tconn, ui.RoleTypeWindow, AudioPlayerName, time.Minute); err != nil {
		return mtbferrors.New(mtbferrors.ChromeOpenAudioPlayer, err)
	}

	return nil
}

// StartToPlayByModifiedDateInDescendingOrder by copying audio file to Downloads folder and click enter on it.
func (p *Player) StartToPlayByModifiedDateInDescendingOrder(ctx context.Context, filename, fullFilename string) error {
	return p.startToPlayBySorting(ctx, filename, fullFilename, true)
}

// StartToPlay by copying audio file to Downloads folder and click enter on it.
func (p *Player) StartToPlay(ctx context.Context, filename, fullFilename string) error {
	return p.startToPlayBySorting(ctx, filename, fullFilename, false)
}

// Play plays audio player "play" button.
func (p *Player) Play(ctx context.Context) error {
	if err := mtbfui.WaitForElement(ctx, p.tconn, ui.RoleTypeButton, "Play", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.AudioWaitPlayButton, err)
	}

	if err := mtbfui.ClickElement(ctx, p.tconn, ui.RoleTypeButton, "Play"); err != nil {
		return mtbferrors.New(mtbferrors.AudioClickPlayButton, err)
	}
	return nil

}

// Pause plays audio player "Pause" button.
func (p *Player) Pause(ctx context.Context) error {
	if err := mtbfui.WaitForElement(ctx, p.tconn, ui.RoleTypeButton, "Pause", time.Minute); err != nil {
		debug.TakeScreenshot(ctx)
		return mtbferrors.New(mtbferrors.AudioWaitPauseButton, err)
	}

	if err := mtbfui.ClickElement(ctx, p.tconn, ui.RoleTypeButton, "Pause"); err != nil {
		return mtbferrors.New(mtbferrors.AudioClickPauseButton, err)
	}
	return nil
}

// Close plays audio player "Close" button.
func (p *Player) Close(ctx context.Context) error {
	defer p.tconn.Close()
	defer p.tconn.CloseTarget(ctx)
	defer p.files.Close(ctx)
	if err := apps.Close(ctx, p.tconn, audioPlayerAppID); err != nil {
		return mtbferrors.New(mtbferrors.ChromeCloseApp, err, "Audio Player")
	}
	return nil
}

// GetPlayingTime returns audio player playing time.
func (p *Player) GetPlayingTime(ctx context.Context) (playtime int, err error) {
	javascript := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			let playTime;
			%v
			const recursive = root => {
				const regex = /(\d*):(\d*) \/ \d*:\d*/;
				const target = { attributes: { role: 'staticText', name: regex } };
				const playTimeNodes = root.findAll(target);
				if (playTimeNodes.length) {
					const node = playTimeNodes[1];
					const [full, min, sec] = regex.exec(node.name);
					playTime = Number(min) * 60 + Number(sec);
					console.log('GetAudioPlayingTime: ', node.name, ', playtime: ', playTime);
				}
			};
			chrome.automation.getDesktop(root => recursive(getAudioPlayer(root)));
			if (Number.isInteger(playTime)) {
				resolve(playTime);
			} else {
				reject("none time has been found.");
			}
		})`, ScriptTemplate["getAudioPlayer"])

	if err = p.tconn.EvalPromise(ctx, javascript, &playtime); err != nil {
		return -1, mtbferrors.New(mtbferrors.AudioPlayTime, err)
	}
	return
}

// IsPlaying verify audio is still playing.
func (p *Player) IsPlaying(ctx context.Context, timeout time.Duration) (err error) {
	return IsPlaying(ctx, p.tconn, timeout)
}

// IsPausing verify audio is now pausing.
func (p *Player) IsPausing(ctx context.Context, timeout time.Duration) (err error) {
	return IsPausing(ctx, p.tconn, timeout)
}
