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
	"chromiumos/tast/errors"
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

type dragAndDropParams struct {
	testfunc func(context.Context, *chrome.TestConn, *uiauto.Context, *input.KeyboardEventWriter, *filesapp.FilesApp, string, []string) error
}

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
		Params: []testing.Param{{
			Name: "single_drag_and_drop",
			Val: dragAndDropParams{
				testfunc: testSingleFileDragAndDrop,
			},
		}, {
			Name: "multiple_drag_and_drop",
			Val: dragAndDropParams{
				testfunc: testMultipleFilesDragAndDrop,
			},
		}},
	})
}

// FilesAppDragAndDropPin tests the functionality of pinning files to Holding Space by
// dragging and dropping single/multiple files to/from the Files app.
func FilesAppDragAndDropPin(ctx context.Context, s *testing.State) {
	params := s.Param().(dragAndDropParams)
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

	// Create our first file, with appropriate permissions so we can delete later.
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

	listFiles := []string{testFile1, testFile2, testFile3}

	uia := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Perform additional parameterized testing.
	if err := params.testfunc(ctx, tconn, uia, kb, fsapp, myFilesPath, listFiles); err != nil {
		s.Fatal("Fail to perform parameterized testing: ", err)
	}

}

func testSingleFileDragAndDrop(ctx context.Context, tconn *chrome.TestConn, uia *uiauto.Context, kb *input.KeyboardEventWriter, fsapp *filesapp.FilesApp, myFilesPath string, listFiles []string) error {
	// Get the location of the tray for dragging.
	tray := holdingspace.FindTray()
	trayLocation, err := uia.Location(ctx, tray)
	if err != nil {
		errors.Wrap(err, "failed to find holding space tray location")
	}

	if err := uiauto.Combine("Pin single file from Files App to Holding Space by drag and drop",
		fsapp.DragAndDropFile(listFiles[0], trayLocation.CenterPoint(), kb),
		uia.LeftClick(tray),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(listFiles[0])),
	)(ctx); err != nil {
		errors.Errorf("failed to pin file %q by dragging: %s", listFiles[0], err)
	}

	fileLocation1, err := uia.Location(ctx, holdingspace.FindPinnedFileChip().Name(listFiles[0]))
	if err != nil {
		errors.Wrap(err, "failed to get holding space test file location")
	}

	fsappLocation, err := uia.Location(ctx, nodewith.Role(role.ListBox))
	if err != nil {
		errors.Wrap(err, "failed to get Files App location")
	}

	if err := uiauto.Combine("Drag and drop a single pinned file from Holding Space to Files App",
		mouse.Drag(tconn, fileLocation1.CenterPoint(), fsappLocation.CenterPoint(), time.Second),
		uia.Gone(holdingspace.FindChip()),
	)(ctx); err != nil {
		errors.Errorf("failed to drag and drop a single pinned file %v from Holding Space to Files App: %s", listFiles[0], err)
	}

	// Remove the copied file later.
	defer os.Remove(filepath.Join(myFilesPath, "test (1).txt"))

	return nil
}

func testMultipleFilesDragAndDrop(ctx context.Context, tconn *chrome.TestConn, uia *uiauto.Context, kb *input.KeyboardEventWriter, fsapp *filesapp.FilesApp, myFilesPath string, listFiles []string) error {
	tray := holdingspace.FindTray()
	trayLocation, err := uia.Location(ctx, tray)
	if err != nil {
		errors.Wrap(err, "failed to find holding space tray location")
	}

	fsappLocation, err := uia.Location(ctx, nodewith.Role(role.ListBox))
	if err != nil {
		errors.Wrap(err, "failed to get Files App location")
	}

	if err := uiauto.Combine("Pin multiple files from Files App to Holding Space by drag and drop",
		fsapp.DragAndDropFile(strings.Join(listFiles[1:], " "), trayLocation.CenterPoint(), kb),
		uia.LeftClick(tray),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(listFiles[1])),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(listFiles[2])),
	)(ctx); err != nil {
		errors.Errorf("failed to pin multiple items %v from Files App to Holding Space by drag and drop: %s", listFiles[1:], err)
	}

	if err := uiauto.Combine("Copy multiple files from Holding Space to Files App by drag and drop",
		func(ctx context.Context) error {
			if err := kb.AccelPress(ctx, "Ctrl"); err != nil {
				errors.Wrap(err, "failed to press Ctrl")
			}
			// Select multiple files in Holding Space.
			for _, fileName := range listFiles {
				if err := uia.LeftClick(holdingspace.FindPinnedFileChip().Name(fileName))(ctx); err != nil {
					errors.Errorf("failed to select %s : %s", fileName, err)
				}
			}
			fileLocation3, err := uia.Location(ctx, holdingspace.FindPinnedFileChip().Name(listFiles[len(listFiles)-1]))
			if err != nil {
				errors.Wrap(err, "failed to get holding space test file location")
			}
			if err := kb.AccelRelease(ctx, "Ctrl"); err != nil {
				errors.Wrap(err, "failed to release Ctrl")
			}
			if err = mouse.Drag(tconn, fileLocation3.CenterPoint(), fsappLocation.CenterPoint(), time.Second)(ctx); err != nil {
				errors.Errorf("failed to drag and drop multiple pinned files %v from Holding Space to Files App: %s", listFiles[1:], err)
			}
			return err
		},
		uia.Gone(holdingspace.FindChip()),
	)(ctx); err != nil {
		errors.Wrap(err, "failed to Copy multiple files from Holding Space to Files App by drag and drop")
	}

	// Remove the copied files later.
	defer os.Remove(filepath.Join(myFilesPath, "test (2).txt"))
	defer os.Remove(filepath.Join(myFilesPath, "test (3).txt"))

	return nil
}
