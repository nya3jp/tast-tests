// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"path/filepath"
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

func init() {
	testing.AddTest(&testing.Test{
		Func: FilesAppPinAndUnpinFolder,
		Desc: "Pin/Unpin a folder from Files app, verify that the folder's presence in holdingspace",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// FilesAppPinAndUnpinFolder verifies the folder's pin/unpin functionality from Files app.
func FilesAppPinAndUnpinFolder(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}
	defer func(ctx context.Context) {
		if err := files.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Files app")
		}
	}(cleanupCtx)

	if err := verifyTipInModes(ctx, tconn, true /* isTablet */); err != nil {
		s.Fatal("Failed to verify tip in tablet mode: ", err)
	}
	if err := verifyTipInModes(ctx, tconn, false /* isTablet */); err != nil {
		s.Fatal("Failed to verify tip in clamshell mode: ", err)
	}

	folderName := "New Folder"

	testing.ContextLog(ctx, "Create a folder")
	if err := createTarget(folderName); err != nil {
		s.Fatal("Failed to create a folder: ", err)
	}
	defer func(ctx context.Context) {
		if err := os.Remove(filepath.Join(filesapp.DownloadPath, folderName)); err != nil {
			testing.ContextLog(ctx, "Failed to remove the folder: ", err)
		}
	}(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder: ", err)
	}

	ui := uiauto.New(tconn)

	// To prevent the "Pin to shelf" option from being hidden, maxmize the Files app.
	maxBtn := nodewith.Name("Maximize").HasClass("FrameCaptionButton").Role(role.Button)
	if err := ui.IfSuccessThen(
		ui.WaitUntilExists(maxBtn),
		ui.LeftClick(maxBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to maximize the Files app: ", err)
	}

	tote := holdingspace.New(tconn)

	if err := uiauto.Combine("pin/unpin a folder and verify it's in tote or not",
		files.ClickContextMenuItem(folderName, "Pin to shelf"),
		tote.Expand(),
		ui.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(folderName)),
		tote.Collapse(),
		files.ClickContextMenuItem(folderName, "Unpin from shelf"),
		ui.WaitUntilGone(holdingspace.FindTray()), // After unpinning the targets from Files app, the holding space tray node should be gone, and the tote window is not able to expand.
	)(ctx); err != nil {
		s.Fatal("Failed to do the action: ", err)
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

	ui := uiauto.New(tconn)

	if err := ui.WaitUntilExists(nodewith.Role(role.StaticText).Name("Create a shortcut for your files"))(ctx); err != nil {
		return errors.Wrapf(err, "failed to find the educational tip when isTablet is %t", isTablet)
	}

	return nil
}

// createTarget creates the target if it is not in the directory "Downloads".
func createTarget(folderName string) error {
	if exists, err := isTargetExists(folderName); err != nil {
		return errors.Wrap(err, "failed to check if the folder exists")
	} else if exists {
		return nil
	}

	return os.Mkdir(filepath.Join(filesapp.DownloadPath, folderName), 0755)
}

// isTargetExists checks if the target is in the directory "Downloads" or not.
func isTargetExists(filePattern string) (bool, error) {
	files, err := filepath.Glob(filepath.Join(filesapp.DownloadPath, filePattern))
	if err != nil {
		return false, errors.Wrapf(err, "the pattern %q is malformed", filePattern)
	}

	if len(files) == 0 {
		return false, nil
	}

	return true, nil
}
