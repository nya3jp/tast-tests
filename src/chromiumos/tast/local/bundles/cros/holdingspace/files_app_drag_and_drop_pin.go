// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesAppDragAndDropPin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that dragging and dropping on Holding Space pins the item",
		Contacts: []string{
			"angusmclean@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// FilesAppDragAndDropPin tests the functionality of pinning files to Holding Space by
// dragging and dropping single/multiple files to/from the Files app.
func FilesAppDragAndDropPin(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open filesapp: ", err)
	}
	defer fsapp.Close(ctx)

	// Reset the holding space and `MarkTimeOfFirstAdd` to make the `HoldingSpaceTrayIcon`
	// show.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn,
		holdingspace.ResetHoldingSpaceOptions{MarkTimeOfFirstAdd: true}); err != nil {
		s.Fatal("Failed to reset holding space: ", err)
	}

	// Create our file, with appropriate permissions so we can delete later.
	const testFile1 = "test1.txt"
	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's MyFiles path: ", err)
	}
	testFilePath := filepath.Join(myFilesPath, testFile1)
	if err := ioutil.WriteFile(testFilePath, []byte("Per aspera, ad astra"),
		0644); err != nil {
		s.Fatalf("Creating file %q failed: %s", testFilePath, err)
	}
	defer os.Remove(testFilePath)

	uia := uiauto.New(tconn)

	// Get the location of the tray for dragging.
	tray := holdingspace.FindTray()
	trayLocation, err := uia.Location(ctx, tray)
	if err != nil {
		s.Fatal("Failed to find holding space tray location: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := uiauto.Combine("Pin single file from Files App to Holding Space by drag and drop",
		fsapp.DragAndDropFile(testFile1, trayLocation.CenterPoint(), kb),
		uia.LeftClick(tray),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(testFile1)),
	)(ctx); err != nil {
		s.Fatalf("Failed to pin file %q by dragging: %s", testFile1, err)
	}

	// Create a second file, with appropriate permissions so we can delete later.
	const testFile2 = "test2.txt"
	testFilePath2 := filepath.Join(myFilesPath, testFile2)
	if err := ioutil.WriteFile(testFilePath2, []byte("You're a mean one, Mr. Grinch"),
		0644); err != nil {
		s.Fatalf("Creating file %q failed: %s", testFilePath2, err)
	}
	defer os.Remove(testFilePath2)

	// Create a third file, with appropriate permissions so we can delete later.
	const testFile3 = "test3.txt"
	testFilePath3 := filepath.Join(myFilesPath, testFile3)
	if err := ioutil.WriteFile(testFilePath3, []byte("You're a wizard Harry"),
		0644); err != nil {
		s.Fatalf("Creating file %q failed: %s", testFilePath3, err)
	}
	defer os.Remove(testFilePath3)

	listFiles := []string{testFile2, testFile3}

	if err := uiauto.Combine("Pin multiple files from Files App to Holding Space by drag and drop",
		fsapp.DragAndDropFile(strings.Join(listFiles[:], " "), trayLocation.CenterPoint(), kb),
		uia.LeftClick(tray),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(listFiles[0])),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(listFiles[1])),
	)(ctx); err != nil {
		s.Fatalf("Failed to pin multiple items %v from Files App to Holding Space by drag and drop: %s", listFiles, err)
	}

	fileLocation1, err := uia.Location(ctx, holdingspace.FindPinnedFileChip().Name(testFile1))
	if err != nil {
		s.Fatal("Failed to get holding space test file location: ", err)
	}

	fsappLocation, err := uia.Location(ctx, nodewith.Role(role.ListBox))
	if err != nil {
		s.Fatal("Failed to get Files App location: ", err)
	}

	// Copy single file from Holding Space to Files App by drag and drop.
	if err := uiauto.Combine("Drag and drop a single pinned file from Holding Space to Files App",
		mouse.Drag(tconn, fileLocation1.CenterPoint(), fsappLocation.CenterPoint(), time.Second),
		uia.Gone(holdingspace.FindChip()),
	)(ctx); err != nil {
		s.Fatalf("Failed drag and drop a single pinned file %v from Holding Space to Files App: %s", testFile1, err)
	}
	// Remove the copied file later.
	const copyTestFile1 = "test (1).txt"
	defer os.Remove(filepath.Join(myFilesPath, copyTestFile1))

	// Copy multiple files from Holding Space to Files App by drag and drop.
	uia.LeftClick(tray)(ctx)
	// Hold Ctrl during multi selection.
	if err := kb.AccelPress(ctx, "Ctrl"); err != nil {
		s.Fatal("Failed to press Ctrl: ", err)
	}
	// Select multiple files in Holding Space.
	for _, fileName := range listFiles {
		if err := uia.LeftClick(holdingspace.FindPinnedFileChip().Name(fileName))(ctx); err != nil {
			s.Fatalf("Failed to select %s : %s", fileName, err)
		}
	}
	// Get location of last test file.
	fileLocation3, err := uia.Location(ctx, holdingspace.FindPinnedFileChip().Name(listFiles[len(listFiles)-1]))
	if err != nil {
		s.Fatal("Failed to get holding space test file location: ", err)
	}
	// Release Ctrl.
	if err := kb.AccelRelease(ctx, "Ctrl"); err != nil {
		s.Fatal("Failed to release Ctrl: ", err)
	}
	err = mouse.Drag(tconn, fileLocation3.CenterPoint(), fsappLocation.CenterPoint(), time.Second)(ctx)
	if err != nil {
		s.Fatalf("Failed to drag and drop multiple pinned files %v from Holding Space to Files App: %s", listFiles, err)
	}
	uia.Gone(holdingspace.FindChip())(ctx)
	if err != nil {
		s.Fatal("Failed to automatically close Holding Space by dragging item out of Holding Space: ", err)
	}
	// Remove the copied files later.
	const copyTestFile2 = "test (2).txt"
	defer os.Remove(filepath.Join(myFilesPath, copyTestFile2))
	const copyTestFile3 = "test (3).txt"
	defer os.Remove(filepath.Join(myFilesPath, copyTestFile3))

}
