// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KeyboardList,
		Desc: "Can see currently-connected keyboards on the input page",
		Contacts: []string{
			"hcutts@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// KeyboardList verifies that the input page keyboard list reflects the currently connected keyboards.
func KeyboardList(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("DiagnosticsAppNavigation", "EnableInputInDiagnosticsApp"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx) // Close our own chrome instance

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	dxRootnode, err := diagnosticsapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}

	// Find the Input navigation item and the keyboard list.
	const timeout = 10 * time.Second
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: timeout}
	ui := uiauto.New(tconn)
	inputTab := diagnosticsapp.DxInput.Ancestor(dxRootnode)
	//keyboardList := diagnosticsapp.DxKeyboardList.Ancestor(dxRootnode)
	keyboardListHeading := diagnosticsapp.DxKeyboardHeading.Ancestor(dxRootnode)
	if err := uiauto.Combine("find the keyboard list heading",
		ui.WithTimeout(20 * time.Second).WaitUntilExists(inputTab),
		ui.WithPollOpts(pollOpts).LeftClick(inputTab),
		//ui.WithTimeout(timeout).WaitUntilExists(keyboardList),
		ui.WithTimeout(timeout).WaitUntilExists(keyboardListHeading),
	)(ctx); err != nil {
		s.Fatal("Failed to find the keyboard list heading: ", err)
	}

	// FIXME: instead of looking for one particular keyboard entry:
	//  Create a fake keyboard with uinput before loading the app
	//  Check that keyboard is shown in the list
	//  Remove the fake keyboard, check it leaves the list
	//  Add another fake keyboard, check it shows up

	keyboardEntry := nodewith.HasClass("device-name").Name("AT Translated Set 2 keyboard").Ancestor(dxRootnode)
	if err := ui.WithTimeout(timeout).WaitUntilExists(keyboardEntry); err != nil {
		s.Fatal("Failed to find the keyboard entry: ", err)
	}
}
