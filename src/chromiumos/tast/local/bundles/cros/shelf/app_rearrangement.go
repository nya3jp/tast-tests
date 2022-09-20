// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type rearrangmentTargetAppType string

const (
	chromeAppTest  rearrangmentTargetAppType = "ChromeAppTest"  // Verify the rearrangement behavior on a Chrome app.
	fileAppTest    rearrangmentTargetAppType = "FileAppTest"    // Verify the rearrangement behavior on the File app.
	pwaAppTest     rearrangmentTargetAppType = "PwaAppTest"     // Verify the rearrangement behavior on a PWA.
	androidAppTest rearrangmentTargetAppType = "AndroidAppTest" // Verify the rearrangement behavior on an Android app.
)

type rearrangmentTestType struct {
	appType  rearrangmentTargetAppType
	underRTL bool // If true, the system UI is adapted to right-to-left languages.
	bt       browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppRearrangement,
		LacrosStatus: testing.LacrosVariantExists,
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
		Params: []testing.Param{{
			Name: "rearrange_chrome_apps",
			Val: rearrangmentTestType{
				appType:  chromeAppTest,
				underRTL: false,
				bt:       browser.TypeAsh,
			},
			Fixture: "install2Apps",
		}, {
			Name: "rearrange_chrome_apps_rtl",
			Val: rearrangmentTestType{
				appType:  chromeAppTest,
				underRTL: true,
				bt:       browser.TypeAsh,
			},
			Fixture: "install2Apps",
		}, {
			Name: "rearrange_file_app",
			Val: rearrangmentTestType{
				appType:  fileAppTest,
				underRTL: false,
				bt:       browser.TypeAsh,
			},
			Fixture: "install2Apps",
		}, {
			Name: "rearrange_file_app_rtl",
			Val: rearrangmentTestType{
				appType:  fileAppTest,
				underRTL: true,
				bt:       browser.TypeAsh,
			},
			Fixture: "install2Apps",
		}, {
			Name: "rearrange_pwa_app",
			Val: rearrangmentTestType{
				appType:  pwaAppTest,
				underRTL: false,
				bt:       browser.TypeAsh,
			},
			Fixture: fixture.ChromePolicyLoggedIn,
		}, {
			Name: "rearrange_android_app_androidp",
			Val: rearrangmentTestType{
				appType:  androidAppTest,
				underRTL: false,
				bt:       browser.TypeAsh,
			},
			Fixture:           "arcBooted",
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "rearrange_android_app_androidvm",
			Val: rearrangmentTestType{
				appType:  androidAppTest,
				underRTL: false,
				bt:       browser.TypeAsh,
			},
			Fixture:           "arcBooted",
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name: "rearrange_chrome_apps_lacros",
			Val: rearrangmentTestType{
				appType:  chromeAppTest,
				underRTL: false,
				bt:       browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "install2LacrosApps",
		}, {
			Name: "rearrange_chrome_apps_rtl_lacros",
			Val: rearrangmentTestType{
				appType:  chromeAppTest,
				underRTL: true,
				bt:       browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "install2LacrosApps",
		}, {
			Name: "rearrange_file_app_lacros",
			Val: rearrangmentTestType{
				appType:  fileAppTest,
				underRTL: false,
				bt:       browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "install2LacrosApps",
		}, {
			Name: "rearrange_file_app_rtl_lacros",
			Val: rearrangmentTestType{
				appType:  fileAppTest,
				underRTL: true,
				bt:       browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "install2LacrosApps",
		}, {
			Name: "rearrange_pwa_app_lacros",
			Val: rearrangmentTestType{
				appType:  pwaAppTest,
				underRTL: false,
				bt:       browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
		}, {
			Name: "rearrange_android_app_androidp_lacros",
			Val: rearrangmentTestType{
				appType:  androidAppTest,
				underRTL: false,
				bt:       browser.TypeLacros,
			},
			Fixture:           "lacrosWithArcBooted",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
		}, {
			Name: "rearrange_android_app_androidvm_lacros",
			Val: rearrangmentTestType{
				appType:  androidAppTest,
				underRTL: false,
				bt:       browser.TypeLacros,
			},
			Fixture:           "lacrosWithArcBooted",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
		}},
	})
}

// AppRearrangement tests app icon rearrangement on the shelf.
func AppRearrangement(ctx context.Context, s *testing.State) {
	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	var cr *chrome.Chrome
	var closeBrowser uiauto.Action
	var err error

	testType := s.Param().(rearrangmentTestType)
	testAppType := testType.appType
	isunderRTL := testType.underRTL
	bt := testType.bt
	switch testAppType {
	case chromeAppTest, fileAppTest:
		options := s.FixtValue().([]chrome.Option)
		if isunderRTL {
			options = append(options, chrome.ExtraArgs("--lang=ar"))
		}
		cr, _, closeBrowser, err = browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(), options...)
		if err != nil {
			s.Fatalf("Failed to start %v browser: %v", bt, err)
		}
		defer cr.Close(cleanupCtx)
		defer closeBrowser(cleanupCtx)
	case pwaAppTest:
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
		// Setup the browser based on the type. ash-chrome can load PWA immediately on startup, but lacros-chrome won't until lacros process starts first.
		_, closeBrowser, err = browserfixt.SetUp(ctx, cr, bt)
		if err != nil {
			s.Fatalf("Failed to start %v browser: %v", bt, err)
		}
		defer closeBrowser(cleanupCtx)
	case androidAppTest:
		cr = s.FixtValue().(*arc.PreData).Chrome
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
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the primary browser app info: ", err)
	}

	var itemsToUnpin []string
	for _, item := range items {
		if item.AppID != browserApp.ID {
			itemsToUnpin = append(itemsToUnpin, item.AppID)
		}
	}

	// Unpin all apps except the browser.
	if err := ash.UnpinApps(ctx, tconn, itemsToUnpin); err != nil {
		s.Fatalf("Failed to unpin apps %v: %v", itemsToUnpin, err)
	}

	// The ids of the apps to pin.
	var appIDsToPin []string

	// The app ids by pin order before any drag-and-drop operations. An app that is pinned earlier has a smaller array index.
	var defaultAppIDsInPinOrder []string

	// The updated app ids by pin order after dragging the target app from the last slot to the first slot.
	var updatedAppIDsInPinOrder []string

	// Update appIDsToPin based on the test type.
	switch testAppType {
	case chromeAppTest:
		fakeAppIDs, err := fakeAppIDs(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get fake app names when the test type is ChromeAppTest: ", err)
		}

		if len(fakeAppIDs) != 2 {
			s.Fatalf("Failed to find all fake apps: want 2; got %q: ", len(fakeAppIDs))
		}

		appIDsToPin = []string{apps.Settings.ID, fakeAppIDs[1], fakeAppIDs[0]}
		defaultAppIDsInPinOrder = []string{browserApp.ID, apps.Settings.ID, fakeAppIDs[1], fakeAppIDs[0]}
		updatedAppIDsInPinOrder = []string{fakeAppIDs[0], browserApp.ID, apps.Settings.ID, fakeAppIDs[1]}

	case fileAppTest:
		fakeAppIDs, err := fakeAppIDs(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get fake app names when the test type is FileAppTest: ", err)
		}

		if len(fakeAppIDs) != 2 {
			s.Fatalf("Failed to find all fake apps: want 2; got %q: ", len(fakeAppIDs))
		}

		appIDsToPin = []string{apps.Settings.ID, fakeAppIDs[1], apps.FilesSWA.ID}
		defaultAppIDsInPinOrder = []string{browserApp.ID, apps.Settings.ID, fakeAppIDs[1], apps.FilesSWA.ID}
		updatedAppIDsInPinOrder = []string{apps.FilesSWA.ID, browserApp.ID, apps.Settings.ID, fakeAppIDs[1]}

	case pwaAppTest:
		fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
		var cleanUp func(ctx context.Context) error
		pwaAppID, _, cleanUp, err := policyutil.InstallPwaAppByPolicy(ctx, tconn, cr, fdms, s.DataFileSystem())
		if err != nil {
			s.Fatal("Failed to install PWA: ", err)
		}

		appIDsToPin = []string{apps.Settings.ID, apps.FilesSWA.ID, pwaAppID}
		defaultAppIDsInPinOrder = []string{browserApp.ID, apps.Settings.ID, apps.FilesSWA.ID, pwaAppID}
		updatedAppIDsInPinOrder = []string{pwaAppID, browserApp.ID, apps.Settings.ID, apps.FilesSWA.ID}

		// Use a shortened context for test operations to reserve time for cleanup.
		cleanupCtx := ctx
		var cancel context.CancelFunc
		ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		defer cleanUp(cleanupCtx)

	case androidAppTest:
		// Install a mock Android app under temporary sort.
		const apk = "ArcInstallAppWithAppListSortedTest.apk"
		a := s.FixtValue().(*arc.PreData).ARC
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatal("Failed installing app under temporary sort: ", err)
		}

		appName := "InstallAppWithAppListSortedMockApp"
		installedArcAppID, err := ash.WaitForChromeAppByNameInstalled(ctx, tconn, appName, 1*time.Minute)
		if err != nil {
			s.Fatalf("Failed to wait until %s is installed: %v", appName, err)
		}

		appIDsToPin = []string{apps.Settings.ID, apps.FilesSWA.ID, installedArcAppID}
		defaultAppIDsInPinOrder = []string{browserApp.ID, apps.Settings.ID, apps.FilesSWA.ID, installedArcAppID}
		updatedAppIDsInPinOrder = []string{installedArcAppID, browserApp.ID, apps.Settings.ID, apps.FilesSWA.ID}
	}

	// Pin additional apps to create a more complex scenario for testing.
	if err := ash.PinApps(ctx, tconn, appIDsToPin); err != nil {
		s.Fatalf("Failed to pin %v to shelf: %v", appIDsToPin, err)
	}

	if err := ash.WaitUntilShelfIconAnimationFinishAction(tconn)(ctx); err != nil {
		s.Fatal("Failed to wait for shelf icon animation to finish after pinning additional apps: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, defaultAppIDsInPinOrder); err != nil {
		s.Fatal("Failed to verify shelf icon indices before any drag-and-drop operations: ", err)
	}

	defaultPinnedAppNamesInOrder, err := ash.ShelfItemTitleFromID(ctx, tconn, defaultAppIDsInPinOrder)
	if err != nil {
		s.Fatalf("Failed to get the app names of default pinned apps %v: %v", defaultAppIDsInPinOrder, err)
	}

	// Always use the last app as the target app. The target app is the one that is going to be dragged around the shelf.
	targetAppName := defaultPinnedAppNamesInOrder[len(defaultPinnedAppNamesInOrder)-1]
	targetAppID := defaultAppIDsInPinOrder[len(defaultAppIDsInPinOrder)-1]

	ui := uiauto.New(tconn)
	if err := ash.VerifyShelfAppBounds(ctx, tconn, ui, appNamesInVisualOrder(defaultPinnedAppNamesInOrder, isunderRTL), true); err != nil {
		s.Fatal("Failed to verify shelf app bounds: ", err)
	}

	shelfAppBounds, err := ash.ShelfAppBoundsForNames(ctx, tconn, ui, defaultPinnedAppNamesInOrder)
	if err != nil {
		s.Fatal("Failed to get shelf app bounds: ", err)
	}

	firstSlotCenter := shelfAppBounds[0].CenterPoint()
	lastSlotCenter := shelfAppBounds[len(shelfAppBounds)-1].CenterPoint()

	const middleAppIndex = 2
	middleSlotBounds := shelfAppBounds[middleAppIndex]
	middleSlotCenter := middleSlotBounds.CenterPoint()
	middleAppName := defaultPinnedAppNamesInOrder[middleAppIndex]

	if err := startDragAction(tconn, "start drag on the target app from the last slot", lastSlotCenter)(ctx); err != nil {
		s.Fatal("Failed to start drag on the target app before moving to the middle point from the last slot: ", err)
	}

	if err := uiauto.Combine("move from the last slot to the middle slot",
		mouse.Move(tconn, middleSlotCenter, 1*time.Second),
		ash.WaitUntilShelfIconAnimationFinishAction(tconn))(ctx); err != nil {
		s.Fatal("Failed to move the target app from the last slot to the middle slot: ", err)
	}

	updatedMiddleAppBounds, err := ash.ShelfAppBoundsForNames(ctx, tconn, ui, []string{middleAppName})
	if err != nil {
		s.Fatal("Failed to get shelf app bounds after moving the target app from the last slot to the middle slot: ", err)
	}

	if !isunderRTL && updatedMiddleAppBounds[0].Left <= middleSlotBounds.Right() {
		// Expect that the app icon previously located on moveMiddleLocation moves rightward when it is not under RTL.
		s.Fatalf("Failed to check the app movement: want %s to move rightward; actually it does not move or moves leftward", middleAppName)
	}

	if isunderRTL && updatedMiddleAppBounds[0].Right() >= middleSlotBounds.Left {
		// Expect that the app icon previously located on moveMiddleLocation moves leftward when it is under RTL.
		s.Fatalf("Failed to check the app movement under RTL: want %s to move leftward; actually it does not move or moves rightward", middleAppName)
	}

	if err := uiauto.Combine("move to the first slot then release", mouse.Move(tconn, firstSlotCenter, time.Second),
		ash.WaitUntilShelfIconAnimationFinishAction(tconn),
		mouse.Release(tconn, mouse.LeftButton), uiauto.Sleep(time.Second))(ctx); err != nil {
		s.Fatalf("Failed to move %s to the first slot: %v", targetAppName, err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, updatedAppIDsInPinOrder); err != nil {
		s.Fatalf("Failed to verify shelf icon indices to be %v: %v", updatedAppIDsInPinOrder, err)
	}

	// Update middleAppName after drag-and-drop.
	updatedPinnedAppNamesInOrder, err := ash.ShelfItemTitleFromID(ctx, tconn, updatedAppIDsInPinOrder)
	if err != nil {
		s.Fatalf("Failed to get the app names of the updated pinned apps %v: %v", updatedAppIDsInPinOrder, err)
	}
	middleAppName = updatedPinnedAppNamesInOrder[middleAppIndex]

	if err := startDragAction(tconn, "start drag on the target app from the first slot", firstSlotCenter)(ctx); err != nil {
		s.Fatal("Failed to start drag on the target app before moving from the first slot to the middle slot: ", err)
	}

	if err := uiauto.Combine("move from the first slot to the middle slot",
		mouse.Move(tconn, middleSlotCenter, 1*time.Second),
		ash.WaitUntilShelfIconAnimationFinishAction(tconn))(ctx); err != nil {
		s.Fatal("Failed to move the target app from the first slot to the middle slot: ", err)
	}

	updatedMiddleAppBounds, err = ash.ShelfAppBoundsForNames(ctx, tconn, ui, []string{middleAppName})
	if err != nil {
		s.Fatal("Failed to get shelf app bounds after moving the target app from the first slot to the middle location: ", err)
	}

	if !isunderRTL && updatedMiddleAppBounds[0].Right() >= middleSlotBounds.Left {
		// Expect that the app icon previously located on moveMiddleLocation moves leftward.
		s.Fatalf("Failed to check the app movement after moving from the first to middle: want %s to move leftward; actually it does not move or moves rightward", middleAppName)
	}

	if isunderRTL && updatedMiddleAppBounds[0].Left <= middleSlotBounds.Right() {
		// Expect that the app icon previously located on moveMiddleLocation moves rightward under RTL.
		s.Fatalf("Failed to check the app movement after moving from the first to middle under RTL: want %s to move rightward; actually it does not move or moves leftward", middleAppName)
	}

	if err := uiauto.Combine("move to the last slot then release", mouse.Move(tconn, lastSlotCenter, time.Second), ash.WaitUntilShelfIconAnimationFinishAction(tconn),
		mouse.Release(tconn, mouse.LeftButton), uiauto.Sleep(time.Second))(ctx); err != nil {
		s.Fatalf("Failed to move %s to the last slot: %v", targetAppName, err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, defaultAppIDsInPinOrder); err != nil {
		s.Fatal("Failed to verify shelf icon indices before launching the target app: ", err)
	}

	// Launch the target app.
	if err := ash.LaunchAppFromShelf(ctx, tconn, targetAppName, targetAppID); err != nil {
		s.Fatalf("Failed to launch %s(%s) from the shelf: %v", targetAppName, targetAppID, err)
	}

	// Verify that app rearrangement works for a pinned shelf app with the activated window.
	if err := getDragAndDropAction(tconn, "move the target app with the activated window from the last slot to the first slot", lastSlotCenter, firstSlotCenter)(ctx); err != nil {
		s.Fatal("Failed to move the target app with the activated window from the last slot to the first slot: ", err)
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, updatedAppIDsInPinOrder); err != nil {
		s.Fatal("Failed to verify shelf icon indices after moving the target app with the activated window from the last slot to the first slot: ", err)
	}

	if err := getDragAndDropAction(tconn, "move the target app with the activated window from the first slot to the last slot", firstSlotCenter, lastSlotCenter)(ctx); err != nil {
		s.Fatal("Failed to move the target app with the activated window from the first slot to the last slot")
	}

	if err := ash.VerifyShelfIconIndices(ctx, tconn, defaultAppIDsInPinOrder); err != nil {
		s.Fatal("Failed to verify shelf icon indices after moving the target app with the activated window from the first slot to the last slot: ", err)
	}

	if err := ash.UnpinApps(ctx, tconn, []string{targetAppID}); err != nil {
		s.Fatalf("Failed to unpin %s(%s): %v", targetAppName, targetAppID, err)
	}

	var pinnedApps []string
	var numPinnedApps int

	if pinnedApps, err = ash.GetPinnedAppIds(ctx, tconn); err != nil {
		s.Fatal("Failed to get the pinned app ids before dragging the unpinned app: ", err)
	}

	numPinnedApps = len(pinnedApps)

	if err := getDragAndDropAction(tconn, "move the unpinned app with the activated window from the last slot to the first slot", lastSlotCenter, firstSlotCenter)(ctx); err != nil {
		s.Fatal("Failed to move the unpinned app from the last slot to the first slot: ", err)
	}

	// Verify that an unpinned app can be moved across the separator to the pinned apps and pin the app.
	if err := ash.VerifyShelfIconIndices(ctx, tconn, updatedAppIDsInPinOrder); err != nil {
		s.Fatal("Failed to verify shelf icon indices after the unpinned app is dragged then dropped: ", err)
	}

	if pinnedApps, err = ash.GetPinnedAppIds(ctx, tconn); err != nil {
		s.Fatal("Failed to get the pinned app ids after dragging the unpinned app: ", err)
	}

	// The number of pinned apps should be increased by 1 after pinning the unpinned app by dragging.
	if len(pinnedApps) != numPinnedApps+1 {
		s.Fatal("Failed to pin the dragged unpinned app")
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

func fakeAppIDs(ctx context.Context, tconn *chrome.TestConn) ([]string, error) {
	fakeAppIDs := make([]string, 0)
	installedApps, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the installed apps")
	}
	for _, app := range installedApps {
		if strings.Contains(app.Name, "fake") {
			fakeAppIDs = append(fakeAppIDs, app.AppID)
		}
	}

	return fakeAppIDs, nil
}

func startDragAction(tconn *chrome.TestConn, actionName string, dragStartLocation coords.Point) uiauto.Action {
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
		ash.WaitUntilShelfIconAnimationFinishAction(tconn),
		mouse.Release(tconn, mouse.LeftButton),
		uiauto.Sleep(time.Second))
}

// appNamesInVisualOrder returns the apps names in visual order from the name array in pin order.
func appNamesInVisualOrder(namesInPinOrder []string, isunderRTL bool) []string {
	// When it is not under RTL, the visual order is the same as the pin order.
	if !isunderRTL {
		return namesInPinOrder
	}

	// Under RTL, the visual order is the reversal of the pin order.
	size := len(namesInPinOrder)
	namesInVisualOrder := make([]string, size)
	for i := size - 1; i >= 0; i-- {
		namesInVisualOrder[size-1-i] = namesInPinOrder[i]
	}
	return namesInVisualOrder
}
