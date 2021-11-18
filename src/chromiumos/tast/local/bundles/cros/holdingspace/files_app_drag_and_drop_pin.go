// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FilesAppDragAndDropPin,
		Desc: "Checks that dragging and dropping on Holding Space pins the item",
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

// FilesAppDragAndDropPin tests the functionality of pinning files to Holding Space by
// dragging and dropping from the Files app.
func FilesAppDragAndDropPin(ctx context.Context, s *testing.State) {
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

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open filesapp: ", err)
	}

	// Reset the holding space and `MarkTimeOfFirstAdd` to make the `HoldingSpaceTrayIcon`
	// show.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn,
		holdingspace.ResetHoldingSpaceParam{MarkTimeOfFirstAdd: true}); err != nil {
		s.Fatalf("Failed to show holding space tray: %s", err)
	}

	// Create our file, with appropriate permissions so we can delete later.
	const testFile = "test.txt"
	testFilePath := filepath.Join(filesapp.MyFilesPath, testFile)
	if err := ioutil.WriteFile(testFilePath, []byte("Per aspera, ad astra"),
		0644); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFilePath, err)
	}
	defer os.Remove(testFilePath)

	uia := uiauto.New(tconn)

	// Get the location of the tray for dragging.
	tray := holdingspace.FindTray()
	trayLocation, err := uia.Location(ctx, tray)
	if err != nil {
		s.Fatalf("Failed to find holding space tray location: %s", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatalf("Failed to get keyboard: %s", err)
	}
	defer kb.Close()

	if err := uiauto.Combine("Drag and drop file on holding space",
		fsapp.
			DragAndDropFile(testFile, trayLocation.CenterPoint(), kb),
		uia.LeftClickUntil(holdingspace.FindTray(),
			uia.Exists(holdingspace.FindPinnedFileChip(testFile))),
	)(ctx); err != nil {
		s.Fatalf("Failed to pin item %q by dragging: %s", testFile, err)
	}

}
