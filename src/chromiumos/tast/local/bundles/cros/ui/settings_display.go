// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const uiTime = 3 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsDisplay,
		Desc:         "Open OS Settings and check main sections are displayed properly",
		Contacts:     []string{"tim.chang@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
	})
}

func SettingsDisplay(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "settings_display")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	ui := uiauto.New(tconn)

	// Open OS settings from system tray by UI operation instead of using ossettings.Launch().
	systemTray := nodewith.HasClass("UnifiedSystemTray")
	settingsIcon := nodewith.Name("Settings").HasClass("TopShortcutButton")

	if err := uiauto.Combine("open settings from the system tray",
		ui.LeftClickUntil(systemTray, ui.WithTimeout(uiTime).WaitUntilExists(settingsIcon)),
		ui.LeftClickUntil(settingsIcon, ui.WithTimeout(uiTime).WaitUntilExists(ossettings.SearchBoxFinder)),
		ui.WaitUntilExists(ossettings.SearchBoxFinder.Focused()),
	)(ctx); err != nil {
		s.Fatal("Failed to open OS settings and check its state: ", err)
	}

	if err := goThroughMainSections(ctx, tconn, ui); err != nil {
		s.Fatal("Failed to go through each main sections: ", err)
	}
}

func goThroughMainSections(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {

	const (
		advanced             = "Advanced"
		item                 = "item"
		title                = "title"
		addNetworkConnection = "Add network connection"
		use24HourClock       = "Use 24-hour clock"

		centeredContainer = "cr-centered-card-container"
	)

	advancedNode := nodewith.Name(advanced).Role(role.Button)
	mainSections := nodewith.HasClass(item).Role(role.Link)
	if err := uiauto.Combine("expand advanced settings",
		ui.LeftClickUntil(
			advancedNode.First(),
			ui.WithTimeout(uiTime).WaitUntilExists(advancedNode.First().State(state.Focused, true)),
		),
		ui.WaitUntilExists(mainSections.First()),
	)(ctx); err != nil {
		return err
	}

	settingsNodes, err := ui.NodesInfo(ctx, mainSections)
	if err != nil {
		return errors.Wrap(err, "failed to get all of settings sections from navigation")
	}

	settingsCenteredView := nodewith.HasClass(centeredContainer).Role(role.Main)
	titleOfSection := nodewith.HasClass(title).Role(role.Heading).Ancestor(settingsCenteredView)

	for _, node := range settingsNodes {
		if err := ui.LeftClickUntil(
			mainSections.Name(node.Name).First(),
			ui.WithTimeout(uiTime).WaitUntilExists(titleOfSection.Name(node.Name)),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to click each basic sections and check they are displayed properly")
		}

		switch node.Name {
		case "Apps":
			// Swipe navigation to go through advanced settings.
			if err := swipeLeftNavigation(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to swipt left navigation")
			}
		case "Network":
			// Expand the 'Add connection' button.
			expandableBtn := nodewith.Name(addNetworkConnection).Role(role.Button).State(state.Collapsed, true)
			if err := ui.LeftClickUntil(
				expandableBtn.First(),
				ui.WithTimeout(uiTime).WaitUntilExists(expandableBtn.State(state.Collapsed, false)),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to expand the collapsed button")
			}
		case "Date and time":
			// Switch toggle button to enable 24-hour clock.
			toggleBtn := nodewith.Name(use24HourClock).Role(role.ToggleButton)
			if err := ui.LeftClickUntil(
				toggleBtn.First(),
				ui.WithTimeout(uiTime).WaitUntilExists(toggleBtn.State(state.Focused, true)),
			)(ctx); err != nil {
				return errors.Wrap(err, "failed to switch the toggle button")
			}
		default:
			testing.ContextLogf(ctx, "Display settings node %q ", node.Name)
		}
	}
	return nil
}

func swipeLeftNavigation(ctx context.Context, tconn *chrome.TestConn) error {
	tpw, err := input.Trackpad(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a trackpad device")
	}
	defer tpw.Close()

	tew, err := tpw.NewMultiTouchWriter(2)
	if err != nil {
		return errors.Wrap(err, "failed to create a multi touch writer")
	}
	defer tew.Close()

	var (
		x      = tpw.Width() / 4
		ystart = tpw.Height() / 4
		yend   = tpw.Height() / 4 * 3
		d      = tpw.Width() / 8 // x-axis distance between two fingers.
	)

	if err := tew.DoubleSwipe(ctx, x, ystart, x, yend, d, time.Second); err != nil {
		return errors.Wrap(err, "failed to swipe left navigation of settings")
	}
	if err := tew.End(); err != nil {
		return errors.Wrap(err, "failed to end a touch")
	}
	return nil
}
