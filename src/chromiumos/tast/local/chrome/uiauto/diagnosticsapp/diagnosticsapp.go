// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package diagnosticsapp contains drivers for controlling the ui of diagnostics SWA.
package diagnosticsapp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// KeyState defines keyboard tester's key state.
type KeyState string

// TestParams defines parameters that can be set in test to describe how it
// should run.
type TestParams struct {
	// Represents if device has default narrow view for navigation.
	IsNarrowDevice bool
}

const (
	// These strings come from IDS_KEYBOARD_DIAGRAM_ARIA_LABEL_NOT_PRESSED,
	// IDS_KEYBOARD_DIAGRAM_ARIA_LABEL_PRESSED
	// and IDS_KEYBOARD_DIAGRAM_ARIA_LABEL_TESTED in chromeos/chromeos_strings.grd.

	// KeyNotPressed is used to verify the key is in the not pressed state.
	KeyNotPressed KeyState = "key not pressed"

	// KeyPressed is used to verify the key is in the pressed state.
	KeyPressed KeyState = "key pressed"

	// KeyTested is used to verify the key is in the tested state.
	KeyTested KeyState = "key tested"
)

// SkipNarrowPlatformsHwdeps test parameter configuration for skipping
// narrow devices.
var SkipNarrowPlatformsHwdeps = hwdep.D(
	// "gru" is the platform name for scarlet devices.
	hwdep.SkipOnPlatform("gru"))

// NarrowPlatformsHwdeps test parameter configuration for including
// narrow devices.
var NarrowPlatformsHwdeps = hwdep.D(
	// "gru" is the platform name for scarlet devices. Scarlet
	// needs to be treated differently to handle mobile navigation.
	hwdep.Platform("gru"))

var (
	// diagnosticsRootNodeParams export is used to find the root node of diagnostics.
	diagnosticsRootNodeParams = nodewith.Name(apps.Diagnostics.Name).Role(role.Window)

	// DxLogButton export is used to find session log button.
	DxLogButton = nodewith.ClassName("session-log-button").Role(role.Button)

	// DxMemoryTestButton export is used to find routine test button.
	DxMemoryTestButton = nodewith.Name("Run Memory test").Role(role.Button)

	// DxCPUTestButton export is used to find routine test button.
	DxCPUTestButton = nodewith.Name("Run CPU test").Role(role.Button)

	// DxViewReportButton export is used to find the see report button.
	DxViewReportButton = nodewith.Name("See Report").Role(role.Button)

	// DxCancelTestButton export is used to find routine test cancel button.
	DxCancelTestButton = nodewith.Name("Stop test").Role(role.Button)

	// DxCPUChart export is used to find the realtime cpu chart.
	DxCPUChart = nodewith.ClassName("legend-group").Role(role.GenericContainer)

	// DxPassedBadge export is used to find success badge notification.
	DxPassedBadge = nodewith.Name("PASSED").Role(role.StaticText)

	// DxProgressBadge export is used to find successful launch of a routine.
	DxProgressBadge = nodewith.Name("RUNNING").Role(role.StaticText)

	// DxCancelledBadge export is used to find cancelled badge.
	DxCancelledBadge = nodewith.Name("STOPPED").Role(role.StaticText)

	// DxConnectivity export is used to find the Connectivity navigation item.
	DxConnectivity = nodewith.Name("Connectivity").Role(role.GenericContainer)

	// DxNetworkList export is used to find the network list.
	DxNetworkList = nodewith.ClassName("diagnostics-network-list-container").Role(role.GenericContainer)

	// DxInput export is used to find the Input navigation item.
	DxInput = nodewith.Name("Input").Role(role.GenericContainer)

	// DxInternalKeyboardTestButton used to find the internal keyboard test button on the input page.
	DxInternalKeyboardTestButton = nodewith.Name("Test").Role(role.Button).First()

	// DxKeyboardHeading export is used to find the keyboard heading on the input page.
	DxKeyboardHeading = nodewith.Name("Keyboard").Role(role.StaticText)

	// DxVirtualKeyboardHeading export is used to find the virtual keyboard heading on the input page.
	DxVirtualKeyboardHeading = nodewith.NameContaining("Tast virtual keyboard").Role(role.StaticText)

	// DxDefocusingMsg export is used to find the pop up message when input page isn't focused.
	DxDefocusingMsg = nodewith.Name("Keys aren't tested when you're using another window").Role(role.StaticText)

	// DxKeyboardTester export is used to find the keyboard tester on the input page.
	DxKeyboardTester = nodewith.HasClass("body-container").Role(role.GenericContainer)

	// DxNarrowMenuButton export is used to find the navigation menu on narrow views.
	DxNarrowMenuButton = nodewith.Name("Diagnostics").Role(role.Button).First()

	defaultPolling = testing.PollOptions{Interval: time.Second, Timeout: 20 * time.Second}
)

// DiagnosticsRootNode returns the root ui node of Diagnotsics app.
func DiagnosticsRootNode(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	ui := uiauto.New(tconn)
	err := ui.WithTimeout(20 * time.Second).WaitUntilExists(diagnosticsRootNodeParams)(ctx)
	return diagnosticsRootNodeParams, err
}

// Launch diagnostics via default method and finder and error.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	err := apps.Launch(ctx, tconn, apps.Diagnostics.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch diagnostics app")
	}

	err = ash.WaitForApp(ctx, tconn, apps.Diagnostics.ID, time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "diagnostics app did not appear in shelf after launch")
	}

	dxRootNode, err := DiagnosticsRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find diagnostics app")
	}
	return dxRootNode, nil
}

// Close closes the diagnostics app.
func Close(ctx context.Context, tconn *chrome.TestConn) error {
	return apps.Close(ctx, tconn, apps.Diagnostics.ID)
}

// KeyNodeFinder creates a Finder with a name containing the key name and key state.
func KeyNodeFinder(key string, state KeyState) *nodewith.Finder {
	return nodewith.Name(fmt.Sprintf("%s %s", key, state)).Role(role.GenericContainer)
}

// CheckGlyphsbyRegion verifies several regional keys for a certain region.
func CheckGlyphsbyRegion(ui *uiauto.Context, regionCode string) action.Action {
	return func(ctx context.Context) error {
		for _, keyName := range regionalKeys[regionCode] {
			if err := ui.WaitUntilExists(nodewith.NameContaining(keyName).First())(ctx); err != nil {
				return errors.Wrapf(err, "failed to find regional key %v in region code %v", keyName, regionCode)
			}
			testing.ContextLogf(ctx, "Region %v regional key %v found", regionCode, keyName)
		}
		return nil
	}
}

// WaitUntilColorModeNudgeGoneIfExists will get rid of the color theme nudge
// that appears the first time launching chrome. This method will find and
// wait for nudge to go away as there is no dismiss button.
func WaitUntilColorModeNudgeGoneIfExists(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	nudge := nodewith.NameContaining("Switch between dark and light theme").First()
	nudgeFound, err := ui.IsNodeFound(ctx, nudge)
	if err != nil {
		return errors.Wrap(err, "failed to search for nudge")
	}

	if nudgeFound {
		if err := ui.WithTimeout(45 * time.Second).WaitUntilGone(nudge)(ctx); err != nil {
			return errors.Wrap(err, "nudge not dismissing")
		}
	}
	return nil
}

// ClickNavigationMenuButton will click the hamburger menu button in the
// Diagnostics app to toggle the navigation view on narrow views.
func ClickNavigationMenuButton(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	dxRootNode, err := DiagnosticsRootNode(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find diagnostics app")
	}

	// Get menu node under Diagnostics app and click it.
	menuButton := DxNarrowMenuButton.Ancestor(dxRootNode)
	if err := uiauto.Combine("find and click menu button",
		ui.WithTimeout(20*time.Second).WaitUntilExists(menuButton),
		ui.FocusAndWait(menuButton),
		ui.WithPollOpts(defaultPolling).LeftClick(menuButton),
	)(ctx); err != nil {
		return errors.Wrap(err, "menu click failed")
	}
	return nil
}
