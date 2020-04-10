// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// ScriptTemplate is the script template for audio player.
var ScriptTemplate = map[string]string{
	"getAudioPlayer": `const getAudioPlayer =
		root => root.find({ attributes: { role: 'window', className: 'WidgetDelegateView', name: 'Audio Player' }});`,
}

// ClickButton clicks audio player button.
func ClickButton(ctx context.Context, conn *chrome.Conn, name string) error {
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
func Play(ctx context.Context, conn *chrome.Conn) error {
	return ClickButton(ctx, conn, "Play")
}

// Pause plays audio player "Pause" button.
func Pause(ctx context.Context, conn *chrome.Conn) error {
	return ClickButton(ctx, conn, "Pause")
}

// Close plays audio player "Close" button.
func Close(ctx context.Context, conn *chrome.Conn) error {
	return ClickButton(ctx, conn, "Close")
}
