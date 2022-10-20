// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/holdingspace"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type dragDropParams struct {
	testfunc    func(context.Context, *chrome.TestConn, *uiauto.Context, *browser.Browser, []string) error
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeBrowserDragAndDrop,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks dragging and dropping files out of Holding Space",
		Contacts: []string{
			"angelsan@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name: "single_drag_and_drop",
			Val: dragDropParams{
				testfunc:    testSingleDragAndDrop,
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "multiple_drag_and_drop",
			Val: dragDropParams{
				testfunc:    testMultipleDragAndDrop,
				browserType: browser.TypeAsh,
			},
		}, {
			Name: "lacros_single_drag_and_drop",
			Val: dragDropParams{
				testfunc:    testSingleDragAndDrop,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "lacros_multiple_drag_and_drop",
			Val: dragDropParams{
				testfunc:    testMultipleDragAndDrop,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

// ChromeBrowserDragAndDrop tests the functionality of dragging and dropping single/multiple files from Holding Space to a Chrome Browser window.
func ChromeBrowserDragAndDrop(ctx context.Context, s *testing.State) {
	params := s.Param().(dragDropParams)
	bt := params.browserType
	// cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Connect to a fresh ash-chrome instance (cr) to ensure holding space first-run state,
	// also get a browser instance (br) for browser functionality in common.
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig())
	if err != nil {
		s.Fatalf("Failed to connect to %v browser: %v", bt, err)
	}
	defer cr.Close(ctx)
	defer closeBrowser(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	// Reset the holding space.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn, holdingspace.ResetHoldingSpaceOptions{}); err != nil {
		s.Fatal("Failed to reset holding space: ", err)
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	screenshotLocations := make([]string, 3)

	uia := uiauto.New(tconn)

	screenshotLocations[0], err = holdingspace.TakeScreenshot(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to capture first screenshot: ", err)
	}
	defer os.Remove(screenshotLocations[0])

	screenshotLocations[1], err = holdingspace.TakeScreenshot(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to capture second screenshot: ", err)
	}
	defer os.Remove(screenshotLocations[1])

	screenshotLocations[2], err = holdingspace.TakeScreenshot(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to capture third screenshot: ", err)
	}
	defer os.Remove(screenshotLocations[2])

	var screenshotName1 = filepath.Base(screenshotLocations[0])
	var screenshotName2 = filepath.Base(screenshotLocations[1])
	var screenshotName3 = filepath.Base(screenshotLocations[2])

	if err = uiauto.Combine("verify state after third screenshot",
		uia.LeftClick(holdingspace.FindTray()),
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(screenshotName1)),
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(screenshotName2)),
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(screenshotName3)),
	)(ctx); err != nil {
		s.Fatal("Failed to verify state after third screenshot: ", err)
	}

	conn, err := br.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	defer conn.Close()

	// Perform additional parameterized testing.
	if err := params.testfunc(ctx, tconn, uia, br, screenshotLocations); err != nil {
		s.Fatal("Fail to perform parameterized testing: ", err)
	}
}

// testSingleDragAndDrop performs testing of dragging and dropping a single item out of holdingspace.
func testSingleDragAndDrop(ctx context.Context, tconn *chrome.TestConn, uia *uiauto.Context, br *browser.Browser, screenshotLocations []string) error {
	chromeWindowFinder := nodewith.NameContaining("Google Chrome").Role(role.Window).HasClass("BrowserRootView")
	chromeLocation, err := uia.Location(ctx, chromeWindowFinder)

	uia.LeftClick(holdingspace.FindTray())(ctx)
	var screenshotName1 = filepath.Base(screenshotLocations[0])
	screenShotLocation, err := uia.Location(ctx, holdingspace.FindScreenCaptureView().Name(screenshotName1))
	if err != nil {
		return errors.Wrap(err, "failed to get holding space test file location")
	}
	// Drag and drop a single file from Holding Space to Chrome browser.
	mouse.Drag(tconn, screenShotLocation.CenterPoint(), chromeLocation.CenterPoint(), time.Second)(ctx)
	if err = uia.Gone(holdingspace.FindChip())(ctx); err != nil {
		return errors.Wrap(err, "failed to automatically close Holding Space by dragging item out of Holding Space")
	}
	if err = uia.WaitUntilExists(nodewith.Role(role.Tab).Name(screenshotName1))(ctx); err != nil {
		return errors.Wrap(err, "failed to open first screenshot in Chrome browser")
	}
	return nil
}

// testMultipleDragAndDrop performs testing of dragging and dropping multiple items out of holdingspace.
func testMultipleDragAndDrop(ctx context.Context, tconn *chrome.TestConn, uia *uiauto.Context, br *browser.Browser, screenshotLocations []string) error {
	chromeWindowFinder := nodewith.NameContaining("Google Chrome").Role(role.Window).HasClass("BrowserRootView")
	chromeLocation, err := uia.Location(ctx, chromeWindowFinder)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()
	// Drag and drop second and third screenshots from Holding Space to Chrome browser.
	if err = uia.LeftClick(holdingspace.FindTray())(ctx); err != nil {
		return errors.Wrap(err, "failed to left click on tray")
	}
	if err := kb.AccelPress(ctx, "Ctrl"); err != nil {
		return errors.Wrap(err, "failed to press Ctrl")
	}
	for _, screenshotLocation := range screenshotLocations[1:] {
		if err := uia.LeftClick(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocation)))(ctx); err != nil {
			return errors.Errorf("failed to select %s : %s", filepath.Base(screenshotLocation), err)
		}
	}
	if err := kb.AccelRelease(ctx, "Ctrl"); err != nil {
		return errors.Wrap(err, "failed to release Ctrl")
	}
	var screenshotName3 = filepath.Base(screenshotLocations[2])
	screenshotLocation3, err := uia.Location(ctx, holdingspace.FindScreenCaptureView().Name(screenshotName3))
	if err != nil {
		return errors.Wrap(err, "failed to get holding space file location")
	}
	if err = mouse.Drag(tconn, screenshotLocation3.CenterPoint(), chromeLocation.CenterPoint(), time.Second)(ctx); err != nil {
		return errors.Errorf("failed to drag and drop multiple files %v from Holding Space to Chrome Browser: %s", screenshotLocations[1:], err)
	}
	if err = uia.Gone(holdingspace.FindChip())(ctx); err != nil {
		return errors.Wrap(err, "failed to automatically close Holding Space by dragging multiple items out of Holding Space")
	}
	if err = uia.WaitUntilExists(nodewith.Role(role.Tab).Name(screenshotName3))(ctx); err != nil {
		return errors.Wrap(err, "failed to open third screenshot in Chrome browser")
	}
	return nil
}
