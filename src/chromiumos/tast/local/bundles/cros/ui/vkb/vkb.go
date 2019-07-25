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

// SetCurrentInputMethod sets the current input method used by the virtual
// keyboard.
func SetCurrentInputMethod(ctx context.Context, tconn *chrome.Conn, inputMethod string) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`
new Promise((resolve, reject) => {
	chrome.autotestPrivate.setWhitelistedPref(
		'settings.language.preload_engines', %[1]q, () => {
			chrome.inputMethodPrivate.setCurrentInputMethod(%[1]q, () => {
				if (chrome.runtime.lastError) {
					reject(chrome.runtime.lastError.message);
				} else {
					resolve();
				}
			});
		}
	);
})
`, inputMethod), nil)
}

// IsShown checks if the virtual keyboard is currently shown. It checks whether
// there is a visible DOM element with an accessibility role of "keyboard".
func IsShown(ctx context.Context, tconn *chrome.Conn) (shown bool, err error) {
	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const keyboard = root.find({ attributes: { role: 'keyboard' }});
		resolve(keyboard && !keyboard.state.invisible);
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
		const check = () => {
			try {
				const keyboard = root.find({ attributes: { role: 'keyboard' }});
				if (keyboard && !keyboard.state.invisible) {
					resolve();
					return;
				}
			} catch (e) {
				console.log(e);
			}
			setTimeout(check, 10);
		}
		check();
	});
})
`, nil)
}

// WaitUntilButtonsRender waits for the virtual keyboard to render some buttons.
func WaitUntilButtonsRender(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const check = () => {
			try {
				const keyboard = root.find({ attributes: { role: 'keyboard' }});
				// English keyboard should have at least 26 keys.
				if (keyboard && keyboard.findAll({ attributes: { role: 'button' }}).length >= 26) {
					resolve();
					return;
				}
			} catch (e) {
				console.log(e);
			}
			setTimeout(check, 10);
		}
		check();
	});
})
`, nil)
}

// UIConn returns a connection to the virtual keyboard HTML page,
// where JavaScript can be executed to simulate interactions with the UI.
// The connection is lazily created, and this function will block until the
// extension is loaded or ctx's deadline is reached. The caller should close
// the returned connection.
func UIConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	extURLPrefix := "chrome-extension://jkghodnilhceideoidjikpgommlajknk/inputview.html"
	f := func(t *chrome.Target) bool { return strings.HasPrefix(t.URL, extURLPrefix) }
	return c.NewConnForTarget(ctx, f)
}

// TapKey simulates a tap event on the middle of the specified key. The key can
// be any letter of the alphabet, "space" or "backspace".
func TapKey(ctx context.Context, kconn *chrome.Conn, key string) error {
	return kconn.Eval(ctx, fmt.Sprintf(`
	(() => {
		// Multiple keys can have the same aria label but only one is visible.
		const keys = document.querySelectorAll('[aria-label=%[1]q]')
		if (!keys) {
			throw new Error('Key %[1]q not found. No element with aria-label %[1]q.');
		}
		for (const key of keys) {
			const rect = key.getBoundingClientRect();
			if (rect.width <= 0 || rect.height <= 0) {
				continue;
			}
			const e = new Event('pointerdown');
			e.clientX = rect.x + rect.width / 2;
			e.clientY = rect.y + rect.height / 2;
			key.dispatchEvent(e);
			key.dispatchEvent(new Event('pointerup'));
			return;
		}
		throw new Error('Key %[1]q not clickable. Found elements with aria-label %[1]q, but they were not visible.');
	})()
`, key), nil)
}

// GetSuggestions returns suggestions that are currently displayed by the
// virtual keyboard.
func GetSuggestions(ctx context.Context, kconn *chrome.Conn) ([]string, error) {
	var suggestions []string
	err := kconn.Eval(ctx, `
	(() => {
		const elems = document.querySelectorAll('.candidate-span');
		return Array.prototype.map.call(elems, x => x.textContent);
	})()
`, &suggestions)
	return suggestions, err
}
