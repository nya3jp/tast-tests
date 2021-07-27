// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinToToteFromFiles,
		Desc: "Checks that pinning to Tote from the Files app works",
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

// PinToToteFromFiles tests the functionality of pinning files to Tote from the Files app
func PinToToteFromFiles(ctx context.Context, s *testing.State) {
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

	// Setup the test files.
	const textFile1 = "test1.txt"
	const textFile2 = "test2.txt"
	testFileLocation1 := filepath.Join(filesapp.MyFilesPath, textFile1)
	testFileLocation2 := filepath.Join(filesapp.MyFilesPath, textFile2)

	// Create our two test files
	if err := ioutil.WriteFile(testFileLocation1, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation1, err)
	}
	defer os.Remove(testFileLocation1)

	if err := ioutil.WriteFile(testFileLocation2, []byte("blahblah"), 0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation2, err)
	}
	defer os.Remove(testFileLocation2)

	// Pin the first file to the shelf by using the context menu in the files app
	if err := fsapp.ClickContextMenuItem(textFile2, "Pin to shelf")(ctx); err != nil {
		s.Fatalf("Pinning file %s failed: %s", testFileLocation2, err)
	}

	// Make sure the first item got pinned
	_, err = ash.GetPinnedItem(ctx, tconn, textFile2)
	if err != nil {
		s.Fatalf("Failed to find item %s: %s", textFile2, err)
	}

	// Drag and drop to pin the second item
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatalf("Failed to get keyboard: %s", err)
	}
	defer kb.Close()

	toteIcon, err := ash.GetToteTrayIconNode(ctx, tconn)
	if err != nil {
		s.Fatalf("Failed to get tote tray icon: %s", err)
	}

	fsapp.WithTimeout(5*time.Second).WithInterval(time.Second).DragAndDropFile(textFile1, toteIcon.Location.CenterPoint(), kb)(ctx)

	// Confirm the second item has been pinned
	_, err = ash.GetPinnedItem(ctx, tconn, textFile1)
	if err != nil {
		s.Fatalf("Failed to find file %s in holding space: %s", textFile1, err)
	}

	// Unpin the first item through the context menu in the files app
	if err := fsapp.ClickContextMenuItem(textFile1, "Unpin from shelf")(ctx); err != nil {
		s.Fatalf("Unpinning file %s failed: %s", testFileLocation1, err)
	}
	if err := ash.WaitUntilNotPinned(ctx, tconn, textFile1); err != nil {
		s.Fatalf("File %s did not unpin: %s", testFileLocation1, err)
	}

	// Unpin the second item through the context menu in tote
	ash.UnpinItem(ctx, tconn, textFile2)
}
