// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ash implements a library used for communication with Chrome Ash.
package ash

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// ShelfBehavior represents the different Chrome OS shelf behaviors.
type ShelfBehavior string

// As defined in ShelfAutoHideBehavior here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	// ShelfBehaviorAlwaysAutoHide represents always auto-hide.
	ShelfBehaviorAlwaysAutoHide ShelfBehavior = "always"
	//ShelfBehaviorNeverAutoHide represents never auto-hide, meaning that it is always visible.
	ShelfBehaviorNeverAutoHide = "never"
	// ShelfBehaviorHidden represents always hidden, used for debugging, since this state is not exposed to the user.
	ShelfBehaviorHidden = "hidden"
	// ShelfBehaviorInvalid represents an invalid state.
	ShelfBehaviorInvalid = "invalid"
)

// SetShelfBehavior sets the shelf visibility behavior.
// displayID is the display that contains the shelf.
func SetShelfBehavior(ctx context.Context, tconn *chrome.TestConn, displayID string, b ShelfBehavior) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAutoHideBehavior(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, b)
	return tconn.EvalPromise(ctx, expr, nil)
}

// GetShelfBehavior returns the shelf visibility behavior.
// displayID is the display that contains the shelf.
func GetShelfBehavior(ctx context.Context, tconn *chrome.TestConn, displayID string) (ShelfBehavior, error) {
	var b ShelfBehavior
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAutoHideBehavior(%q, function(behavior) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(behavior);
		    }
		  });
		})`, displayID)
	if err := tconn.EvalPromise(ctx, expr, &b); err != nil {
		return ShelfBehaviorInvalid, err
	}
	switch b {
	case ShelfBehaviorAlwaysAutoHide, ShelfBehaviorNeverAutoHide, ShelfBehaviorHidden:
	default:
		return ShelfBehaviorInvalid, errors.Errorf("invalid shelf behavior %q", b)
	}
	return b, nil
}

// WaitForShelf waits for the shelf to exist in the UI tree.
func WaitForShelf(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	params := ui.FindParams{Role: ui.RoleTypeToolbar, ClassName: "ShelfView"}
	if err := ui.WaitUntilExists(ctx, tconn, params, timeout); err != nil {
		return errors.Wrap(err, "shelf not found")
	}
	return nil
}

// PinApp pins the shelf icon for the app specified by appID.
func PinApp(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	query := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.pinShelfIcon)(%q)", appID)
	return tconn.EvalPromise(ctx, query, nil)
}

// ShelfAlignment represents the different Chrome OS shelf alignments.
type ShelfAlignment string

// As defined in ShelfAlignment here:
// https://cs.chromium.org/chromium/src/ash/public/cpp/shelf_types.h
const (
	ShelfAlignmentBottom       ShelfAlignment = "Bottom"
	ShelfAlignmentLeft                        = "Left"
	ShelfAlignmentRight                       = "Right"
	ShelfAlignmentBottomLocked                = "BottomLocked"
	ShelfAlignmentInvalid                     = "Invalid"
)

// SetShelfAlignment sets the shelf alignment.
// displayID is the display that contains the shelf.
func SetShelfAlignment(ctx context.Context, tconn *chrome.TestConn, displayID string, a ShelfAlignment) error {
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.setShelfAlignment(%q, %q, function() {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve();
		    }
		  });
		})`, displayID, a)
	return tconn.EvalPromise(ctx, expr, nil)
}

// GetShelfAlignment returns the shelf alignment.
// displayID is the display that contains the shelf.
func GetShelfAlignment(ctx context.Context, tconn *chrome.TestConn, displayID string) (ShelfAlignment, error) {
	var a ShelfAlignment
	expr := fmt.Sprintf(
		`new Promise(function(resolve, reject) {
		  chrome.autotestPrivate.getShelfAlignment(%q, function(alignment) {
		    if (chrome.runtime.lastError) {
		      reject(new Error(chrome.runtime.lastError.message));
		    } else {
		      resolve(alignment);
		    }
		  });
		})`, displayID)
	if err := tconn.EvalPromise(ctx, expr, &a); err != nil {
		return ShelfAlignmentInvalid, err
	}
	switch a {
	case ShelfAlignmentBottom, ShelfAlignmentLeft, ShelfAlignmentRight, ShelfAlignmentBottomLocked:
	default:
		return ShelfAlignmentInvalid, errors.Errorf("invalid shelf alignment %q", a)
	}
	return a, nil
}

// ShelfItemType represents the type of a shelf item.
type ShelfItemType string

// As defined in ShelfItemType in autotest_private.idl.
const (
	ShelfItemTypeApp       ShelfItemType = "App"
	ShelfItemTypePinnedApp ShelfItemType = "PinnedApp"
	ShelfItemTypeShortcut  ShelfItemType = "BrowserShortcut"
	ShelfItemTypeDialog    ShelfItemType = "Dialog"
)

// ShelfItemStatus repsents the type of the current status of a shelf item.
type ShelfItemStatus string

// As defined in ShelfItemStatus in autotest_private.idl.
const (
	ShelfItemClosed    ShelfItemStatus = "Closed"
	ShelfItemRunning   ShelfItemStatus = "Running"
	ShelfItemAttention ShelfItemStatus = "Attention"
)

// ShelfItem corresponds to the "ShelfItem" defined in autotest_private.idl.
type ShelfItem struct {
	AppID           string          `json:"appId"`
	LaunchID        string          `json:"launchId"`
	Title           string          `json:"title"`
	Type            ShelfItemType   `json:"type"`
	Status          ShelfItemStatus `json:"status"`
	ShowsToolTip    bool            `json:"showsTooltip"`
	PinnedByPolicy  bool            `json:"pinnedByPolicy"`
	HasNotification bool            `json:"hasNotification"`
}

// ShelfState corresponds to the "ShelfState" defined in autotest_private.idl
type ShelfState struct {
	ScrollDistance float32 `json:"scrollDistance"`
}

// ScrollableShelfInfoClass corresponds to the "ScrollableShelfInfo" defined in autotest_private.idl
type ScrollableShelfInfoClass struct {
	MainAxisOffset         float32        `json:"mainAxisOffset"`
	PageOffset             float32        `json:"pageOffset"`
	TargetMainAxisOffset   float32        `json:"targetMainAxisOffset"`
	LeftArrowBounds        coords.Rect    `json:"leftArrowBounds"`
	RightArrowBounds       coords.Rect    `json:"rightArrowBounds"`
	IsAnimating            bool           `json:"isAnimating"`
	IsOverflow             bool           `json:"isOverflow"`
	IsShelfWidgetAnimating bool           `json:"isShelfWidgetAnimating"`
	IconsBoundsInScreen    []*coords.Rect `json:"iconsBoundsInScreen"`
}

// HotseatStateType corresponds to the "HotseatState" defined in autotest_private.idl.
type HotseatStateType string

const (
	// ShelfHidden means that hotseat is shown off screen.
	ShelfHidden HotseatStateType = "Hidden"
	// ShelfShownClamShell means that hotseat is shown within the shelf in clamshell mode.
	ShelfShownClamShell HotseatStateType = "ShownClamShell"
	// ShelfShownHomeLauncher means that hotseat is shown in the tablet mode home launcher's shelf.
	ShelfShownHomeLauncher HotseatStateType = "ShownHomeLauncher"
	// ShelfExtended means that hotseat is shown above the shelf.
	ShelfExtended HotseatStateType = "Extended"
)

// HotseatSwipeDescriptor corresponds to the "HotseatSwipeDescriptor" defined in autotest_private.idl.
type HotseatSwipeDescriptor struct {
	SwipeStartLocation coords.Point `json:"swipeStartLocation"`
	SwipeEndLocation   coords.Point `json:"swipeEndLocation"`
}

// HotseatInfoClass corresponds to the "HotseatInfo" defined in autotest_private.idl.
type HotseatInfoClass struct {
	SwipeUp      HotseatSwipeDescriptor `json:"swipeUp"`
	HotseatState HotseatStateType       `json:"state"`
	IsAnimating  bool                   `json:"isAnimating"`
}

// ShelfInfo corresponds to the "ShelfInfo" defined in autotest_private.idl.
type ShelfInfo struct {
	HotseatInfo         HotseatInfoClass         `json:"hotseatInfo"`
	ScrollableShelfInfo ScrollableShelfInfoClass `json:"scrollableShelfInfo"`
}

// AppType defines the types of available apps.
type AppType string

// Corresponds to the definition in autotest_private.idl.
const (
	Arc       AppType = "Arc"
	BuiltIn   AppType = "BuiltIn"
	Crostini  AppType = "Crostini"
	Extension AppType = "Extension"
	Lacros    AppType = "Lacros"
	Web       AppType = "Web"
	MacNative AppType = "MacNative"
)

// AppInstallSource maps apps::mojom::InstallSource.
type AppInstallSource string

// Corresponds to the definition in autotest_private.idl
const (
	Unknown AppInstallSource = "Unknown"
	System  AppInstallSource = "System"
	Policy  AppInstallSource = "Policy"
	Oem     AppInstallSource = "Oem"
	Default AppInstallSource = "Default"
	Sync    AppInstallSource = "Sync"
	User    AppInstallSource = "User"
)

// AppReadiness maps apps::mojom::Readiness.
type AppReadiness string

// Corresponds to the definition in autotest_private.idl
const (
	Ready               AppReadiness = "Ready"
	DisabledByBlacklist AppReadiness = "DisabledByBlacklist"
	DisabledByPolicy    AppReadiness = "DisabledByPolicy"
	DisabledByUser      AppReadiness = "DisabledByUser"
	Terminated          AppReadiness = "Terminated"
	UninstalledByUser   AppReadiness = "UninstalledByUser"
)

// ChromeApp corresponds to the "App" defined in autotest_private.idl.
type ChromeApp struct {
	AppID                 string           `json:"appId"`
	Name                  string           `json:"name"`
	ShortName             string           `json:"shortName"`
	PublisherID           string           `json:"publisherId"`
	Type                  AppType          `json:"type"`
	InstallSource         AppInstallSource `json:"installSource"`
	Readiness             AppReadiness     `json:"readiness"`
	AdditionalSearchTerms []string         `json:"additionalSearchTerms"`
	ShowInLauncher        bool             `json:"showInLauncher"`
	ShowInSearch          bool             `json:"showInSearch"`
}

// ChromeApps returns all of the installed apps.
func ChromeApps(ctx context.Context, tconn *chrome.TestConn) ([]*ChromeApp, error) {
	var s []*ChromeApp
	chromeQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.getAllInstalledApps)()")
	if err := tconn.EvalPromise(ctx, chromeQuery, &s); err != nil {
		return nil, errors.Wrap(err, "failed to call getAllInstalledApps")
	}
	return s, nil
}

// ChromeAppInstalled checks if an app specified by appID is installed.
func ChromeAppInstalled(ctx context.Context, tconn *chrome.TestConn, appID string) (bool, error) {
	installedApps, err := ChromeApps(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get all installed Apps")
	}

	for _, app := range installedApps {
		if app.AppID == appID {
			return true, nil
		}
	}
	return false, nil
}

// WaitForChromeAppInstalled waits for the app specified by appID to appear in installed apps.
func WaitForChromeAppInstalled(ctx context.Context, tconn *chrome.TestConn, appID string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if installed, err := ChromeAppInstalled(ctx, tconn, appID); err != nil {
			return testing.PollBreak(err)
		} else if !installed {
			return errors.New("failed to wait for installed app by id: " + appID)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// ShelfItems returns the list of apps in the shelf.
func ShelfItems(ctx context.Context, tconn *chrome.TestConn) ([]*ShelfItem, error) {
	var s []*ShelfItem
	shelfQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.getShelfItems)()")
	if err := tconn.EvalPromise(ctx, shelfQuery, &s); err != nil {
		return nil, errors.Wrap(err, "failed to call getShelfItems")
	}
	return s, nil
}

func fetchShelfInfoForState(ctx context.Context, c *chrome.TestConn, state *ShelfState) (*ShelfInfo, error) {
	var s ShelfInfo

	const ShelfQuery = "tast.promisify(chrome.autotestPrivate.getShelfUIInfoForState)"
	if err := c.Call(ctx, &s, ShelfQuery, state); err != nil {
		return nil, errors.Wrap(err, "failed to call getShelfUIInfoForState")
	}
	return &s, nil
}

// FetchScrollableShelfInfoForState returns the scrollable shelf's ui related information for the given state.
func FetchScrollableShelfInfoForState(ctx context.Context, c *chrome.TestConn, state *ShelfState) (*ScrollableShelfInfoClass, error) {
	shelfInfo, err := fetchShelfInfoForState(ctx, c, state)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch scrollable shelf info")
	}

	return &shelfInfo.ScrollableShelfInfo, nil
}

// FetchHotseatInfo returns the hotseat's ui related information.
func FetchHotseatInfo(ctx context.Context, c *chrome.TestConn) (*HotseatInfoClass, error) {
	shelfInfo, err := fetchShelfInfoForState(ctx, c, &ShelfState{})
	if err != nil {

		return nil, errors.Wrap(err, "failed to fetch hotseat info")
	}
	return &shelfInfo.HotseatInfo, nil
}

// ScrollShelfAndWaitUntilFinish triggers the scroll animation by mouse click then waits the animation to finish.
func ScrollShelfAndWaitUntilFinish(ctx context.Context, tconn *chrome.TestConn, buttonBounds coords.Rect, targetOffset float32) error {
	// Before pressing the arrow button, wait scrollable shelf to be idle.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := FetchScrollableShelfInfoForState(ctx, tconn, &ShelfState{})
		if err != nil {
			return errors.Wrap(err, "failed to fetch scrollable shelf's information when waiting for scroll animation")
		}
		if info.IsAnimating {
			return errors.New("unexpected scroll animation status: got true; want false")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait scrollable shelf to be idle before starting the scroll animation")
	}

	// Press the arrow button.
	if err := mouse.Click(ctx, tconn, buttonBounds.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to trigger the scroll animation by clicking at the arrow button")
	}

	// Wait the scroll animation to finish.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := FetchScrollableShelfInfoForState(ctx, tconn, &ShelfState{})
		if err != nil {
			return errors.Wrap(err, "failed to fetch scrollable shelf's information when waiting for scroll animation")
		}
		if info.MainAxisOffset != targetOffset || info.IsAnimating {
			return errors.Errorf("unexpected scrollable shelf status; actual offset: %f, actual animation status: %t, target offset: %f, target animation status: false", info.MainAxisOffset, info.IsAnimating, targetOffset)
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait scrollable shelf to finish scroll animation")
	}

	return nil
}

// AppShown checks if an app specified by appID is shown in the shelf.
func AppShown(ctx context.Context, tconn *chrome.TestConn, appID string) (bool, error) {
	var appShown bool
	shownQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.isAppShown)(%q)", appID)
	if err := tconn.EvalPromise(ctx, shownQuery, &appShown); err != nil {
		errors.Errorf("Running autotestPrivate.isAppShown failed for %v", appID)
		return false, err
	}
	return appShown, nil
}

// WaitForApp waits for the app specified by appID to appear in the shelf.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if visible, err := AppShown(ctx, tconn, appID); err != nil {
			return testing.PollBreak(err)
		} else if !visible {
			return errors.New("app is not shown yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}

// WaitForAppClosed waits for the app specified by appID to be closed.
func WaitForAppClosed(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if visible, err := AppShown(ctx, tconn, appID); err != nil {
			return testing.PollBreak(err)
		} else if visible {
			return errors.New("app is not closed yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}

// WaitForHotseatAnimatingToIdealState waits for the hotseat to reach the expected state after animation.
func WaitForHotseatAnimatingToIdealState(ctx context.Context, tc *chrome.TestConn, state HotseatStateType) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := fetchShelfInfoForState(ctx, tc, &ShelfState{})
		if err != nil {
			return err
		}

		hotseatInfo := info.HotseatInfo
		if hotseatInfo.IsAnimating || hotseatInfo.HotseatState != state {
			return errors.Errorf("got hotseat (state, animating) = (%v, %v); want (%v, false)", hotseatInfo.HotseatState, hotseatInfo.IsAnimating, state)
		}

		scrollableShelfInfo := info.ScrollableShelfInfo
		if scrollableShelfInfo.IsShelfWidgetAnimating {
			return errors.New("got hotseat widget animation state is true; want false")
		}

		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for the expected hotseat state")
	}

	return nil
}

// SwipeUpHotseatAndWaitForCompletion swipes the hotseat up, changing the hotseat state from hidden to extended. The function does not end until the hotseat animation completes.
func SwipeUpHotseatAndWaitForCompletion(ctx context.Context, tconn *chrome.TestConn, stw *input.SingleTouchEventWriter, tcc *input.TouchCoordConverter) error {
	if err := WaitForHotseatAnimationToFinish(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for the animation to finish")
	}

	info, err := FetchHotseatInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the hotseat info")
	}

	// If the hotseat is visible and it is not animating to hidden, we can simply return.
	if info.HotseatState != ShelfHidden {
		return nil
	}

	// Convert the gesture locations from screen coordinates to touch screen coordinates.
	startX, startY := tcc.ConvertLocation(info.SwipeUp.SwipeStartLocation)
	endX, endY := tcc.ConvertLocation(info.SwipeUp.SwipeEndLocation)

	if err := stw.Swipe(ctx, startX, startY, endX, endY, 200*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to finish the gesture")
	}

	// Hotseat should be extended after gesture swipe.
	if err := WaitForHotseatAnimatingToIdealState(ctx, tconn, ShelfExtended); err != nil {
		return errors.Wrap(err, "failed to wait for the hoteat to be extended")
	}

	return nil
}

// EnterShelfOverflow pins enough shelf icons to enter overflow mode.
func EnterShelfOverflow(ctx context.Context, tconn *chrome.TestConn) error {
	// Number of pinned apps in each round of loop.
	const batchNumber = 10

	// Total amount of pinned apps.
	sum := 0

	installedApps, err := ChromeApps(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the list of the installed apps")
	}

	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the display info")
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := FetchScrollableShelfInfoForState(ctx, tconn, &ShelfState{})
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the scrollable shelf info"))
		}

		// Finish if at least one icon's position is out of the screen. Do not use
		// IsOverflow property here, there's a timing issue that it is overflowing
		// at this point but gets re-layouted without overflow soon later.
		// See also https://crbug.com/1105619.
		if len(info.IconsBoundsInScreen) == 0 {
			return errors.New("no icons found")
		}
		lastIconBounds := info.IconsBoundsInScreen[len(info.IconsBoundsInScreen)-1]
		if lastIconBounds.Right() > displayInfo.Bounds.Right() &&
			(info.LeftArrowBounds.Size().Width > 0 || info.RightArrowBounds.Size().Width > 0) {
			return nil
		}

		sum += batchNumber
		if sum > len(installedApps) {
			return testing.PollBreak(errors.Errorf("got %d apps, want at least %d apps", len(installedApps), sum))
		}

		for _, app := range installedApps[sum-batchNumber : sum] {
			if err := PinApp(ctx, tconn, app.AppID); err != nil {
				return testing.PollBreak(errors.Wrapf(err, "failed to pin app %s", app.AppID))
			}
		}
		return errors.New("still not overflow")
	}, nil)
}

// LaunchAppFromShelf opens an app by name which is currently pinned to the shelf.
// The parameter appName should be the name of the app which is same as the value stored in apps.App.Name.
func LaunchAppFromShelf(ctx context.Context, tconn *chrome.TestConn, appName, appID string) error {
	if err := ShowHotseat(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to launch app from shelf")
	}
	params := ui.FindParams{Name: appName, ClassName: "ash/ShelfAppButton"}
	icon, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find app %q", appName)
	}
	defer icon.Release(ctx)
	// Click mouse to launch app.
	if err := icon.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to launch app %q", appName)
	}
	// Make sure app is launched.
	if err := WaitForApp(ctx, tconn, appID); err != nil {
		return errors.Wrap(err, "failed to wait for the app to be launched")
	}
	return nil
}

// ShowHotseat make sure hotseat is shown in tablet mode.
func ShowHotseat(ctx context.Context, tconn *chrome.TestConn) error {
	if tabletMode, err := TabletModeEnabled(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to check if DUT is in tablet mode")
	} else if !tabletMode {
		return nil
	}
	// Get touch controller for tablet.
	tc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create the touch controller")
	}
	defer tc.Close()
	stw := tc.EventWriter()
	tcc := tc.TouchCoordConverter()

	// Make sure hotseat is shown.
	if err := SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	return nil
}

// PinAppFromShelf pins an open app on the shelf using the context menu.
// The parameter appName should be the name of the app which is same as the value stored in apps.App.Name.
func PinAppFromShelf(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	// Find the icon from shelf.
	params := ui.FindParams{Name: appName, ClassName: "ash/ShelfAppButton"}
	icon, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find app %q", appName)
	}
	defer icon.Release(ctx)

	// Open context menu.
	if err := icon.RightClick(ctx); err != nil {
		return errors.Wrap(err, "failed to open context menu")
	}
	// Find option to pin app to shelf.
	params = ui.FindParams{Name: "Pin", ClassName: "MenuItemView"}
	option, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		// The pin to shelf is not available for this icon
		return errors.Wrap(err, `option "Pin" is not available`)
	}
	defer option.Release(ctx)
	// Pin app to shelf.
	if err := option.LeftClick(ctx); err != nil {
		return errors.Wrap(err, `failed to select option "Pin"`)
	}
	// Make sure all items on the shelf are done moving.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location change to be completed")
	}
	return nil
}

// PinAppFromHotseat pins an open app on the hotseat using the context menu.
// The parameter appName should be the name of the app which is same as the value stored in apps.App.Name.
func PinAppFromHotseat(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	// Get touch controller for tablet.
	tc, err := pointer.NewTouchController(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create the touch controller")
	}
	defer tc.Close()
	stw := tc.EventWriter()
	tcc := tc.TouchCoordConverter()

	// Make sure hotseat is shown.
	if err := SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	// Find the icon from hotseat.
	params := ui.FindParams{Name: appName, ClassName: "ash/ShelfAppButton"}
	icon, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find app %q", appName)
	}
	defer icon.Release(ctx)

	// Open context menu.
	x, y := tcc.ConvertLocation(icon.Location.CenterPoint())
	if err := stw.LongPressAt(ctx, x, y); err != nil {
		return errors.Wrapf(err, "failed to long press icon %q", appName)
	}
	if err := stw.End(); err != nil {
		return errors.Wrapf(err, "failed to release pressed icon %q", appName)
	}

	// Find option to pin app to hotseat.
	params = ui.FindParams{Name: "Pin", ClassName: "MenuItemView"}
	option, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		// The pin option is not available for this icon
		return errors.Wrap(err, `option "Pin" is not available`)
	}
	defer option.Release(ctx)

	// Pin app to hotseat.
	x, y = tcc.ConvertLocation(option.Location.CenterPoint())
	if err := stw.Move(x, y); err != nil {
		return errors.Wrap(err, "failed to press option Pin")
	}
	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to release pressed option Pin")
	}
	// Make sure all items on the hotseat are done moving.
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location change to be completed")
	}
	return nil
}

// WaitForHotseatAnimationToFinish waits for the hotseat animation is done.
func WaitForHotseatAnimationToFinish(ctx context.Context, tc *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := fetchShelfInfoForState(ctx, tc, &ShelfState{})
		if err != nil {
			return err
		}
		hotseatInfo := info.HotseatInfo
		if hotseatInfo.IsAnimating || info.ScrollableShelfInfo.IsAnimating {
			return errors.New("hotseat is animating")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for the hotseat animation to finish")
	}

	return nil
}

// WaitForStableShelfBounds waits for the shelf location to be the same for a single iteration of polling.
func WaitForStableShelfBounds(ctx context.Context, tc *chrome.TestConn) error {
	// The shelf info does not provide the shelf bounds themselves, but shelf icon bounds can be used as
	// a proxy for the shelf position.
	var firstIconBounds, lastIconBounds coords.Rect
	var newFirstIconBounds, newLastIconBounds coords.Rect

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		shelfInfo, err := fetchShelfInfoForState(ctx, tc, &ShelfState{})
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to fetch scrollable shelf info"))
		}

		info := shelfInfo.ScrollableShelfInfo
		if len(info.IconsBoundsInScreen) == 0 {
			return testing.PollBreak(errors.New("no icons found"))
		}

		newFirstIconBounds = *info.IconsBoundsInScreen[0]
		newLastIconBounds = *info.IconsBoundsInScreen[len(info.IconsBoundsInScreen)-1]

		if firstIconBounds != newFirstIconBounds || lastIconBounds != newLastIconBounds {
			firstIconBounds = newFirstIconBounds
			lastIconBounds = newLastIconBounds
			return errors.New("Shelf bounds location still changing")
		}

		if info.IsAnimating || info.IsShelfWidgetAnimating {
			return errors.New("Shelf is animating")
		}

		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "Shelf bounds unstable")
	}

	return nil
}
