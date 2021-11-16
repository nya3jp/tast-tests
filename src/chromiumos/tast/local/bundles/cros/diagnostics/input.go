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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Input,
		Desc: "Can successfully navigate to the Input page",
		Contacts: []string{
			"hcutts@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// Input verifies that the Input page can be navigated to.
func Input(ctx context.Context, s *testing.State) {
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

	// Find the Input navigation item.
	ui := uiauto.New(tconn)
	inputTab := diagnosticsapp.DxInput.Ancestor(dxRootnode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(inputTab)(ctx); err != nil {
		s.Fatal("Failed to find the Input navigation item: ", err)
	}
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
	if err := ui.WithPollOpts(pollOpts).LeftClick(inputTab)(ctx); err != nil {
		s.Fatal("Could not click the Input tab: ", err)
	}

	// Find the keyboard list header.
	keyboardListHeading := diagnosticsapp.DxKeyboardHeading.Ancestor(dxRootnode)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(keyboardListHeading)(ctx); err != nil {
		s.Fatal("Failed to find the keyboard list heading: ", err)
	}
}
