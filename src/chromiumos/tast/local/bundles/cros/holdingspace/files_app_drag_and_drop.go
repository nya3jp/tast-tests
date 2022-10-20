// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"fmt"
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
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesAppDragAndDrop,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks dragging and dropping to/from Holding Space from/to Files App",
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
			Name: "single_file",
			Val:/*count=*/ 1,
		}, {
			Name: "multiple_files",
			Val:/*count=*/ 2,
		}},
	})
}

// FilesAppDragAndDrop tests the functionality of pinning files to Holding Space by
// dragging and dropping single/multiple files from Files app to Holding Space.
// FilesAppDragAndDrop tests the functionality of copying files to Files App by
// dragging and dropping single/multiple pinned files from Holding Space to the Files app.
func FilesAppDragAndDrop(ctx context.Context, s *testing.State) {
	param := s.Param().(int)
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	myFilesPath, err := cryptohome.MyFilesPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's MyFiles path: ", err)
	}

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

	// Create number of files needed.
	var listFiles []string
	for i := 0; i < param; i++ {
		testFile := fmt.Sprintf("test%d.txt", i)
		testFilePath := filepath.Join(myFilesPath, testFile)
		if err := ioutil.WriteFile(testFilePath, []byte("Per aspera, ad astra"), 0644); err != nil {
			s.Fatalf("Failed to create file %q: %s", testFilePath, err)
		}
		defer os.Remove(testFilePath)
		listFiles = append(listFiles, testFile)
	}

	uia := uiauto.New(tconn)
	tray := holdingspace.FindTray()
	trayLocation, err := uia.Location(ctx, tray)
	if err != nil {
		s.Fatal("Failed to find holding space tray location: ", err)
	}

	fsappLocation, err := uia.Location(ctx, filesapp.WindowFinder(apps.FilesSWA.ID))
	if err != nil {
		s.Fatal("Failed to get Files App location: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := uiauto.Combine(fmt.Sprintf("Pin %d file(s) from Files App to Holding Space by drag and drop", param),
		fsapp.DragAndDropFiles(listFiles, trayLocation.CenterPoint(), kb),
		uia.LeftClick(tray),
		func(ctx context.Context) error {
			for _, fileName := range listFiles {
				if err := uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(fileName))(ctx); err != nil {
					s.Fatalf("Failed to verify that %q exists in holding space: %s", fileName, err)
				}
			}
			return nil
		},
	)(ctx); err != nil {
		s.Fatalf("Failed to pin %d file(s) from Files App to Holding Space by drag and drop: %s", param, err)
	}

	lastFileLocation, err := uia.Location(ctx, holdingspace.FindPinnedFileChip().Name(listFiles[len(listFiles)-1]))
	if err != nil {
		s.Fatal("Failed to get holding space test file location: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Copy %d file(s) from Holding Space to Files App by drag and drop", param),
		uia.Exists(holdingspace.FindPinnedFilesBubble()),
		kb.AccelPressAction("Ctrl"),
		func(ctx context.Context) error {
			for _, fileName := range listFiles {
				if err := uia.LeftClick(holdingspace.FindPinnedFileChip().Name(fileName))(ctx); err != nil {
					s.Fatalf("Failed to select %q: %s", fileName, err)
				}
			}
			return err
		},
		kb.AccelReleaseAction("Ctrl"),
		mouse.Drag(tconn, lastFileLocation.CenterPoint(), fsappLocation.CenterPoint(), time.Second),
		uia.Gone(holdingspace.FindPinnedFilesBubble()),
		// uia.WaitUntilGone(holdingspace.FindTrayAccessibilityName()),
		func(ctx context.Context) error {
			for _, fileName := range listFiles {
				copiedFileName := fmt.Sprintf("%s (1)%s", fileName[:len(fileName)-len(filepath.Ext(fileName))], filepath.Ext(fileName))
				defer os.Remove(filepath.Join(myFilesPath, copiedFileName))
				if err := fsapp.WaitForFile(copiedFileName)(ctx); err != nil {
					s.Fatalf("Failed to verify that copied file %q exists: %s", copiedFileName, err)
				}
			}
			return nil
		},
	)(ctx); err != nil {
		s.Fatalf("Failed to copy %d file(s) from Holding Space to Files App by drag and drop: %s", param, err)
	}
}
