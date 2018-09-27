// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/bundles/cros/ui/vkb"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardOmnibox,
		Desc:         "Checks that the virtual keyboard appears when clicking on the omnibox",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func VirtualKeyboardOmnibox(s *testing.State) {
	ctx := s.Context()

	cr, err := chrome.New(s.Context(), chrome.ExtraArgs([]string{"--enable-virtual-keyboard"}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	s.Log("Waiting for the virtual keyboard to load in the background")
	if err := vkb.WaitUntilHidden(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to load in the background: ", err)
	}

	// Click on the omnibox.
	if err := tconn.WaitForPromise(ctx, `
new Promise((resolve, reject) => {
	chrome.automation.getDesktop(root => {
		const omnibox = root.find({ attributes: { role: 'textField', inputType: 'url' }});
		if (omnibox) {
			omnibox.doDefault();
			resolve(true);
		} else {
			resolve(false);
		}
	});
})
`); err != nil {
		s.Fatal("Failed to click the omnibox: ", err)
	}

	s.Log("Waiting for the virtual keyboard to show")
	if err := vkb.WaitUntilShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to show: ", err)
	}

	s.Log("Waiting for the virtual keyboard to render buttons")
	if err := vkb.WaitUntilButtonsRender(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the virtual keyboard to render: ", err)
	}
}
