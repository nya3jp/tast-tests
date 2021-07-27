// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DragAndDropPin,
		Desc: "Checks that pinning to Holding Space from the Files app works",
		Contacts: []string{
			"angusmclean@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// DragAndDropPin tests the functionality of pinning files to Holding Space by
// dragging and dropping.
func DragAndDropPin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatalf("Failed to get keyboard: %s", err)
	}
	defer kb.Close()

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open filesapp: ", err)
	}

	junkFile, err := holdingspace.ForceShowHoldingSpaceTrayIcon(ctx, tconn, fsapp)
	if err != nil {
		s.Fatalf("Junk file creation failed: %s", err)
	}
	defer os.Remove(junkFile)

	// Setup the test file.
	const testFile = "test.txt"
	testFilePath, err := holdingspace.CreateFile(ctx, tconn, testFile)

	if err != nil {
		s.Fatalf("Creating file %s failed: %s", testFilePath, err)
	}
	defer os.Remove(testFilePath)

	holdingSpaceTrayLocation, err := holdingspace.GetHoldingSpaceTrayLocation(ctx, tconn)
	if err != nil {
		s.Fatalf("Failed to get holding space tray: %s", err)
	}

	// Click and drag the testFile to the holding space tray. This pins it.
	fsapp.WithTimeout(5*time.Second).WithInterval(time.Second).
		DragAndDropFile(testFile, holdingSpaceTrayLocation.CenterPoint(), kb)(ctx)

	// Confirm testFile has been pinned.
	if err = holdingspace.FileIsPinned(ctx, tconn, testFile); err != nil {
		s.Fatalf("Failed to find file %s in holding space: %s", testFile, err)
	}
}
