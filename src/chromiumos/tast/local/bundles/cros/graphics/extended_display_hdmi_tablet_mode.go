// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtendedDisplayHDMITabletMode,
		Desc:         "Verifies graphics composition on extended display in tabletmode",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
	})
}

func ExtendedDisplayHDMITabletMode(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to enable tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	const (
		settingsDeviceText  = "Device"
		settingsDisplayText = "Displays"
	)

	var (
		resolutionMenuParams  = nodewith.Name("Resolution").Role(role.PopUpButton)
		refreshRateMenuParams = nodewith.Name("Refresh Rate Menu").Role(role.PopUpButton)
		resolution4kParams    = nodewith.Name("3840 x 2160").Role(role.ListBoxOption).First()
		refreshRate60HzParam  = nodewith.Name("60 Hz").Role(role.ListBoxOption).First()
		builtinDisplayParams  = nodewith.Name("Mirror Built-in display").Role(role.CheckBox)
	)

	if err := externalMonitorDetection(ctx, 1); err != nil {
		s.Fatal("Failed to detect extended display monitor: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		capps, err := ash.ChromeApps(ctx, tconn)
		if err != nil {
			testing.PollBreak(err)
		}
		for _, app := range capps {
			if app.AppID == apps.Settings.ID {
				return nil
			}
		}
		return errors.New("Settings app not yet found in available Chrome apps")
	}, nil); err != nil {
		s.Fatal("Failed to find the settings app in the available Chrome apps: ", err)
	}

	cui := uiauto.New(tconn)
	leftClickUIElement := func(elementFinder *nodewith.Finder) error {
		if err := cui.LeftClick(elementFinder)(ctx); err != nil {
			return errors.Wrapf(err, "failed to left click %v with error", elementFinder)
		}
		return nil
	}

	// Launch the Settings app and wait for it to open.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the Settings app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, 5*time.Second); err != nil {
		s.Fatal("Failed to appear settings app in the shelf: ", err)
	}

	if err := settingsPage(ctx, tconn, ui.RoleTypeLink, settingsDeviceText, ui.RoleTypeInlineTextBox, true); err != nil {
		s.Fatalf("Failed to click on %q option in settings page: %v", settingsDeviceText, err)
	}

	if err := settingsPage(ctx, tconn, ui.RoleTypeLink, settingsDisplayText, "", false); err != nil {
		s.Fatalf("Failed to click on %q option in Device page: %v", settingsDisplayText, err)
	}

	if err := leftClickUIElement(builtinDisplayParams); err != nil {
		s.Fatal("Failed to find and click built-in display checkbox with error: ", err)
	}

	var displayName string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		displayInfo, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get external display name")
		}
		if len(displayInfo) < 2 {
			return errors.New("failed please connect external 4K monitor to DUT")
		}
		displayName = displayInfo[1].Name
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		s.Fatal("Failed to get external display info: ", err)
	}

	if err := settingsPage(ctx, tconn, ui.RoleTypeTab, displayName, "", false); err != nil {
		s.Fatalf("Failed to click on 'External Display :%v' option in Display page: %v", displayName, err)
	}

	// Check if the 4k resolution @3840 x 2160 and refresh rate @60HZ is getting listed in the drop down menu.
	if err := leftClickUIElement(resolutionMenuParams); err != nil {
		s.Fatal("Failed to find and click resolution menu with error: ", err)
	}

	if err := leftClickUIElement(resolution4kParams); err != nil {
		s.Fatal("Failed to find and click resolution '3840 x 2160' with error: ", err)
	}

	if err := leftClickUIElement(refreshRateMenuParams); err != nil {
		s.Fatal("Failed to find and click refresh rate menu with error: ", err)
	}

	if err := leftClickUIElement(refreshRate60HzParam); err != nil {
		s.Fatal("Failed to find and click '60 Hz' with error: ", err)
	}
}

func settingsPage(ctx context.Context, tconn *chrome.TestConn, parent ui.RoleType, name string, descendant ui.RoleType, flag bool) error {
	var (
		params = ui.FindParams{
			Role: parent,
			Name: name,
		}
		defaultStablePollOpts = testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 5 * time.Second}
	)
	element, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed waiting to find %s: ", name)
	}
	if flag {
		element, err = element.DescendantWithTimeout(ctx, ui.FindParams{Role: descendant}, 10*time.Second)
		if err != nil {
			return errors.Wrapf(err, "failed waiting to find %s", name)
		}
	}
	defer element.Release(ctx)
	if err := element.StableLeftClick(ctx, &defaultStablePollOpts); err != nil {
		return errors.Errorf("failed to click the %s : %s", name, err)
	}
	return nil
}

func externalMonitorDetection(ctx context.Context, numberOfDisplays int) error {
	const DisplayInfoFile = "/sys/kernel/debug/dri/0/i915_display_info"
	// This regexp will skip pipe A since that's the internal display detection.
	displayInfo := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "cat", DisplayInfoFile).Output()

		if err != nil {
			return errors.Wrap(err, "failed to run display info command")
		}
		matchedString := displayInfo.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "please connect external display as required")
	}
	return nil
}
