// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	mtbfui "chromiumos/tast/local/mtbf/ui"
)

// ScriptTemplate is the script template for audio player.
var ScriptTemplate = map[string]string{
	"getAudioPlayer": `const getAudioPlayer =
		root => root.find({ attributes: { role: 'window', className: 'WidgetDelegateView', name: 'Audio Player' }});`,
}

// ClickButton clicks audio player button.
func ClickButton(ctx context.Context, conn *chrome.TestConn, name string) error {
	script := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			chrome.automation.getDesktop(root => {
				%v
				getAudioPlayer(root).find({ attributes: { role: 'button', name: %q } }).doDefault();
				resolve();
			})
		})`, ScriptTemplate["getAudioPlayer"], name)

	return conn.EvalPromise(ctx, script, nil)
}

// Play plays audio player "play" button.
func Play(ctx context.Context, conn *chrome.TestConn) error {
	if err := mtbfui.WaitForElement(ctx, conn, ui.RoleTypeButton, "Play", time.Minute); err != nil {
		return mtbferrors.New(mtbferrors.AudioWaitPlayButton, err)
	}

	if err := ClickButton(ctx, conn, "Play"); err != nil {
		return mtbferrors.New(mtbferrors.AudioClickPlayButton, err)
	}
	return nil
}

// Pause plays audio player "Pause" button.
func Pause(ctx context.Context, conn *chrome.TestConn) error {
	if err := mtbfui.WaitForElement(ctx, conn, ui.RoleTypeButton, "Pause", time.Minute); err != nil {
		return mtbferrors.New(mtbferrors.AudioWaitPauseButton, err)
	}

	if err := ClickButton(ctx, conn, "Pause"); err != nil {
		return mtbferrors.New(mtbferrors.AudioClickPauseButton, err)
	}
	return nil
}

// Close plays audio player "Close" button.
func Close(ctx context.Context, conn *chrome.TestConn) error {
	if err := ClickButton(ctx, conn, "Close"); err != nil {
		return mtbferrors.New(mtbferrors.AudioClickCloseButton, err)
	}
	return nil
}
