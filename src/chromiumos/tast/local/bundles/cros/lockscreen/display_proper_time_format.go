// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisplayProperTimeFormat,
		Desc:         "Test display proper time format on the lock screen",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts:     []string{"sherrilin@google.com", "chromeos-sw-engprod@google.com", "cros-lurs@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "24_hour_format",
			Val:  true,
		}, {
			Name: "12_hour_format",
			Val:  false,
		}},
	})
}

func DisplayProperTimeFormat(ctx context.Context, s *testing.State) {
	isToggleOn := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	const (
		username = "testuser@gmail.com"
		password = "good"
		PIN      = "123456789012"
	)

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Open settings
	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to open OS settings: ", err)
	}
	osSettings := ossettings.New(tconn)
	defer osSettings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")
	if err := osSettings.WaitForSearchBox()(ctx); err != nil {
		s.Fatal("Failed to wait for OS-settings is ready to use: ", err)
	}

	// Open Advanced settings
	if err := uiauto.Combine("Scroll the Advanced button into view by focusing it, click it, and wait for it to be expanded",
		ensureVisible(osSettings, ossettings.Advanced),
		osSettings.WaitUntilExists(ossettings.Advanced.Onscreen()),
	)(ctx); err != nil {
		s.Fatal("Failed to ensure advanced settings' visibility: ", err)
	}
	if err := expandSubSection(osSettings, ossettings.Advanced, true)(ctx); err != nil {
		s.Fatal("Failed to expand advanced settings: ", err)
	}

	// Open Date and Time
	if err := uiauto.Combine("Scroll the Date and Time button into view by focusing it, click it, and wait for it to be shown",
		osSettings.FocusAndWait(ossettings.DateAndTime),
		osSettings.LeftClick(ossettings.DateAndTime),
		osSettings.WaitUntilExists(ossettings.DateAndTime.State(state.Focused, true)),
	)(ctx); err != nil {
		s.Fatal("Failed to enter Date and Time settings page: ", err)
	}
	if err := toggleSetting(cr, osSettings, "Use 24-hour clock", isToggleOn)(ctx); err != nil {
		s.Fatal("Failed to toggle Use-24-Hour-Clock: ", err)
	}

	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	// Ensure the status area is visible.
	ui := uiauto.New(tconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	if err := ui.WaitUntilExists(statusArea)(ctx); err != nil {
		s.Fatal("Failed to find status area widget: ", err)
	}

	// Verify time format
	TimeView := nodewith.ClassName("TimeView")
	s.Log(TimeView.Pretty())
	info, err := ui.Info(ctx, TimeView)
	if err != nil {
		s.Fatal("Failed to get node info for the time view: ", err)
	}
	s.Log(info.Name)
	if is24HourFormat(info.Name) != isToggleOn {
		s.Fatal("Wrong date time format")
	}

	// Unlock the screen to ensure subsequent tests aren't affected by the screen remaining locked.
	defer func() {
		if err := lockscreen.Unlock(ctx, tconn); err != nil {
			s.Fatal("Failed to unlock the screen: ", err)
		}
	}()
}

func is24HourFormat(timeString string) bool {
	// 12-hour format contains either AM or PM.
	return !strings.Contains(timeString, "AM") && !strings.Contains(timeString, "PM")
}

func toggleSetting(cr *chrome.Chrome, osSettings *ossettings.OSSettings, name string, isToggleOn bool) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("toggle setting: %s", name),
		osSettings.SetToggleOption(cr, name, isToggleOn),
	)
}

func expandSubSection(osSettings *ossettings.OSSettings, node *nodewith.Finder, expected bool) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("expand sub section: %s", node.Pretty()),
		ensureFocused(osSettings, node),
		osSettings.LeftClick(node.State(state.Expanded, !expected)),
		osSettings.WaitUntilExists(node.State(state.Expanded, expected)),
	)
}

func ensureFocused(osSettings *ossettings.OSSettings, node *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		info, err := osSettings.Info(ctx, node)
		if err != nil {
			return err
		}
		if info.State[state.Focused] {
			return nil
		}
		return osSettings.FocusAndWait(node)(ctx)
	}
}

func ensureVisible(osSettings *ossettings.OSSettings, node *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		if found, err := osSettings.IsNodeFound(ctx, nodewith.Role(role.Navigation).First()); err != nil {
			return errors.Wrap(err, "failed to try to find node")
		} else if !found {
			// The main menu might be collapsed depending on window size, expand the main menu to ensure the input node is visible.
			if err = osSettings.LeftClick(ossettings.MenuButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to click menu button")
			}
		}

		info, err := osSettings.Info(ctx, node)
		if err != nil {
			return err
		}
		if !info.State[state.Offscreen] {
			return nil
		}
		return osSettings.MakeVisible(node)(ctx)
	}
}
