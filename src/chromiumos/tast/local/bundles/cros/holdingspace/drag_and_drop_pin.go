// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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
		Pre:          chrome.LoggedInDisableSync(),
	})
}

// DragAndDropPin tests the functionality of pinning files to Holding Space by dragging
// and dropping.
func DragAndDropPin(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open filesapp: ", err)
	}

	// Create a filler file in holding space. This exists to make holding space to show
	// since this is a clean session with no existing holding space usage.
	fillerFileName := "1.txt"
	fillerFilePath, err := holdingspace.CreateAndPinNewfile(ctx, tconn, fsapp, fillerFileName)
	if err != nil {
		s.Fatalf("Failed to create filler file: %s", err)
	}
	defer os.Remove(fillerFilePath)

	// Setup the test file.
	const testFile = "test.txt"
	testFilePath := filepath.Join(filesapp.MyFilesPath, testFile)

	// Create our test file, with appropriate permissions so we can delete later.
	if err := ioutil.WriteFile(testFilePath, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFilePath, err)
	}
	defer os.Remove(testFilePath)

	holdingSpaceTray, err := holdingspace.GetHoldingSpaceTrayNode(ctx, tconn)
	if err != nil {
		s.Fatalf("Failed to get holding space tray: %s", err)
	}

	// Click and drag the testFile to the holding space tray. This pins it.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatalf("Failed to get keyboard: %s", err)
	}
	defer kb.Close()
	fsapp.WithTimeout(5*time.Second).WithInterval(time.Second).
		DragAndDropFile(testFile, holdingSpaceTray.Location.CenterPoint(), kb)(ctx)

	// Confirm testFile has been pinned.
	_, err = holdingspace.GetPinnedItem(ctx, tconn, testFile)
	if err != nil {
		s.Fatalf("Failed to find file %s in holding space: %s", testFile, err)
	}

	// Unpin the second item through the context menu in HoldingSpace.
	holdingspace.UnpinItem(ctx, tconn, testFile)
	holdingspace.UnpinItem(ctx, tconn, fillerFileName)
}
