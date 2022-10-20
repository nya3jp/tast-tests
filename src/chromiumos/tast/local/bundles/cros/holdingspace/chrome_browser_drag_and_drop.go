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
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
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

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeBrowserDragAndDrop,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks dragging and dropping files in and out of Holding Space",
		Contacts: []string{
			"angelsan@chromium.org",
			"dmblack@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		// Params: []testing.Param{{
		// 	Name: "pause_and_resume",
		// 	Val: downloadParams{
		// 		testfunc:    testDownloadPauseAndResume,
		// 		browserType: browser.TypeAsh,
		// 	},
		// }, {
		// 	Name: "lacros_pause_and_resume",
		// 	Val: downloadParams{
		// 		testfunc:    testDownloadPauseAndResume,
		// 		browserType: browser.TypeLacros,
		// 	},
		// 	ExtraSoftwareDeps: []string{"lacros"},
		// }},
	})
}

// ChromeBrowserDragAndDrop tests the functionality of dragging and dropping single/multiple files from Holding Space to a Chrome Broswer window.
func ChromeBrowserDragAndDrop(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Reset the holding space and `MarkTimeOfFirstAdd` to make the `HoldingSpaceTrayIcon`
	// show.
	if err := holdingspace.ResetHoldingSpace(ctx, tconn,
		holdingspace.ResetHoldingSpaceOptions{MarkTimeOfFirstAdd: true}); err != nil {
		s.Fatal("Failed to reset holding space: ", err)
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	// Ensure no screenshots exist prior to testing.
	screenshots, err := holdingspace.GetScreenshots(downloadsPath)
	if err != nil || len(screenshots) != 0 {
		s.Fatal("Failed to verify no screenshots exist: ", err)
	}

	screenshotLocations := make([]string, 3)

	uia := uiauto.New(tconn)
	screenshotLocations[0], err = holdingspace.TakeScreenshot(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to capture first screenshot: ", err)
	}
	if err = uiauto.Combine("verify state after first screenshot",
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(filepath.Base(screenshotLocations[0]))),
	)(ctx); err != nil {
		s.Fatal("Failed to verify state after first screenshot: ", err)
	}

	screenshotLocations[1], err = holdingspace.TakeScreenshot(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to capture second screenshot: ", err)
	}
	if err = uiauto.Combine("verify state after second screenshot",
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(filepath.Base(screenshotLocations[0]))),
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(filepath.Base(screenshotLocations[1]))),
	)(ctx); err != nil {
		s.Fatal("Failed to verify state after second screenshot: ", err)
	}

	screenshotLocations[2], err = holdingspace.TakeScreenshot(ctx, downloadsPath)
	if err != nil {
		s.Fatal("Failed to capture third screenshot: ", err)
	}
	if err = uiauto.Combine("verify state after third screenshot",
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(filepath.Base(screenshotLocations[0]))),
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(filepath.Base(screenshotLocations[1]))),
		uia.WaitUntilExists(holdingspace.FindScreenCaptureView().
			Name(filepath.Base(screenshotLocations[2]))),
	)(ctx); err != nil {
		s.Fatal("Failed to verify state after third screenshot: ", err)
	}

	// Defer removal of screenshots taken during this test.
	defer func() {
		for _, screenshotLocation := range screenshotLocations {
			if len(screenshotLocation) > 0 {
				os.Remove(screenshotLocation)
			}
		}
	}()

	// Trim screenshot filename.
	screenshotName0 := filepath.Base(screenshotLocations[0])
	screenshotName1 := filepath.Base(screenshotLocations[1])
	screenshotName2 := filepath.Base(screenshotLocations[2])

	// Open chrome browser.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to launch chrome app: ", err)
	}
	defer apps.Close(ctx, tconn, apps.Chrome.ID)

	chromeWindowFinder := nodewith.NameContaining("Google Chrome").Role(role.Window)
	chromeLocation, err := uia.Location(ctx, chromeWindowFinder.HasClass("BrowserRootView"))
	screenShotLocation, err := uia.Location(ctx, holdingspace.FindScreenCaptureView().Name(screenshotName0))
	if err != nil {
		s.Fatal("Failed to get holding space test file location: ", err)
	}

	tray := holdingspace.FindTray()

	// Drag and drop a single file from Holding Space to Chrome browser.
	uia.LeftClick(tray)(ctx)
	mouse.Drag(tconn, screenShotLocation.CenterPoint(), chromeLocation.CenterPoint(), time.Second)(ctx)
	err = uia.Gone(holdingspace.FindChip())(ctx)
	if err != nil {
		s.Fatal("Failed to automatically close Holding Space by dragging item out of Holding Space: ", err)
	}
	err = uia.WaitUntilExists(nodewith.Role(role.Tab).Name(screenshotName0))(ctx)
	if err != nil {
		s.Fatalf("Failed to open file %s in Chrome browser: %s", screenshotName0, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Drag and drop multiple files from Holding Space to Chrome browser.
	uia.LeftClick(tray)(ctx)
	if err := kb.AccelPress(ctx, "Ctrl"); err != nil {
		s.Fatal("Failed to press Ctrl: ", err)
	}
	for _, screenshotLocation := range screenshotLocations {
		if err := uia.LeftClick(holdingspace.FindScreenCaptureView().Name(filepath.Base(screenshotLocation)))(ctx); err != nil {
			s.Fatalf("Failed to select %s : %s", filepath.Base(screenshotLocation), err)
		}
	}
	if err := kb.AccelRelease(ctx, "Ctrl"); err != nil {
		s.Fatal("Failed to release Ctrl: ", err)
	}
	screenshotLocation2, err := uia.Location(ctx, holdingspace.FindScreenCaptureView().Name(screenshotName2))
	if err != nil {
		s.Fatal("Failed to get holding space file location: ", err)
	}
	err = mouse.Drag(tconn, screenshotLocation2.CenterPoint(), chromeLocation.CenterPoint(), time.Second)(ctx)
	if err != nil {
		s.Fatalf("Failed to drag and drop multiple files %v from Holding Space to Chrome Browser: %s", screenshotLocations, err)
	}
	uia.Gone(holdingspace.FindChip())(ctx)
	if err != nil {
		s.Fatal("Failed to automatically close Holding Space by dragging multiple items out of Holding Space: ", err)
	}
	err = uia.WaitUntilExists(nodewith.Role(role.Tab).Name(screenshotName1))(ctx)
	if err != nil {
		s.Fatalf("Failed to open file %s in Chrome browser: %s", screenshotName1, err)
	}
	err = uia.WaitUntilExists(nodewith.Role(role.Tab).Name(screenshotName2))(ctx)
	if err != nil {
		s.Fatalf("Failed to open file %s in Chrome browser: %s", screenshotName2, err)
	}
}
