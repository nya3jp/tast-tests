// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vkb contains shared code to interact with the virtual keyboard.
package vkb

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// IsShown checks if the virtual keyboard is currently shown. It checks whether
// there is a visible DOM element with an accessibility role of "keyboard".
func IsShown(ctx context.Context, tconn *chrome.Conn) (shown bool, err error) {
	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		root.addEventListener('loadComplete', () => {
			const keyboard = root.find({ attributes: { role: 'keyboard' }});
			return resolve(keyboard && !keyboard.state.invisible);
		});
	});
})
`, &shown); err != nil {
		return false, err
	}

	return shown, nil
}

// WaitUntilShown waits for the virtual keyboard to appear. It waits until there
// there is a visible DOM element with accessibility role of "keyboard".
func WaitUntilShown(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		setInterval(() => {
			const keyboard = root.find({ attributes: { role: 'keyboard' }});
			if (keyboard && !keyboard.state.invisible)
				resolve();
		}, 500);
	});
})
`, nil)
}

// WaitUntilButtonsRender waits for the virtual keyboard to render some buttons.
func WaitUntilButtonsRender(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		setInterval(() => {
			const keyboard = root.find({ attributes: { role: 'keyboard' }});
			if (keyboard) {
				const buttons = keyboard.findAll({ attributes: { role: 'button' }});
				// English keyboard should have at least 26 keys.
				if (buttons.length >= 26)
					resolve();
			}
		}, 500);
	});
})
`, nil)
}
