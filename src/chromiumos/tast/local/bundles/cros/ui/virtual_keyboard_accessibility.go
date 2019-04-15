// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/vkb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardAccessibility,
		Desc:         "Checks that the accessibility keyboard displays correctly",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "virtual_keyboard"},
	})
}

func VirtualKeyboardAccessibility(ctx context.Context, s *testing.State) {
	// Do not use --enable-virtual-keyboard as it will be enabled via
	// accessibility prefs.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	shown, err := vkb.IsShown(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if the virtual keyboard is initially hidden: ", err)
	}
	if shown {
		s.Fatal("Virtual keyboard is shown, but expected it to be hidden")
	}

	s.Log("Enabling the accessibility keyboard")
	if err := tconn.EvalPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.autotestPrivate.setWhitelistedPref(
		'settings.a11y.virtual_keyboard', true, resolve);
})
`, nil); err != nil {
		s.Fatal("Failed to enable the accessibility keyboard: ", err)
	}

	if err := vkb.ShowVirtualKeyboard(ctx, tconn); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	s.Log("Waiting for the virtual keyboard to show")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}

	kconn, err := vkb.UIConn(ctx, cr)
	if err != nil {
		s.Fatal("Creating connection to virtual keyboard UI failed: ", err)
	}
	defer kconn.Close()

	// Check that the keyboard has modifier and tab keys.
	keys := []string{"ctrl", "alt", "caps lock", "tab"}

	for _, key := range keys {
		if err := vkb.TapKey(ctx, kconn, key); err != nil {
			s.Errorf("Failed to tap %q: %v", key, err)
		}
	}
}
