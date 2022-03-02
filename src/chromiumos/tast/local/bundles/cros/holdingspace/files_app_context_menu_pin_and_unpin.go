// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const (
	fileName   = "test.txt"
	folderName = "test"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesAppContextMenuPinAndUnpin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that pinning to Holding Space from the Files app works",
		Contacts: []string{
			"angusmclean@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Val: fileName,
			},
			{
				Name: "folder",
				Val:  folderName,
			},
		},
	})
}

// FilesAppContextMenuPinAndUnpin tests the functionality of pinning files to
// Holding Space from the Files app.
func FilesAppContextMenuPinAndUnpin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in to Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	fsapp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not open filesapp: ", err)
	}

	if err := verifyTipInModes(ctx, tconn, true /* isTablet */); err != nil {
		s.Fatal("Failed to verify tip in tablet mode: ", err)
	}
	if err := verifyTipInModes(ctx, tconn, false /* isTablet */); err != nil {
		s.Fatal("Failed to verify tip in clamshell mode: ", err)
	}

	targetName := s.Param().(string)
	targetPath := filepath.Join(filesapp.MyFilesPath, targetName)

	if err := createTarget(targetPath, strings.HasSuffix(targetName, "txt") /* isFile */); err != nil {
		s.Fatalf("Failed to create %q: %v", targetPath, err)
	}
	defer os.Remove(targetPath)

	uia := uiauto.New(tconn)

	// To prevent the "Pin to shelf" option from being hidden, maximize the Files app.
	if err := uia.LeftClick(nodewith.Name("Maximize").HasClass("FrameCaptionButton").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to maximize the Files app: ", err)
	}

	// Pin and unpin the item using the context menu in the files app. Unpinning
	// the item should make the Tray disappear, since nothing else is pinned.
	if err := uiauto.Combine("pin and unpin item from shelf via Files app context menu",
		fsapp.ClickContextMenuItem(targetName, "Pin to shelf"),
		uia.LeftClick(holdingspace.FindTray()),
		uia.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(targetName)),
		fsapp.ClickContextMenuItem(targetName, "Unpin from shelf"),
		uia.WaitUntilGone(holdingspace.FindTray()),
		uia.EnsureGoneFor(holdingspace.FindTray(), time.Second),
	)(ctx); err != nil {
		s.Fatalf("Failed to pin and unpin item %q: %v", targetName, err)
	}
}

// verifyTipInModes verifies that Files app displays the educational tip "Create a shortcut for your files" both in clamshell and tablet mode.
func verifyTipInModes(ctx context.Context, tconn *chrome.TestConn, isTablet bool) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTablet)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure tablet mode enabled is %t", isTablet)
	}
	defer cleanup(cleanupCtx)

	return uiauto.New(tconn).WaitUntilExists(nodewith.Name("Create a shortcut for your files").Role(role.StaticText))(ctx)
}

// createTarget creates the target if it is not in the specific file path.
func createTarget(targetPath string, isFile bool) error {
	if isFile {
		// Create our file, with appropriate permissions so we can delete later.
		return ioutil.WriteFile(targetPath, []byte("Per aspera, ad astra"), 0644)
	}

	return os.Mkdir(targetPath, 0755)
}
