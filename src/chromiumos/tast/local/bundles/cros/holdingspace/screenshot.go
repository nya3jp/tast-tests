// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

type screenshotParams struct {
	testfunc func(*chrome.TestConn, *uiauto.Context, string, string) uiauto.Action
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Screenshot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies screenshot behavior in holding space",
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
			Name: "overflow",
			Val: screenshotParams{
				testfunc: testScreenshotOverflow,
			},
		}, {
			Name: "pin_and_unpin",
			Val: screenshotParams{
				testfunc: testScreenshotPinAndUnpin,
			},
		}, {
			Name: "remove",
			Val: screenshotParams{
				testfunc: testScreenshotRemove,
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

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	// Ensure no screenshots exist prior to testing.
	screenshots, err := holdingspace.GetScreenshots(downloadsPath)
	if err != nil || len(screenshots) != 0 {
		s.Fatal("Failed to verify no screenshots exist: ", err)
	}

	ui := uiauto.New(tconn)

	var screenshotLocation string
	if err := uiauto.Combine("capture screenshot",
		// Prior to capturing a screenshot, holding space should be empty and
		// therefore the holding space tray node should not exist.
		ui.EnsureGoneFor(holdingspace.FindTray(), 5*time.Second),

		// Capture a fullscreen screenshot using the virtual keyboard. This should
		// behave consistently across device form factors.
		func(ctx context.Context) error {
			screenshotLocation, err = holdingspace.TakeScreenshot(ctx, downloadsPath)
			return err
		},
	)(ctx); err != nil {
		s.Fatal("Failed to capture screenshot: ", err)
	}

	// Defer clean up of the screenshot file.
	defer os.Remove(screenshotLocation)

	// Trim screenshot filename.
	screenshotName := filepath.Base(screenshotLocation)

	if err := uiauto.Combine("open bubble and confirm initial state",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),

		// The pinned files section should contain an educational prompt and chip
		// informing the user that they can pin a file from the Files app.
		ui.WaitUntilExists(holdingspace.FindPinnedFilesSectionFilesAppPrompt()),
		ui.WaitUntilExists(holdingspace.FindPinnedFilesSectionFilesAppChip()),
	)(ctx); err != nil {
		s.Fatal("Failed to open bubble and confirm initial state: ", err)
	}

	// Perform additional parameterized testing.
	if err := s.Param().(screenshotParams).testfunc(tconn, ui, downloadsPath, screenshotName)(ctx); err != nil {
		s.Fatal("Failed to perform parameterized testing: ", err)
	}

	// Remove the screenshot file. Note that this will result in any associated
	// associated holding space items being removed.
	if err := os.Remove(screenshotLocation); err != nil {
		s.Fatal("Failed to remove screenshot: ", err)
	}

	// Ensure all holding space chips and views associated with the underlying
	// screenshot are removed when the backing file is removed.
	if err := uiauto.Combine("remove associated chips and views",
		ui.WaitUntilGone(holdingspace.FindChip().Name(screenshotName)),
		ui.WaitUntilGone(holdingspace.FindScreenCaptureView().Name(screenshotName)),
	)(ctx); err != nil {
		s.Fatal("Failed to remove associated chips and views: ", err)
	}
}

// testScreenshotLaunch performs testing of launching a screenshot.
func testScreenshotLaunch(
	tconn *chrome.TestConn, ui *uiauto.Context, downloadsPath, screenshotName string) uiauto.Action {
	return uiauto.Combine("launch screenshot",
		// Double click the screenshot view. This will wait until the screenshot
		// view exists and stabilizes before performing the double click.
		ui.DoubleClick(holdingspace.FindScreenCaptureView().Name(screenshotName)),

		// Ensure the Gallery app is launched.
		func(ctx context.Context) error {
			return ash.WaitForApp(ctx, tconn, apps.Gallery.ID, 5*time.Second)
		},

		// Ensure that the screenshot file is opened in the Gallery app.
		ui.WaitUntilExists(nodewith.
			Ancestor(nodewith.NameStartingWith(apps.Gallery.Name).HasClass("BrowserFrame")).
			Role(role.Image).Name(screenshotName)),
	)
}

// testScreenshotOverflow performs testing of screenshot overflow behavior.
func testScreenshotOverflow(
	tconn *chrome.TestConn, ui *uiauto.Context, downloadsPath, screenshotName string) uiauto.Action {
	return func(ctx context.Context) error {
		// Holding space UI can accommodate up to three screenshots at a time. This
		// test will take three additional screenshots to the one already taken in
		// order to force overflow behavior.
		screenshotLocations := make([]string, 3)

		// Defer removal of screenshots taken during this test.
		defer func() {
			for _, screenshotLocation := range screenshotLocations {
				if len(screenshotLocation) > 0 {
					os.Remove(screenshotLocation)
				}
			}
		}()

		return uiauto.Combine("overflow screenshots",
			// Take the first additional screenshot and verify state.
			func(ctx context.Context) error {
				var err error
				if screenshotLocations[0], err = holdingspace.TakeScreenshot(ctx, downloadsPath); err != nil {
					return err
				}
				return uiauto.Combine(
					"verify state after first additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(screenshotName)),
				)(ctx)
			},

			// Take the second additional screenshot and verify state.
			func(ctx context.Context) error {
				var err error
				if screenshotLocations[1], err = holdingspace.TakeScreenshot(ctx, downloadsPath); err != nil {
					return err
				}
				return uiauto.Combine(
					"verify state after second additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[1]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(screenshotName)),
				)(ctx)
			},

			// Take the third additional screenshot and verify state.
			func(ctx context.Context) error {
				var err error
				if screenshotLocations[2], err = holdingspace.TakeScreenshot(ctx, downloadsPath); err != nil {
					return err
				}
				return uiauto.Combine(
					"verify state after third additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[1]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[2]))),
					ui.WaitUntilGone(holdingspace.FindScreenCaptureView().
						Name(screenshotName)),
					ui.EnsureGoneFor(holdingspace.FindScreenCaptureView().
						Name(screenshotName), 5*time.Second),
				)(ctx)
			},

			// Remove the second additional screenshot and verify state.
			func(ctx context.Context) error {
				os.Remove(screenshotLocations[1])
				return uiauto.Combine(
					"verify state after removing second additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(filepath.Base(screenshotLocations[2]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().
						Name(screenshotName)),
					ui.WaitUntilGone(holdingspace.FindScreenCaptureView().
						Name(screenshotLocations[1])),
					ui.EnsureGoneFor(holdingspace.FindScreenCaptureView().
						Name(screenshotLocations[1]), 5*time.Second),
				)(ctx)
			},
		)(ctx)
	}
}

// testScreenshotPinAndUnpin performs testing of pinning and unpinning a screenshot.
func testScreenshotPinAndUnpin(
	tconn *chrome.TestConn, ui *uiauto.Context, downloadsPath, screenshotName string) uiauto.Action {
	return uiauto.Combine("pin and unpin screenshot",
		// Right click the screenshot view. This will wait until the screenshot view
		// exists and stabilizes before showing the context menu.
		ui.RightClick(holdingspace.FindScreenCaptureView().Name(screenshotName)),

		// Left click the "Pin" context menu item. This will result in the creation
		// of a pinned holding space item backed by the same screenshot.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Pin")),

		// Ensure that a chip is added to holding space for the pinned item.
		ui.WaitUntilExists(holdingspace.FindPinnedFileChip().Name(screenshotName)),

		// Right click the screenshot view to show the context menu.
		ui.RightClick(holdingspace.FindScreenCaptureView().Name(screenshotName)),

		// Left click the "Unpin" context menu item. This will result in the pinned
		// holding space item backed by the same screenshot being destroyed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Unpin")),

		// Ensure that the pinned file chip is removed from holding space.
		ui.WaitUntilGone(holdingspace.FindPinnedFileChip().Name(screenshotName)),
		ui.EnsureGoneFor(holdingspace.FindPinnedFileChip().Name(screenshotName), 5*time.Second),

		// Ensure that the screenshot view continues to exist despite the pinned
		// holding space item associated with the same screenshot file being destroyed.
		ui.Exists(holdingspace.FindScreenCaptureView().Name(screenshotName)),
	)
}

// testScreenshotRemove performs testing of removing a screenshot.
func testScreenshotRemove(
	tconn *chrome.TestConn, ui *uiauto.Context, downloadsPath, screenshotName string) uiauto.Action {
	return uiauto.Combine("remove screenshot",
		// Right click the screenshot view. This will wait until the screenshot view
		// exists and stabilizes before showing the context menu.
		ui.RightClick(holdingspace.FindScreenCaptureView().Name(screenshotName)),

		// Left click the "Remove" context menu item. Note that this will result in
		// the holding space item associated with the underlying screenshot being
		// removed and the context menu being closed.
		ui.LeftClick(holdingspace.FindContextMenuItem().Name("Remove")),

		// Ensure that the screenshot view is removed.
		ui.WaitUntilGone(holdingspace.FindScreenCaptureView().Name(screenshotName)),
		ui.EnsureGoneFor(holdingspace.FindScreenCaptureView().Name(screenshotName), 5*time.Second),
	)
}
