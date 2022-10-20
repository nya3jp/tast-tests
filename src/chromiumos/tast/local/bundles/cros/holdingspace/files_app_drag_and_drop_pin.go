// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
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
// dragging and dropping from the Files app.
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
	const testFile = "test.txt"
	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's MyFiles path: ", err)
	}
	testFilePath := filepath.Join(myFilesPath, testFile)
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

	if err := uiauto.Combine("Drag and drop file on holding space",
		fsapp.DragAndDropFile(testFile, trayLocation.CenterPoint(), kb),
		uia.LeftClick(holdingspace.FindTray()),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(testFile)),
	)(ctx); err != nil {
		s.Fatalf("Failed to pin item %q by dragging: %s", testFile, err)
	}
	fileLocation, err := uia.Location(ctx, holdingspace.FindPinnedFileChip().Name(testFile))
	if err != nil {
		s.Fatal("Failed to get holding space test file location: ", err)
	}

	fsappLocation, err := uia.Location(ctx, nodewith.Role(role.ListBox))
	if err != nil {
		s.Fatal("Failed to get Files App location: ", err)
	}

	// Copy text file to Files App by drag and drop.
	uia.LeftClick(tray)
	mouse.Drag(tconn, fileLocation.CenterPoint(), fsappLocation.CenterPoint(), time.Second)(ctx)
	// Check holding space is closed.
	uia.Gone(holdingspace.FindChip())
	if err != nil {
		s.Fatal("Failed to move file to Files Download: ", err)
	}
	const copyTestFile = "test (1).txt"
	defer os.Remove(filepath.Join(myFilesPath, copyTestFile))

	// Open chrome browser.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to launch chrome app: ", err)
	}
	defer apps.Close(ctx, tconn, apps.Chrome.ID)

	chromeWindowFinder := nodewith.NameContaining("Google Chrome").Role(role.Window)
	chromeLocation, err := uia.Location(ctx, chromeWindowFinder.HasClass("BrowserRootView"))

	// Open test file in Chrome browser by drag and drop.
	// uia.LeftClick(tray)
	mouse.Click(tconn, trayLocation.CenterPoint(), mouse.LeftButton)
	uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(testFile))
	mouse.Drag(tconn, fileLocation.CenterPoint(), chromeLocation.CenterPoint(), time.Second)(ctx)
	// Check holding space is closed.
	uia.Gone(holdingspace.FindChip())
	uia.WaitUntilExists(nodewith.Role(role.Tab).Name("testFile"))(ctx)
	if err != nil {
		s.Fatal("Failed to open test file in Chrome browser: ", err)
	}

}
