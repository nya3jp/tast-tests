// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package virtualkeyboard

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Omnibox,
		Desc:         "Checks that the virtual keyboard appears when clicking on the omnibox",
		SoftwareDeps: []string{"chrome_login"},
	})
}

// Click on the omnibox using the automation API.
func clickOmnibox(ctx context.Context, tconn *chrome.Conn) error {
	return tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		root.addEventListener('loadComplete', () => {
			const omnibox = root.find({ attributes: { role: 'textField', inputType: 'url' }});
			if (omnibox) {
				omnibox.doDefault();
				return resolve();
			} else {
				return reject('Could not find the omnibox in accessibility tree');
			}
		});
	});
})
`, nil)
}

// Checks if the virtual keyboard is shown.
func isVirtualKeyboardShown(ctx context.Context, tconn *chrome.Conn) (shown bool, err error) {
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

// Wait for the virtual keyboard to appear.
func waitForVirtualKeyboardToShow(ctx context.Context, tconn *chrome.Conn) error {
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

// Wait for the virtual keyboard to render some buttons.
func waitForVirtualKeyboardToRenderButtons(ctx context.Context, tconn *chrome.Conn) error {
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

func Omnibox(s *testing.State) {
	defer faillog.SaveIfError(s)

	ctx := s.Context()

	cr, err := chrome.New(s.Context(), chrome.VirtualKeyboardEnabled())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Check that virtual keyboard is hidden initially.
	shown, err := isVirtualKeyboardShown(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the virtual keyboard from the accesibility tree: ", err)
	}
	if shown {
		s.Fatal("Virtual keyboard is shown, but expected it to be hidden")
	}

	// Click on the omnibox to trigger the virtual keyboard.
	if err := clickOmnibox(ctx, tconn); err != nil {
		s.Fatal("Failed to click the omnibox: ", err)
	}

	// Wait for the virtual keyboard to be shown.
	s.Log("Waiting for the virtual keyboard to show")
	if err := waitForVirtualKeyboardToShow(ctx, tconn); err != nil {
		s.Fatal("Failed to get the virtual keyboard from the accessibility tree: ", err)
	}

	// Wait for the virtual keyboard to render.
	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := waitForVirtualKeyboardToRenderButtons(ctx, tconn); err != nil {
		s.Fatal("Failed to get the virtual keyboard from the accessibility tree: ", err)
	}
}
