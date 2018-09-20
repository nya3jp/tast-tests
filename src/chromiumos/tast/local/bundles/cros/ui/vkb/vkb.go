// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vkb contains shared code to interact with the virtual keyboard.
package vkb

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/local/chrome"
)

// ShowVirtualKeyboard forces the virtual keyboard to open.
func ShowVirtualKeyboard(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.inputMethodPrivate.showInputView(resolve);
})
`, nil)
}

// IsShown checks if the virtual keyboard is currently shown. It checks whether
// there is a visible DOM element with an accessibility role of "keyboard".
func IsShown(ctx context.Context, tconn *chrome.Conn) (shown bool, err error) {
	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		root.addEventListener('loadComplete', () => {
			const keyboard = root.find({ attributes: { role: 'keyboard' }});
			resolve(!!keyboard && !keyboard.state.invisible);
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
			if (keyboard && !keyboard.state.invisible) {
				resolve();
			}
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
				if (buttons.length >= 26) {
					resolve();
				}
			}
		}, 500);
	});
})
`, nil)
}

// UIConn returns a connection to the virtual keyboard HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The connection is lazily created, and this function will block until the
// extension is loaded or ctx's deadline is reached. The caller should close
// the returned connection.
func UIConn(c *chrome.Chrome, ctx context.Context) (*chrome.Conn, error) {
	extURLPrefix := "chrome-extension://jkghodnilhceideoidjikpgommlajknk/inputview.html"
	f := func(t *chrome.Target) bool { return strings.HasPrefix(t.URL, extURLPrefix) }
	return c.NewConnForTarget(ctx, f)
}

// TapKey simulates a tap event on the middle of the specified key. The key can
// be any letter of the alphabet, "space" or "backspace".
func TapKey(ctx context.Context, kconn *chrome.Conn, key string) error {
	return kconn.Eval(ctx, fmt.Sprintf(`
	(() => {
		const key = document.querySelector('[aria-label=%[1]q]');
		if (!key) {
			throw new Error('Key %[1]q not found. No element with aria-label %[1]q.');
		}
		const rect = key.getBoundingClientRect();
		const e = new Event('pointerdown');
		e.clientX = rect.x + rect.width / 2;
		e.clientY = rect.y + rect.height / 2;
		key.dispatchEvent(e);
		key.dispatchEvent(new Event('pointerup'));
	})()
`, key), nil)
}
