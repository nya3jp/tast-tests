// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/wmp"
	"chromiumos/tast/testing"
)

type screenshotParams struct {
	testfunc func(context.Context, *testing.State, *chrome.TestConn, *uiauto.Context, string)
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Screenshot,
		Desc: "Verifies screenshot behavior in holding space",
		Contacts: []string{
			"dmblack@google.com",
			"tote-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "launch",
			Val: screenshotParams{
				testfunc: testScreenshotLaunch,
			},
		}, {
			Name: "pin_and_unpin",
			Val: screenshotParams{
				testfunc: testScreenshotPinAndUnpin,
			},
		}},
	})
}

// Screenshot verifies screenshot behavior in holding space. It is expected that
// capturing a screenshot will result in an item being added to holding space
// from which the user can launch/pin/unpin the screenshot.
func Screenshot(ctx context.Context, s *testing.State) {
	// Connect to a fresh Chrome instance to ensure holding space first-run state.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	// Ensure no screenshots exist prior to testing.
	screenshots, err := getScreenshots()
	if err != nil || len(screenshots) != 0 {
		s.Fatal("Failed to verify no screenshots exist: ", err)
	}

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("capture screenshot",
		// Prior to capturing a screenshot, holding space should be empty and
		// therefore the holding space tray node should not exist.
		ui.EnsureGoneFor(holdingspace.FindTray(), 5*time.Second),

		// Capture a fullscreen screenshot using the capture mode feature in quick
		// settings. Note that this will result in multiple user interactions with
		// system UI as part of the screen capture user flow.
		wmp.CaptureScreenshot(tconn, wmp.FullScreen),
	)(ctx); err != nil {
		s.Fatal("Failed to capture screenshot: ", err)
	}

	// Ensure a screenshot has been captured.
	screenshots, err = getScreenshots()
	if err != nil || len(screenshots) != 1 {
		s.Fatal("Failed to capture screenshot: ", err)
	}

	// Defer clean up of the screenshot file in the event that this test fails early
	// without having explicitly removed the file itself.
	filelocation := screenshots[0]
	defer os.Remove(filelocation)

	// Trim screenshot filename.
	filename := filepath.Base(filelocation)

	// Perform additional parameterized testing.
	s.Param().(screenshotParams).testfunc(ctx, s, tconn, ui, filename)

	// Explicitly remove the screenshot file. Note that this will result in any
	// associated holding space items being removed, triggering a change in UI state.
	if err := os.Remove(filelocation); err != nil {
		s.Fatal("Failed to remove screenshot: ", err)
	}

	// Ensure all holding space chips and views associated with the underlying
	// screenshot are removed when the backing file is removed.
	if err := uiauto.Combine("remove associated chips and views",
		ui.WaitUntilGone(holdingspace.FindChip().Name(filename)),
		ui.WaitUntilGone(holdingspace.FindScreenCaptureView().Name(filename)))(ctx); err != nil {
		s.Fatal("Failed to remove associated chips and views: ", err)
	}
}

// testScreenshotLaunch performs testing of launching a screenshot.
func testScreenshotLaunch(
	ctx context.Context, s *testing.State, tconn *chrome.TestConn,
	ui *uiauto.Context, filename string) {
	if err := uiauto.Combine("launch screenshot",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),

		// Double click the screenshot view. This will wait until the screenshot
		// view exists and stabilizes before performing the double click.
		ui.DoubleClick(holdingspace.FindScreenCaptureView().Name(filename)),

		// Ensure the Gallery app is launched.
		func(ctx context.Context) error {
			return ash.WaitForApp(ctx, tconn, apps.Gallery.ID, 5*time.Second)
		},

		// Ensure that the screenshot file is opened in the Gallery app.
		ui.WaitUntilExists(nodewith.
			Ancestor(nodewith.NameStartingWith(apps.Gallery.Name).HasClass("BrowserFrame")).
			Role(role.Image).Name(filename)))(ctx); err != nil {
		s.Fatal("Failed to launch screenshot: ", err)
	}
}

// testScreenshotPinAndUnpin performs testing of pinning and unpinning a screenshot.
func testScreenshotPinAndUnpin(
	ctx context.Context, s *testing.State, tconn *chrome.TestConn,
	ui *uiauto.Context, filename string) {
	if err := uiauto.Combine("pin and unpin screenshot",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),

		// Right click the screenshot view. This will wait until the screenshot view
		// exists and stabilizes before showing the context menu.
		ui.RightClick(holdingspace.FindScreenCaptureView().Name(filename)),

		// Left click the "Pin" context menu item. This will result in the creation
		// of a pinned holding space item backed by the same screenshot.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Pin")),

		// Ensure that a chip is added to holding space for the pinned item.
		ui.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(filename)),

		// Right click the screenshot view to show the context menu.
		ui.RightClick(holdingspace.FindScreenCaptureView().Name(filename)),

		// Left click the "Unpin" context menu item. This will result in the pinned
		// holding space item backed by the same screenshot being destroyed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Unpin")),

		// Ensure that the pinned file chip is removed from holding space.
		ui.WaitUntilGone(holdingspace.FindPinnedFileChip().Name(filename)),
		ui.EnsureGoneFor(holdingspace.FindPinnedFileChip().Name(filename), 5*time.Second),

		// Ensure that the screenshot view continues to exist despite the pinned
		// holding space item associated with the same screenshot file being destroyed.
		ui.Exists(holdingspace.FindScreenCaptureView().Name(filename)),
	)(ctx); err != nil {
		s.Fatal("Failed to pin and unpin screenshot: ", err)
	}
}

// getScreenshots returns the names of screenshot files present in the users
// downloads directory. Screenshot files are assumed to match a specific pattern.
func getScreenshots() ([]string, error) {
	return filepath.Glob(filepath.Join(filesapp.DownloadPath, "Screenshot*.png"))
}
