// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ash implements a library used for communication with Chrome Ash.
package ash

import (
	"context"
	"fmt"
	"sort"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
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

	// shelfIconClassName is the class name of the node of the apps on shelf.
	shelfIconClassName = "ash/ShelfAppButton"
)

// SetShelfBehavior sets the shelf visibility behavior.
// displayID is the display that contains the shelf.
func SetShelfBehavior(ctx context.Context, tconn *chrome.TestConn, displayID string, b ShelfBehavior) error {
	return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setShelfAutoHideBehavior)", displayID, b)
}

// GetShelfBehavior returns the shelf visibility behavior.
// displayID is the display that contains the shelf.
func GetShelfBehavior(ctx context.Context, tconn *chrome.TestConn, displayID string) (ShelfBehavior, error) {
	var b ShelfBehavior
	if err := tconn.Call(ctx, &b, "tast.promisify(chrome.autotestPrivate.getShelfAutoHideBehavior)", displayID); err != nil {
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
	params := nodewith.Role(role.Toolbar).ClassName("ShelfView")
	if err := uiauto.New(tconn).WithTimeout(timeout).WaitUntilExists(params)(ctx); err != nil {
		return errors.Wrap(err, "shelf not found")
	}
	return nil
}

// PinApp pins the shelf icon for the app specified by appID.
// Deprecated. Use PinApps() instead.
func PinApp(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	return PinApps(ctx, tconn, []string{appID})
}

// ShelfIconPinUpdateParam is defined in autotest_private.idl.
type ShelfIconPinUpdateParam struct {
	AppID     string `json:"appId"`
	ShouldPin bool   `json:"pinned"`
}

// GetPinnedAppIds returns the ids of the pinned apps. Note that the browser shortcut is not among the return value because it is always pinned to shelf.
func GetPinnedAppIds(ctx context.Context, tconn *chrome.TestConn) ([]string, error) {
	shelfItems, err := ShelfItems(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pinned app ids")
	}

	var pinnedApps []string
	for _, shelfItem := range shelfItems {
		if shelfItem.Type == ShelfItemTypePinnedApp {
			pinnedApps = append(pinnedApps, shelfItem.AppID)
		}
	}

	return pinnedApps, nil
}

// ResetShelfPinState returns a callback to reset shelf app pin states to default. The callback should be run before the test ends. This function should be called before any change in shelf pin states.
func ResetShelfPinState(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context) error, error) {
	defaultPinnedApps, err := GetPinnedAppIds(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get default pinned apps")
	}

	return func(ctx context.Context) error {
		currentPinnedApps, err := GetPinnedAppIds(ctx, tconn)

		if err != nil {
			return errors.Wrap(err, "failed to get current pinned apps")
		}

		// Indicates the apps which were pinned initially but are unpinned now. We are going to pin these apps.
		var toPinApps []string

		// Indicates the apps which were unpinned initially but are pinned now. We are going to unpin these apps.
		var toUnpinApps []string

		// Calculate the apps to pin and unpin by searching pairs in sorted arrays.
		sort.Strings(defaultPinnedApps)
		sort.Strings(currentPinnedApps)
		defaultArrayIdx := 0
		currentArrayIdx := 0
		for defaultArrayIdx < len(defaultPinnedApps) && currentArrayIdx < len(currentPinnedApps) {
			defaultApp := defaultPinnedApps[defaultArrayIdx]
			currentApp := currentPinnedApps[currentArrayIdx]
			if defaultApp == currentApp {
				// Find a pair.
				defaultArrayIdx++
				currentArrayIdx++
			} else if defaultApp < currentApp {
				// |defaultApp| cannot be found among |currentPinnedApps| so it should be pinned.
				toPinApps = append(toPinApps, defaultApp)
				defaultArrayIdx++
			} else {
				// |currentApp| cannot be found among |defaultPinnedApps| so it should be unpinned.
				toUnpinApps = append(toUnpinApps, currentApp)
				currentArrayIdx++
			}
		}

		// Pin the remaining apps in |defaultPinnedApps|.
		for defaultArrayIdx < len(defaultPinnedApps) {
			toPinApps = append(toPinApps, defaultPinnedApps[defaultArrayIdx])
			defaultArrayIdx++
		}

		// Unpin the remaining apps in |currentPinnedApps|.
		for currentArrayIdx < len(currentPinnedApps) {
			toUnpinApps = append(toUnpinApps, currentPinnedApps[currentArrayIdx])
			currentArrayIdx++
		}

		if err := PinAndUnpinApps(ctx, tconn, toPinApps, toUnpinApps); err != nil {
			return err
		}

		return nil
	}, nil
}

func setPinState(ctx context.Context, tconn *chrome.TestConn, updateParams []ShelfIconPinUpdateParam) error {
	err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setShelfIconPin)", updateParams)
	return err
}

// PinAndUnpinApps pins and unpins the apps specified by appIDs to shelf.
func PinAndUnpinApps(ctx context.Context, tconn *chrome.TestConn, appIDsToPin, appIDsToUnpin []string) error {
	var params []ShelfIconPinUpdateParam
	for _, appID := range appIDsToPin {
		params = append(params, ShelfIconPinUpdateParam{appID, true})
	}
	for _, appID := range appIDsToUnpin {
		params = append(params, ShelfIconPinUpdateParam{appID, false})
	}

	return setPinState(ctx, tconn, params)
}

// PinApps pins the apps specified by appIDs to shelf.
func PinApps(ctx context.Context, tconn *chrome.TestConn, appIDs []string) error {
	return PinAndUnpinApps(ctx, tconn, appIDs, []string{})
}

// UnpinApps unpins the apps specified by appIDs to shelf.
func UnpinApps(ctx context.Context, tconn *chrome.TestConn, appIDs []string) error {
	return PinAndUnpinApps(ctx, tconn, []string{}, appIDs)
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
	return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.setShelfAlignment)", displayID, a)
}

// GetShelfAlignment returns the shelf alignment.
// displayID is the display that contains the shelf.
func GetShelfAlignment(ctx context.Context, tconn *chrome.TestConn, displayID string) (ShelfAlignment, error) {
	var a ShelfAlignment
	if err := tconn.Call(ctx, &a, "tast.promisify(chrome.autotestPrivate.getShelfAlignment)", displayID); err != nil {
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
	IconsUnderAnimation    bool           `json:"iconsUnderAnimation"`
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
	Arc               AppType = "Arc"
	BuiltIn           AppType = "BuiltIn"
	Crostini          AppType = "Crostini"
	Extension         AppType = "Extension"
	StandaloneBrowser AppType = "StandaloneBrowser"
	Web               AppType = "Web"
	MacNative         AppType = "MacNative"
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
	if err := tconn.Call(ctx, &s, "tast.promisify(chrome.autotestPrivate.getAllInstalledApps)"); err != nil {
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
	if err := tconn.Call(ctx, &s, "tast.promisify(chrome.autotestPrivate.getShelfItems)"); err != nil {
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

// WaitUntilShelfIconAnimationFinish waits for the shelf icon animation to finish.
func WaitUntilShelfIconAnimationFinish(ctx context.Context, tconn *chrome.TestConn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := FetchScrollableShelfInfoForState(ctx, tconn, &ShelfState{})
		if err != nil {
			return errors.Wrap(err, "failed to fetch scrollable shelf's information")
		}
		if info.IconsUnderAnimation {
			return errors.New("unexpected shelf icon animation status: got true; want false")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait shelf icon animation to be idle")
	}

	return nil
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
	if err := tconn.Call(ctx, &appShown, "tast.promisify(chrome.autotestPrivate.isAppShown)", appID); err != nil {
		return false, errors.Wrapf(err, "failed to run autotestPrivate.isAppShown for %q", appID)
	}
	return appShown, nil
}

// WaitForApp waits for the app specified by appID to appear in the shelf.
func WaitForApp(ctx context.Context, tconn *chrome.TestConn, appID string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if visible, err := AppShown(ctx, tconn, appID); err != nil {
			return testing.PollBreak(err)
		} else if !visible {
			return errors.New("app is not shown yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
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

// VerifyShelfIconIndices checks whether the apps are ordered as expected.
func VerifyShelfIconIndices(ctx context.Context, tconn *chrome.TestConn, expectedApps []string) error {
	items, err := ShelfItems(ctx, tconn)

	if err != nil {
		return errors.Wrap(err, "failed to get shelf items")
	}

	for index, item := range items {
		if expectedApps[index] != item.AppID {
			return errors.Errorf("unexpected icon at the index(%d) on the shelf: got %s; want %s", index, item.AppID, expectedApps[index])
		}
	}

	return nil
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
	params := nodewith.Name(appName).ClassName(shelfIconClassName)
	if err := uiauto.New(tconn).WithTimeout(10 * time.Second).LeftClick(params)(ctx); err != nil {
		return errors.Wrapf(err, "failed to launch app %q", appName)
	}
	// Make sure app is launched.
	if err := WaitForApp(ctx, tconn, appID, time.Minute); err != nil {
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
	tsew, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to access the touchscreen")
	}
	defer tsew.Close()
	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to create the single touch writer")
	}

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
	icon := nodewith.Name(appName).ClassName(shelfIconClassName)
	option := nodewith.Name("Pin").ClassName("MenuItemView")
	ac := uiauto.New(tconn)
	return uiauto.Combine(
		"click icon and then click pin",
		ac.RightClick(icon),
		ac.LeftClick(option),
	)(ctx)
}

// PinAppFromHotseat pins an open app on the hotseat using the context menu.
// The parameter appName should be the name of the app which is same as the value stored in apps.App.Name.
func PinAppFromHotseat(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	// Get touch controller for tablet.
	tsew, tcc, err := touch.NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to access the touchscreen")
	}
	defer tsew.Close()
	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to create the single touch writer")
	}

	// Make sure hotseat is shown.
	if err := SwipeUpHotseatAndWaitForCompletion(ctx, tconn, stw, tcc); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	tc, err := touch.New(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to initialize the touch context")
	}

	return uiauto.Combine(
		"open the menu and tap the pin menu",
		tc.LongPress(nodewith.Name(appName).ClassName(shelfIconClassName)),
		tc.Tap(nodewith.Name("Pin").ClassName("MenuItemView")),
	)(ctx)
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

// ShowHotseatAction returns a function that makes sure hotseat is shown in tablet mode.
func ShowHotseatAction(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		return ShowHotseat(ctx, tconn)
	}
}

// RightClickApp returns a function that right clicks the given app's icon on the shelf.
func RightClickApp(tconn *chrome.TestConn, appName string) uiauto.Action {
	appOnShelf := nodewith.Name(appName).Role(role.Button).ClassName(shelfIconClassName)
	return uiauto.Combine(fmt.Sprintf("right click %s icon on the shelf", appName),
		ShowHotseatAction(tconn),
		uiauto.New(tconn).RightClick(appOnShelf))
}
