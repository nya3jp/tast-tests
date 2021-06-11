// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemTrayPerf,
		Desc:         "Measures animation smoothness of system tray animations",
		Contacts:     []string{"amehfooz@chromium.org", "leandre@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func SystemTrayPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Add some notifications so that notification centre shows up when opening Quick Settings.
	const uiTimeout = 30 * time.Second
	n := 5
	for i := 0; i <= n; i++ {
		if _, err := ash.CreateTestNotification(ctx, tconn, ash.NotificationTypeBasic, "TestBasicNotification"+strconv.Itoa(i), "test message"); err != nil {
			s.Fatal("Failed to create test basic notification: ", err)
		}
		if _, err := ash.CreateTestNotification(ctx, tconn, ash.NotificationTypeImage, "TestImageNotification"+strconv.Itoa(i), "test message"); err != nil {
			s.Fatal("Failed to create test image notification: ", err)
		}
		if _, err := ash.CreateTestNotification(ctx, tconn, ash.NotificationTypeProgress, "TestProgressNotification"+strconv.Itoa(i), "test message"); err != nil {
			s.Fatal("Failed to create test progress notification: ", err)
		}
		if _, err := ash.CreateTestNotification(ctx, tconn, ash.NotificationTypeList, "TestListNotification"+strconv.Itoa(i), "test message"); err != nil {
			s.Fatal("Failed to create test list notification: ", err)
		}
	}

	// Wait for the last notification to finish creating.
	if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle("TestListNotification"+strconv.Itoa(n))); err != nil {
		s.Fatal("Failed waiting for notification: ", err)
	}

	ac := uiauto.New(tconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	collapseButton := nodewith.ClassName("CollapseButton")
	runner := perfutil.NewRunner(cr)

	// Toggle the Status Area button to record input latency of showing Quick Settings and notification centre.
	runner.RunMultiple(ctx, s, "open", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := uiauto.Combine(
			"open the uber tray, then click again to close it",
			ac.LeftClick(statusArea),
			ac.WaitUntilExists(collapseButton),
			ac.LeftClick(statusArea),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to open the uber tray then close it")
		}
		return nil
	},
		"Ash.StatusAreaShowBubble.PresentationTime",
		"Ash.StatusAreaShowBubble.PresentationTime.MaxLatency"),
		perfutil.StoreLatency)

	// Opens the Ubertray again.
	if err := uiauto.Combine(
		"open the uber tray",
		ac.LeftClick(statusArea),
		ac.WaitUntilExists(collapseButton),
	)(ctx); err != nil {
		s.Fatal("Failed to open the uber tray: ", err)
	}

	// Toggle the collapsed state of the system tray.
	runner.RunMultiple(ctx, s, "collapse", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		return uiauto.Combine(
			"collapse and expand",
			ac.LeftClick(collapseButton),
			ac.WaitForLocation(collapseButton),
			ac.LeftClick(collapseButton),
			ac.WaitForLocation(collapseButton),
		)(ctx)
	},
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToCollapsed",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded"),
		perfutil.StoreSmoothness)

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
