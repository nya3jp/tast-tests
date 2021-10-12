// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains drivers for controlling the ui of Shimless
// RMA SWA.
package shimlessrma

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/shimlessrmaapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WelcomeNextCancel,
		Desc: "Can successfully start. move to next screen and cancel the Shimless RMA app",
		Contacts: []string{
			"gavindodd@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// WelcomeNextCancel verifies that the Shimless RMA app can open, move to the
// next screen and then cancel.
func WelcomeNextCancel(ctx context.Context, s *testing.State) {
	// Create a valid empty rmad state file.
	err := shimlessrmaapp.CreateEmptyStateFile()
	if err != nil {
		s.Fatal("Failed to create rmad state file: ", err)
	}
	defer shimlessrmaapp.RemoveStateFile()

	// Open Chrome with Shimless RMA enabled.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ShimlessRMAFlow"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx) // Close our own chrome instance

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Open Shimless RMA app.
	app, err := shimlessrmaapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Shimless RMA app: ", err)
	}

	// Wait for Welcome page to load.
	if err := app.WaitForWelcomePageToLoad()(ctx); err != nil {
		s.Fatal("Failed to load Welcome page: ", err)
	}

	// Click the next button
	if err := app.LeftClickNextButton()(ctx); err != nil {
		s.Fatal("Failed to click next button: ", err)
	}

	// Wait for Update OS page to load.
	if err := app.WaitForUpdateOSPageToLoad()(ctx); err != nil {
		s.Fatal("Failed to load Update OS page: ", err)
	}

	// Click the cancel button
	if err := app.LeftClickCancelButton()(ctx); err != nil {
		s.Fatal("Failed to click cancel button: ", err)
	}

	// Wait for cancel to complete.
	if err := app.WaitForStateFileDeleted()(ctx); err != nil {
		s.Fatal("Failed to cancel RMA, state file not deleted: ", err)
	}
}
