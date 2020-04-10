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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/debug"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

// PlayFromDownloadsFolder copies audio files to download folder using given name, and plays audio file by clicking the it.
func PlayFromDownloadsFolder(ctx context.Context, files *filesapp.FilesApp, srcFile string, dstFileMame string) (err error) {
	// Copy file to download folder.
	audioFileLocation := filepath.Join(filesapp.DownloadPath, dstFileMame)
	if err := fsutil.CopyFile(srcFile, audioFileLocation); err != nil {
		return mtbferrors.New(mtbferrors.VideoCopy, err, dstFileMame, audioFileLocation)
	}

	testing.ContextLog(ctx, "Open the Downloads folder")
	if err := files.OpenDownloads(ctx); err != nil {
		return mtbferrors.New(mtbferrors.ChromeOpenFolder, err, "Downloads")
	}

	testing.ContextLog(ctx, "Wait for audio file shows on files app")
	if err := files.WaitForElement(ctx, filesapp.RoleStaticText, dstFileMame, 10*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeClickItem, err, dstFileMame)
	}
	// Make sure UI is stable before clicking
	testing.Sleep(ctx, 2*time.Second)
	if err := files.ClickElement(ctx, filesapp.RoleStaticText, dstFileMame); err != nil {
		return mtbferrors.New(mtbferrors.ChromeRenderTime, err, dstFileMame)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeGetKeyboard, err)
	}
	defer kb.Close()
	testing.Sleep(ctx, 2*time.Second)

	testing.ContextLogf(ctx, "Open Audio Player to play %s", dstFileMame)
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
