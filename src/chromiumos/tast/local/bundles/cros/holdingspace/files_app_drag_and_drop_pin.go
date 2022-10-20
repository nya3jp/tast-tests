// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"fmt"
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
		Params: []testing.Param{{
			Name: "single_file",
			Val:/*count=*/ 1,
		}, {
			Name: "multiple_files",
			Val:/*count=*/ 2,
		}},
	})
}

// FilesAppDragAndDropPin tests the functionality of pinning files to Holding Space by
// dragging and dropping single/multiple files to/from the Files app.
func FilesAppDragAndDropPin(ctx context.Context, s *testing.State) {
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

	uia := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Create number of files needed.
	var listFiles []string
	for i := 0; i < param; i++ {
		testFile := fmt.Sprintf("test%d.txt", i)
		testFilePath := filepath.Join(myFilesPath, testFile)
		if err := ioutil.WriteFile(testFilePath, []byte("Per aspera, ad astra"), 0644); err != nil {
			s.Fatalf("Failed to create file %q failed: %s", testFilePath, err)
		}
		defer os.Remove(testFilePath)
		listFiles = append(listFiles, testFile)
	}

	tray := holdingspace.FindTray()
	trayLocation, err := uia.Location(ctx, tray)
	if err != nil {
		s.Fatal("Failed to find holding space tray location: ", err)
	}

	fsappLocation, err := uia.Location(ctx, nodewith.Role(role.ListBox))
	if err != nil {
		s.Fatal("Failed to get Files App location: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Pin %d file(s) from Files App to Holding Space by drag and drop", param),
		fsapp.DragAndDropFile(strings.Join(listFiles, " "), trayLocation.CenterPoint(), kb),
		uia.LeftClick(tray),
		func(ctx context.Context) error {
			for _, fileName := range listFiles {
				if err := uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(fileName))(ctx); err != nil {
					s.Fatalf("Failed to verify that %s exists in holding space: %s", fileName, err)
				}
			}
			return nil
		},
	)(ctx); err != nil {
		s.Fatalf("Failed to pin %d items %v from Files App to Holding Space by drag and drop: %s", param, listFiles, err)
	}

	lastFileLocation, err := uia.Location(ctx, holdingspace.FindPinnedFileChip().Name(listFiles[len(listFiles)-1]))
	if err != nil {
		s.Fatal("Failed to get holding space test file location: ", err)
	}

	if err := uiauto.Combine(fmt.Sprintf("Copy %d file(s) from Holding Space to Files App by drag and drop", param),
		kb.AccelPressAction("Ctrl"),
		func(ctx context.Context) error {
			for _, fileName := range listFiles {
				if err := uia.LeftClick(holdingspace.FindPinnedFileChip().Name(fileName))(ctx); err != nil {
					s.Fatalf("Failed to select %s : %s", fileName, err)
				}
			}
			return err
		},
		kb.AccelReleaseAction("Ctrl"),
		mouse.Drag(tconn, lastFileLocation.CenterPoint(), fsappLocation.CenterPoint(), time.Second),
		uia.Gone(holdingspace.FindTrayBubble()),
		func(ctx context.Context) error {
			for _, fileName := range listFiles {
				var copiedFileName = fmt.Sprintf("%s (1).txt", fileName[:len(fileName)-len(filepath.Ext(fileName))])
				if err := fsapp.SelectFile(copiedFileName)(ctx); err != nil {
					s.Fatalf("Failed to select copied file %s: %s", copiedFileName, err)
				}
				if err := fsapp.FileExists(copiedFileName)(ctx); err != nil {
					s.Fatalf("Failed to verify that copied file %s exists: %s", copiedFileName, err)
				}
				defer os.Remove(filepath.Join(copiedFileName))
			}
			return nil
		},
	)(ctx); err != nil {
		s.Fatalf("Failed to copy %d file(s) from Holding Space to Files App by drag and drop: %s", param, err)
	}
}
