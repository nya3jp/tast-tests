// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package holdingspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type screenshotParams struct {
	testfunc func(*chrome.TestConn, *uiauto.Context, string) uiauto.Action
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

	// Ensure no screenshots exist prior to testing.
	screenshots, err := getScreenshots()
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
		takeScreenshot(&screenshotLocation),
	)(ctx); err != nil {
		s.Fatal("Failed to capture screenshot: ", err)
	}

	// Defer clean up of the screenshot file.
	defer os.Remove(screenshotLocation)

	// Trim screenshot filename.
	screenshotName := filepath.Base(screenshotLocation)

	// Perform additional parameterized testing.
	if err := s.Param().(screenshotParams).testfunc(tconn, ui, screenshotName)(ctx); err != nil {
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
		ui.WaitUntilGone(holdingspace.FindScreenCaptureView().Name(screenshotName)))(ctx); err != nil {
		s.Fatal("Failed to remove associated chips and views: ", err)
	}
}

// testScreenshotLaunch performs testing of launching a screenshot.
func testScreenshotLaunch(
	tconn *chrome.TestConn, ui *uiauto.Context, screenshotName string) uiauto.Action {
	return uiauto.Combine("launch screenshot",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),

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
	tconn *chrome.TestConn, ui *uiauto.Context, screenshotName string) uiauto.Action {
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
			// Left click the tray to open the bubble.
			ui.LeftClick(holdingspace.FindTray()),
			ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(screenshotName)),

			// Take the first additional screenshot.
			takeScreenshot(&screenshotLocations[0]),

			// Verify state after taking the first additional screenshot. Note that these assertions are wrapped
			// in a function to ensure that `takeScreenshot()` has updated `screenshotLocations` before evaluating.
			func(ctx context.Context) error {
				return uiauto.Combine("verify state after first additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(screenshotName)),
				)(ctx)
			},

			// Take the second additional screenshot.
			takeScreenshot(&screenshotLocations[1]),

			// Verify state after taking the second additional screenshot. Note that these assertions are wrapped
			// in a function to ensure that `takeScreenshot()` has updated `screenshotLocations` before evaluating.
			func(ctx context.Context) error {
				return uiauto.Combine("verify state after second additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[1]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(screenshotName)),
				)(ctx)
			},

			// Take the third additional screenshot.
			takeScreenshot(&screenshotLocations[2]),

			// Verify state after taking the third additional screenshot. Note that these assertions are wrapped
			// in a function to ensure that `takeScreenshot()` has updated `screenshotLocations` before evaluating.
			func(ctx context.Context) error {
				return uiauto.Combine("verify state after third additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[1]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[2]))),
					ui.WaitUntilGone(holdingspace.FindScreenCaptureView().Name(screenshotName)),
					ui.EnsureGoneFor(holdingspace.FindScreenCaptureView().Name(screenshotName), 5*time.Second),
				)(ctx)
			},

			// Remove the second additional screenshot and verify state. Note that removal/assertions are wrapped
			// in a function to ensure that `takeScreenshot()` has updated `screenshotLocations` before evaluating.
			func(ctx context.Context) error {
				os.Remove(screenshotLocations[1])
				return uiauto.Combine("verify state after removing second additional screenshot",
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[0]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocations[2]))),
					ui.WaitUntilExists(holdingspace.FindScreenCaptureView().Name(screenshotName)),
					ui.WaitUntilGone(holdingspace.FindScreenCaptureView().Name(screenshotLocations[1])),
					ui.EnsureGoneFor(holdingspace.FindScreenCaptureView().Name(screenshotLocations[1]), 5*time.Second),
				)(ctx)
			},
		)(ctx)
	}
}

// testScreenshotPinAndUnpin performs testing of pinning and unpinning a screenshot.
func testScreenshotPinAndUnpin(
	tconn *chrome.TestConn, ui *uiauto.Context, screenshotName string) uiauto.Action {
	return uiauto.Combine("pin and unpin screenshot",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),

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
	tconn *chrome.TestConn, ui *uiauto.Context, screenshotName string) uiauto.Action {
	return uiauto.Combine("remove screenshot",
		// Left click the tray to open the bubble.
		ui.LeftClick(holdingspace.FindTray()),

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

// getScreenshots returns the names of screenshot files present in the users
// downloads directory. Screenshot files are assumed to match a specific pattern.
func getScreenshots() ([]string, error) {
	return filepath.Glob(filepath.Join(filesapp.DownloadPath, "Screenshot*.png"))
}

// takeScreenshot returns an action which captures a fullscreen screenshot using
// the virtual keyboard. This should behave consistently across device form factors.
// NOTE: The location of the screenshot taken is asynchronously returned via `out`
// on successful execution of the returned action.
func takeScreenshot(out *string) uiauto.Action {
	return func(ctx context.Context) error {
		// Cache existing screenshots.
		screenshots, err := getScreenshots()
		if err != nil {
			return err
		}

		// Create virtual keyboard.
		keyboard, err := input.VirtualKeyboard(ctx)
		if err != nil {
			return err
		}
		defer keyboard.Close()

		// Take a screenshot.
		if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
			return err
		}

		// Wait for screenshot.
		return testing.Poll(ctx, func(ctx context.Context) error {
			newScreenshots, err := getScreenshots()
			if err != nil {
				return testing.PollBreak(err)
			}
			if reflect.DeepEqual(screenshots, newScreenshots) {
				return errors.New("waiting for screenshot")
			}
			sort.Strings(screenshots)
			for _, newScreenshot := range newScreenshots {
				if sort.SearchStrings(screenshots, newScreenshot) == len(screenshots) {
					*out = newScreenshot
					return nil
				}
			}
			return nil
		}, nil)
	}
}
