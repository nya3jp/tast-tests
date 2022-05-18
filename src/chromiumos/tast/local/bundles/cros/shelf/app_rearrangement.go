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
	chromeAppTest rearrangmentTestType = "ChromeAppTest" // Verify the rearrangement behavior on a Chrome app.
	fileAppTest   rearrangmentTestType = "FileAppTest"   // Verify the rearrangement behavior on the File app.
	pwaAppTest    rearrangmentTestType = "PwaAppTest"    // Verify the rearrangement behavior on a PWA.
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
		Timeout:      3 * time.Minute,
		Data:         []string{"web_app_install_force_list_index.html", "web_app_install_force_list_manifest.json", "web_app_install_force_list_service-worker.js", "web_app_install_force_list_icon-192x192.png", "web_app_install_force_list_icon-512x512.png"},
		Params: []testing.Param{
			{
				Name:    "rearrange_chrome_apps",
				Val:     chromeAppTest,
				Fixture: "install2Apps",
			},
			{
				Name:    "rearrange_file_app",
				Val:     fileAppTest,
				Fixture: "install2Apps",
			},
			{
				Name:    "rearrange_pwa_app",
				Val:     pwaAppTest,
				Fixture: fixture.ChromePolicyLoggedIn,
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
		fallthrough
	case fileAppTest:
		var err error
		cr, err = chrome.New(ctx, s.FixtValue().([]chrome.Option)...)

		if err != nil {
			s.Fatal("Failed to start chrome: ", err)
		}
		// defer cr.Close(ctx)
	case pwaAppTest:
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
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

	fakeAppIDs := make([]string, 0)
	installedApps, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the installed apps: ", err)
	}
	for _, app := range installedApps {
		if strings.Contains(app.Name, "fake") {
			fakeAppIDs = append(fakeAppIDs, app.AppID)
		}
	}
	if len(fakeAppIDs) != 2 {
		s.Fatalf("Failed to find all fake apps: expect to have 2; actually %q", len(fakeAppIDs))
	}

	appIDsToPin := append([]string{apps.Settings.ID}, fakeAppIDs[0])

	// Update appIDsToPin based on the test type.
	switch testType {
	case chromeAppTest:
		// Use a fake chrome app as the target app.
		appIDsToPin = append(appIDsToPin, fakeAppIDs[1])
	case fileAppTest:
		// Use the File app as the target app.
		appIDsToPin = append(appIDsToPin, apps.Files.ID)
	case pwaAppTest:
		fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
		var cleanUp func(ctx context.Context) error
		pwaAppID, _, cleanUp, err := policyutil.InstallPwaAppByPolicy(ctx, tconn, cr, fdms, s.DataFileSystem())
		if err != nil {
			s.Fatal("Failed to install PWA: ", err)
		}

		appIDsToPin = append(appIDsToPin, pwaAppID)

		// Use a shortened context for test operations to reserve time for cleanup.
		cleanupCtx := ctx
		var cancel context.CancelFunc
		ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		defer cleanUp(cleanupCtx)
	}

	// Pin additional apps to create a more complex scenario for testing.
	if err := ash.PinApps(ctx, tconn, appIDsToPin); err != nil {
		s.Fatalf("Failed to pin %v to shelf: %v", appIDsToPin, err)
	}

	if err := ash.WaitUntilShelfIconAnimationFinish(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for shelf icon animation to finish after pinning additional apps: ", err)
	}

	pinnedAppIDsInOrder := append([]string{chromeApp.ID}, appIDsToPin...)
	pinnedAppNamesInOrder, err := ash.ShelfItemTitleFromID(ctx, tconn, pinnedAppIDsInOrder)
	if err != nil {
		s.Fatalf("Failed to get the app names of %v: %v", pinnedAppIDsInOrder, err)
	}

	// Always use the last pinned app as the target app. The target app is the app that will be dragged and moved around the shelf.
	targetAppID := pinnedAppIDsInOrder[len(pinnedAppIDsInOrder)-1]
	targetAppName := pinnedAppNamesInOrder[len(pinnedAppNamesInOrder)-1]

	if err := ash.VerifyShelfIconIndices(ctx, tconn, pinnedAppIDsInOrder); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	ui := uiauto.New(tconn)
	if err := ash.VerifyShelfAppBounds(ctx, tconn, ui, pinnedAppNamesInOrder, true); err != nil {
		s.Fatal("Failed to verify shelf app bounds: ", err)
	}

	shelfAppBounds, err := ash.GetShelfAppBoundsFromNames(ctx, tconn, ui, pinnedAppNamesInOrder)
	if err != nil {
		s.Fatal("Failed to get shelf app bounds: ", err)
	}

	moveStartLocation := shelfAppBounds[len(shelfAppBounds)-1].CenterPoint()
	moveEndLocation := shelfAppBounds[0].CenterPoint()
	const middleAppIndex = 2
	middleAppBounds := shelfAppBounds[middleAppIndex]
	moveMiddleLocation := middleAppBounds.CenterPoint()

	if err := getStartDragAction(tconn, "start dragging on the target app", moveStartLocation)(ctx); err != nil {
		s.Fatal("Failed to start dragging on the target app before moving to the middle point from the last slot: ", err)
	}

	if err := uiauto.Combine("move to the middle point of shelf from the last slot",
		mouse.Move(tconn, moveMiddleLocation, 1*time.Second),
		uiauto.Sleep(time.Second))(ctx); err != nil {
		s.Fatal("Failed to move the target app to the middle location from the last slot: ", err)
	}

	shelfAppBounds, err = ash.GetShelfAppBoundsFromNames(ctx, tconn, ui, pinnedAppNamesInOrder)
	if err != nil {
		s.Fatal("Failed to get shelf app bounds after moving the target app from the last slot to the middle location: ", err)
	}

	// Expect that the app icon previously located on moveMiddleLocation moves rightward.
	if shelfAppBounds[middleAppIndex].Left <= middleAppBounds.Right() {
		s.Fatalf("Failed to check the app movement: expect %s to move rightward; actually it does not move or moves leftward", pinnedAppNamesInOrder[middleAppIndex])
	}

	if err := uiauto.Combine("move to the first slot then release", mouse.Move(tconn, moveEndLocation, time.Second),
		mouse.Release(tconn, mouse.LeftButton), uiauto.Sleep(time.Second))(ctx); err != nil {
		s.Fatalf("Failed to move %s to the first slot: %v", targetAppName, err)
	}

	// Update the pinned apps' ids and names in order.
	pinnedAppIDsInOrder = []string{targetAppID, chromeApp.ID, apps.Settings.ID, fakeAppIDs[0]}
	pinnedAppNamesInOrder, err = ash.ShelfItemTitleFromID(ctx, tconn, pinnedAppIDsInOrder)
	if err != nil {
		s.Fatalf("Failed to get the app names of %v: %v", pinnedAppIDsInOrder, err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, pinnedAppIDsInOrder); err != nil {
		s.Fatalf("Failed to verify shelf icon indices to be %v: %v", pinnedAppIDsInOrder, err)
	}

	shelfAppBounds, err = ash.GetShelfAppBoundsFromNames(ctx, tconn, ui, pinnedAppNamesInOrder)
	if err != nil {
		s.Fatal("Failed to get shelf app bounds: ", err)
	}

	if err := getStartDragAction(tconn, "start dragging on the target app", moveEndLocation)(ctx); err != nil {
		s.Fatal("Failed to start dragging on the target app before moving to the middle point from the first slot: ", err)
	}

	if err := uiauto.Combine("move to the middle point of shelf from the first slot",
		mouse.Move(tconn, moveMiddleLocation, 1*time.Second),
		uiauto.Sleep(time.Second))(ctx); err != nil {
		s.Fatal("Failed to move the target app to the middle location from the first slot: ", err)
	}

	shelfAppBounds, err = ash.GetShelfAppBoundsFromNames(ctx, tconn, ui, pinnedAppNamesInOrder)
	if err != nil {
		s.Fatal("Failed to get shelf app bounds after moving the target app from the last slot to the middle location: ", err)
	}

	// Expect that the app icon previously located on moveMiddleLocation moves leftward.
	if shelfAppBounds[middleAppIndex].Right() >= middleAppBounds.Left {
		s.Fatalf("Failed to check the app movement: expect %s to move leftward; actually it does not move or moves rightward", pinnedAppNamesInOrder[middleAppIndex])
	}

	if err := uiauto.Combine("move to the last slot then release", mouse.Move(tconn, moveStartLocation, time.Second),
		mouse.Release(tconn, mouse.LeftButton), uiauto.Sleep(time.Second))(ctx); err != nil {
		s.Fatalf("Failed to move %s to the last slot: %v", targetAppName, err)
	}

	if err := getDragAndDropAction(tconn, "move the target app from the first slot to the third slot", moveEndLocation, moveStartLocation)(ctx); err != nil {
		s.Fatal("Failed to move the target app from the first slot to the third slot")
	}

	// Fix here!!!
	if err := ash.VerifyShelfIconIndices(ctx, tconn, pinnedAppIDsInOrder); err != nil {
		s.Fatal("Failed to verify shelf icon indices: ", err)
	}

	// Launch the target app.
	if err := ash.LaunchAppFromShelf(ctx, tconn, targetAppName, targetAppID); err != nil {
		s.Fatalf("Failed to launch %s(%s) from the shelf: %v", targetAppName, targetAppID, err)
	}

	shelfAppBounds, err = ash.GetShelfAppBoundsFromNames(ctx, tconn, ui, pinnedAppNamesInOrder)
	if err != nil {
		s.Fatal("Failed to get shelf app bounds after launching the target app: ", err)
	}

	moveStartLocation = shelfAppBounds[len(shelfAppBounds)-1].CenterPoint()
	moveEndLocation = shelfAppBounds[0].CenterPoint()

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

func getStartDragAction(tconn *chrome.TestConn, actionName string, dragStartLocation coords.Point) uiauto.Action {
	return uiauto.Combine(actionName,
		mouse.Move(tconn, dragStartLocation, 0),
		// Drag in tablet mode starts with a long press.
		mouse.Press(tconn, mouse.LeftButton),
		uiauto.Sleep(time.Second))
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
