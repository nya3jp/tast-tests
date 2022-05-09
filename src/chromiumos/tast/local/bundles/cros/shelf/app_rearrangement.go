// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type rearrangmentTestType string

const (
	chromeAppTest  rearrangmentTestType = "ChromeAppTest"  // Verify the rearrangement behavior on a Chrome app.
	fileAppTest    rearrangmentTestType = "FileAppTest"    // Verify the rearrangement behavior on the File app.
	pwaAppTest     rearrangmentTestType = "PwaAppTest"     // Verify the rearrangement behavior on a PWA.
	androidAppTest rearrangmentTestType = "AndroidAppTest" // Verify the rearrangement behavior on an Android app.
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppRearrangement,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests the rearrangement of shelf app icons",
		Contacts: []string{
			"andrewxu@chromium.org",
			"tbarzic@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"web_app_install_force_list_index.html", "web_app_install_force_list_manifest.json", "web_app_install_force_list_service-worker.js", "web_app_install_force_list_icon-192x192.png", "web_app_install_force_list_icon-512x512.png"},
		Params: []testing.Param{
			{
				Name:    "rearrange_chrome_apps",
				Val:     chromeAppTest,
				Fixture: "install2Apps",
				Timeout: 3 * time.Minute,
			},
			{
				Name:    "rearrange_file_app",
				Val:     fileAppTest,
				Fixture: "chromeLoggedIn",
				Timeout: 3 * time.Minute,
			},
			{
				Name:    "rearrange_pwa_app",
				Val:     pwaAppTest,
				Fixture: fixture.ChromePolicyLoggedIn,
				Timeout: 3 * time.Minute,
			},
			{
				Name:              "rearrange_android_app_androidp",
				Val:               androidAppTest,
				Fixture:           "chromeEnrolledLoggedInARC",
				ExtraSoftwareDeps: []string{"android_p"},
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
			{
				Name:              "rearrange_android_app_androidvm",
				Val:               androidAppTest,
				Fixture:           "chromeEnrolledLoggedInARC",
				ExtraSoftwareDeps: []string{"android_vm"},
				Timeout:           chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
			},
		},
	})
}

// AppRearrangement tests app icon rearrangement on the shelf.
func AppRearrangement(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome

	testType := s.Param().(rearrangmentTestType)
	switch testType {
	case chromeAppTest:
		var err error
		cr, err = chrome.New(ctx, s.FixtValue().([]chrome.Option)...)

		if err != nil {
			s.Fatal("Failed to start chrome: ", err)
		}
		defer cr.Close(ctx)
	case fileAppTest:
		cr = s.FixtValue().(*chrome.Chrome)
	case pwaAppTest:
	case androidAppTest:
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	var a *arc.ARC
	if testType == androidAppTest {
		// Setup ARC.
		a, err = arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close(ctx)
	}

	// Ensure that the device is in clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(ctx)

	resetPinState, err := ash.ResetShelfPinState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the function to reset pin states: ", err)
	}
	defer resetPinState(ctx)

	items, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	var itemsToUnpin []string
	for _, item := range items {
		if item.AppID != chromeApp.ID {
			itemsToUnpin = append(itemsToUnpin, item.AppID)
		}
	}

	// Unpin all apps except the browser.
	if err := ash.UnpinApps(ctx, tconn, itemsToUnpin); err != nil {
		s.Fatalf("Failed to unpin apps %v: %v", itemsToUnpin, err)
	}

	// Pin an extra app to create a more complex scenario for testing.
	if err := ash.PinApps(ctx, tconn, []string{apps.Settings.ID}); err != nil {
		s.Fatal("Failed to pin Settings to shelf")
	}

	// Calculate the name and the ID of the target app which is the app that is going to be dragged and dropped on the shelf.
	var targetAppName string
	var targetAppID string
	switch testType {
	case chromeAppTest:
		installedApps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get the installed apps: ", err)
		}

		// Use a fake app as the target app.
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
		// Use the File app as the target app.
		targetAppID = apps.Files.ID
		targetAppName = apps.Files.Name
	case pwaAppTest:
		fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
		var cleanUp func(ctx context.Context) error
		targetAppID, targetAppName, cleanUp, err = policyutil.InstallPwaAppByPolicy(ctx, tconn, cr, fdms, s.DataFileSystem())
		if err != nil {
			s.Fatal("Failed to install PWA: ", err)
		}

		// Use a shortened context for test operations to reserve time for cleanup.
		cleanupCtx := ctx
		var cancel context.CancelFunc
		ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		defer cleanUp(cleanupCtx)

	case androidAppTest:
		// Install a mock Android app under temporary sort.
		const apk = "ArcInstallAppWithAppListSortedTest.apk"
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatal("Failed installing app under temporary sort: ", err)
		}

		targetAppName = "InstallAppWithAppListSortedMockApp"
		targetAppID, err = ash.WaitForChromeAppByNameInstalled(ctx, tconn, targetAppName, 1*time.Minute)
		if err != nil {
			s.Fatalf("Failed to wait until %s is installed: %v", targetAppName, err)
		}
	}

	// Pin the target app.
	if err := ash.PinApps(ctx, tconn, []string{targetAppID}); err != nil {
		s.Fatalf("Failed to pin %s(%s) to shelf: %v", targetAppName, targetAppID, err)
	}

	if err := ash.WaitUntilShelfIconAnimationFinish(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for shelf icon animation to finish: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}); err != nil {
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
	if err := getDragAndDropAction(tconn, "move the target app from the third slot to the first slot", moveStartLocation, moveEndLocation)(ctx); err != nil {
		s.Fatal("Failed to move the target app from the third slot to the first slot: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{targetAppID, chromeApp.ID, apps.Settings.ID}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	if err := getDragAndDropAction(tconn, "move the target app from the first slot to the third slot", moveEndLocation, moveStartLocation)(ctx); err != nil {
		s.Fatal("Failed to move the target app from the first slot to the third slot")
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	// Launch the target app.
	if err := ash.LaunchAppFromShelf(ctx, tconn, targetAppName, targetAppID); err != nil {
		s.Fatalf("Failed to launch %s(%s) from the shelf: %v", targetAppName, targetAppID, err)
	}

	info, err = ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
	if err != nil {
		s.Fatal("Failed to fetch the scrollable shelf info")
	}

	moveStartLocation = info.IconsBoundsInScreen[2].CenterPoint()
	moveEndLocation = info.IconsBoundsInScreen[0].CenterPoint()

	// Verify that app rearrangement works for a pinned shelf app with the activated window.
	if err := getDragAndDropAction(tconn, "move the target app with the activated window from the third slot to the first slot", moveStartLocation, moveEndLocation)(ctx); err != nil {
		s.Fatal("Failed to move the target app with the activated window from the third slot to the first slot: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{targetAppID, chromeApp.ID, apps.Settings.ID}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	if err := getDragAndDropAction(tconn, "move the target app with the activated window from the first slot to the third slot", moveEndLocation, moveStartLocation)(ctx); err != nil {
		s.Fatal("Failed to move the target app with the activated window from the first slot to the third slot")
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{targetAppID}); err != nil {
		s.Fatalf("Failed to unpin %s(%s): %v", targetAppName, targetAppID, err)
	}

	// Verify that an unpinned app with the activated window should not be able to be placed in front of the pinned apps.
	if err := getDragAndDropAction(tconn, "move the unpinned app with the activated window from the third slot to the first slot", moveStartLocation, moveEndLocation)(ctx); err != nil {
		s.Fatal("Failed to move the unpinned app from the third slot to the first slot: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, []string{chromeApp.ID, apps.Settings.ID, targetAppID}); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	// Cleanup.
	activeWindow, err := ash.GetActiveWindow(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the active window: ", err)
	}
	if err := activeWindow.CloseWindow(ctx, tconn); err != nil {
		s.Fatalf("Failed to close the window(%s): %v", activeWindow.Name, err)
	}
}

func getDragAndDropAction(tconn *chrome.TestConn, actionName string, startLocation, endLocation coords.Point) uiauto.Action {
	return uiauto.Combine(actionName,
		mouse.Move(tconn, startLocation, 0),
		// Drag in tablet mode starts with a long press.
		mouse.Press(tconn, mouse.LeftButton),
		uiauto.Sleep(time.Second),

		mouse.Move(tconn, endLocation, 2*time.Second),
		mouse.Release(tconn, mouse.LeftButton),
		uiauto.Sleep(time.Second))
}
