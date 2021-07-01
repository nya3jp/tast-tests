// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type rearrangmentTestType string

const (
	chromeAppTest rearrangmentTestType = "ChromeAppTest" // Verify the rearrangement behavior on a Chrome app.
	arcAppTest    rearrangmentTestType = "ArcAppTest"    // Verify the rearrangement behavior on an Arc app.
	fileAppTest   rearrangmentTestType = "FileAppTest"   // Verify the rearrangement behavior on the File app.
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AppRearrangement,
		Desc: "Tests the rearrangement of shelf app icons",
		Contacts: []string{
			"andrewxu@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{
			{
				Name:              "rearrange_arc_apps",
				ExtraAttr:         []string{"group:arc-functional"},
				ExtraSoftwareDeps: []string{"android_p"},
				Val:               arcAppTest,
			},
			{
				Name:    "rearrange_chrome_apps",
				Val:     chromeAppTest,
				Fixture: "install2Apps",
			},
			{
				Name:    "rearrange_file_app",
				Val:     fileAppTest,
				Fixture: "chromeLoggedIn",
			},
		},
	})
}

// AppRearrangement tests the basic features of hotseat.
func AppRearrangement(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome

	testType := s.Param().(rearrangmentTestType)
	switch testType {
	case arcAppTest:
		var err error
		flags := arc.DisableSyncFlags()
		flags = append(flags, "--disable-sync")
		cr, err = chrome.New(ctx,
			chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			chrome.ARCSupported(),
			chrome.ExtraArgs(flags...))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)
	case chromeAppTest:
		var err error
		cr, err = chrome.New(ctx, s.FixtValue().([]chrome.Option)...)

		if err != nil {
			s.Fatal("Failed to start chrome: ", err)
		}
		defer cr.Close(ctx)
	case fileAppTest:
		cr = s.FixtValue().(*chrome.Chrome)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Wait for ARC++ default apps to get installed if ARC is enabled.
	if testType == arcAppTest {
		if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayStore.ID, ctxutil.MaxTimeout); err != nil {
			s.Fatalf("Failed to wait for PlayStore(%s) to be installed: %v", apps.PlayStore.ID, err)
		}
	}

	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}

	var itemsToUnpin []string
	for _, item := range items {
		if item.AppID != apps.Chrome.ID {
			itemsToUnpin = append(itemsToUnpin, item.AppID)
		}
	}

	// Unpin all apps except for the browser.
	cleanUp, err := ash.UnpinApps(ctx, tconn, itemsToUnpin)
	if err != nil {
		s.Fatalf("Failed to unpin apps %v: %v", itemsToUnpin, err)
	}
	defer cleanUp(ctx)

	if err := ash.PinApp(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to pin Settings to shelf")
	}

	ac := uiauto.New(tconn)

	var targetAppName string
	var targetAppID string
	switch testType {
	case arcAppTest:
		targetAppID = apps.PlayStore.ID
		targetAppName = apps.PlayStore.Name
	case chromeAppTest:
		installedApps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the installed apps: ", err)
		}
		for _, app := range installedApps {
			if strings.Contains(app.Name, "fake") {
				targetAppID = app.AppID
				targetAppName = app.Name
				break
			}
		}
		if targetAppID == "" {
			s.Fatal("Failed to find the fake app")
		}
	case fileAppTest:
		targetAppID = apps.Files.ID
		targetAppName = apps.Files.Name
	}

	if err := ash.PinApp(ctx, tconn, targetAppID); err != nil {
		s.Fatalf("Failed to pin %s(%s) to shelf: %v", targetAppName, targetAppID, err)
	}

	// Wait until the shelf animation ends.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}, []int{0, 1, 2}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	var info *ash.ScrollableShelfInfoClass
	info, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to fetch the scrollable shelf info")
	}

	moveStartLocation := info.IconsBoundsInScreen[2].CenterPoint()
	moveEndLocation := info.IconsBoundsInScreen[0].CenterPoint()

	// Verify that app rearrangement works for a pinned shelf app.

	if err := uiauto.Combine("move the target icon from the third slot to the first slot", getDragAndDropActionStream(ac, tconn, moveStartLocation, moveEndLocation)...)(ctx); err != nil {
		s.Fatal("Failed to move the target icon from the third slot to the first slot: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}, []int{1, 2, 0}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	if err := uiauto.Combine("move the target icon from the first slot to the third slot", getDragAndDropActionStream(ac, tconn, moveEndLocation, moveStartLocation)...)(ctx); err != nil {
		s.Fatal("Failed to move the target icon from the first slot to the third slot")
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}, []int{0, 1, 2}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	// Verify that app rearrangement works for a pinned shelf app with the activated window.

	switch testType {
	case arcAppTest:
		if err := optin.Perform(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to perform Play Store optin: ", err)
		}

		if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
			// When we get here, play store is probably not shown, or it failed to be detected.
			// Just log the message and continue.
			s.Fatal("Failed to wait for Play Store to show: ", err)
		}

		// ...
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			s.Fatal("Failed to wait: ", err)
		}
	case chromeAppTest:
		fallthrough
	case fileAppTest:
		if err := ash.LaunchAppFromShelf(ctx, tconn, targetAppName, targetAppID); err != nil {
			s.Fatalf("Failed to launch %s(%s) from the shelf: %v", targetAppName, targetAppID, err)
		}
	}

	info, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to fetch the scrollable shelf info")
	}

	moveStartLocation = info.IconsBoundsInScreen[2].CenterPoint()
	moveEndLocation = info.IconsBoundsInScreen[0].CenterPoint()

	if err := uiauto.Combine("move the target icon with the activated window from the third slot to the first slot", getDragAndDropActionStream(ac, tconn, moveStartLocation, moveEndLocation)...)(ctx); err != nil {
		s.Fatal("Failed to move the target icon from the third slot to the first slot: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}, []int{1, 2, 0}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	if err := uiauto.Combine("move the target icon with the activated window from the first slot to the third slot", getDragAndDropActionStream(ac, tconn, moveEndLocation, moveStartLocation)...)(ctx); err != nil {
		s.Fatal("Failed to move the target icon from the first slot to the third slot")
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}, []int{0, 1, 2}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	// Verify that an unpinned app with the activated window should not be able to be dragged on top of other pinned apps.

	cleanUp, err = ash.UnpinApps(ctx, tconn, []string{targetAppID})
	if err != nil {
		s.Fatalf("Failed to unpin %s(%s): %v", targetAppName, targetAppID, err)
	}
	defer cleanUp(ctx)

	if err := uiauto.Combine("move the unpinned icon with the activated window from the third slot to the first slot", getDragAndDropActionStream(ac, tconn, moveStartLocation, moveEndLocation)...)(ctx); err != nil {
		s.Fatal("Failed to move the target icon from the third slot to the first slot: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}, []int{0, 1, 2}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	switch testType {
	case arcAppTest:
		if err := optin.ClosePlayStore(ctx, tconn); err != nil {
			s.Fatal("Failed to close Play Store: ", err)
		}
	case chromeAppTest:
		fallthrough
	case fileAppTest:
		activeWindow, err := ash.GetActiveWindow(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the active window: ", err)
		}
		if err := activeWindow.CloseWindow(ctx, tconn); err != nil {
			s.Fatalf("Failed to close the window(%s): %v", activeWindow.Name, err)
		}
	}
}

func getDragAndDropActionStream(ac *uiauto.Context, tconn *chrome.TestConn, startLocation, endLocation coords.Point) []uiauto.Action {
	return []uiauto.Action{mouse.Move(tconn, startLocation, 0),
		// Drag in tablet mode starts with a long press.
		mouse.Press(tconn, mouse.LeftButton),
		ac.Sleep(time.Second),

		mouse.Move(tconn, endLocation, 2*time.Second),
		mouse.Release(tconn, mouse.LeftButton),
		ac.Sleep(time.Second)}
}
