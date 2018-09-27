// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vkb contains shared code to interact with the virtual keyboard.
package vkb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// WaitForPromiseToResolveToTrue repeatedly evaluates the JavaScript expression expr
// (which must return a Promise) until it resolves to true.
func WaitForPromiseToResolveToTrue(ctx context.Context, c *chrome.Conn, expr string) error {
	falseErr := fmt.Errorf("%q resolved to false", expr)
	err := testing.Poll(ctx, func(ctx context.Context) error {
		v := false
		if err := c.EvalPromise(ctx, expr, &v); err != nil {
			return err
		} else if !v {
			return falseErr
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond})
	if err != nil {
		return err
	}
	return nil
}

// ShowVirtualKeyboard forces the virtual keyboard to open.
func ShowVirtualKeyboard(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.inputMethodPrivate.showInputView(resolve);
})
`, nil)
}

// WaitUntilHidden waits for the virtual keyboard to be hidden. It waits until there
// there is an invisible DOM element with accessibility role of "keyboard".
func WaitUntilHidden(ctx context.Context, tconn *chrome.Conn) error {
	return WaitForPromiseToResolveToTrue(ctx, tconn, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const keyboard = root.find({ attributes: { role: 'keyboard' }});
		resolve(!!keyboard && keyboard.state.invisible);
	});
})
`)
}

// WaitUntilShown waits for the virtual keyboard to appear. It waits until there
// there is a visible DOM element with accessibility role of "keyboard".
func WaitUntilShown(ctx context.Context, tconn *chrome.Conn) error {
	return WaitForPromiseToResolveToTrue(ctx, tconn, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const keyboard = root.find({ attributes: { role: 'keyboard' }});
		resolve(!!keyboard && !keyboard.state.invisible);
	});
})
`)
}

// WaitUntilButtonsRender waits for the virtual keyboard to render some buttons.
func WaitUntilButtonsRender(ctx context.Context, tconn *chrome.Conn) error {
	return WaitForPromiseToResolveToTrue(ctx, tconn, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const keyboard = root.find({ attributes: { role: 'keyboard' }});
		// English keyboard should have at least 26 keys.
		resolve(!!keyboard && keyboard.findAll({ attributes: { role: 'button' }}).length >= 26);
	});
})
`)
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
