// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
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
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
	})
}

func SystemTrayPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	var initArcOpt []chrome.Option
	// Only enable arc if it's supoprted.
	if arc.Supported() {
		initArcOpt = []chrome.Option{chrome.ARCEnabled()}
	}

	cr, err := chrome.New(ctx, initArcOpt...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	const (
		apk = "ArcNotificationTest.apk"
		pkg = "org.chromium.arc.testapp.notification"
		cls = ".NotificationActivity"

		// UI IDs in the app.
		idPrefix = pkg + ":id/"
		idID     = idPrefix + "notification_id"
		sendID   = idPrefix + "send_button"
	)

	var d *ui.Device
	if arc.Supported() {
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		// Installing the testing app.
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatalf("Failed to install %s: %v", apk, err)
		}

		// Launching the testing app
		act, err := arc.NewActivity(a, pkg, cls)
		if err != nil {
			s.Fatal("Failed to create a new activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start the activity: ", err)
		}
		defer act.Stop(ctx, tconn)

		d, err = a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed to initialize UI Automator: ", err)
		}
		defer d.Close(ctx)
	}

	// Add some notifications so that notification centre shows up when opening Quick Settings.
	const uiTimeout = 30 * time.Second
	const n = 5
	ts := []ash.NotificationType{
		ash.NotificationTypeBasic,
		ash.NotificationTypeImage,
		ash.NotificationTypeProgress,
		ash.NotificationTypeList,
	}
	for i := 0; i <= n; i++ {
		for _, t := range ts {
			if _, err := ash.CreateTestNotification(ctx, tconn, t, fmt.Sprintf("Test%sNotification%d", t, i), "test message"); err != nil {
				s.Fatalf("Failed to create %d-th %s notification: %v", i, t, err)
			}
		}

		// Create an ARC notification using the testing app.
		if arc.Supported() {
			if err := d.Object(ui.ID(idID)).SetText(ctx, fmt.Sprintf("%d", i)); err != nil {
				s.Fatal("Failed to set message ID in the testing app: ", err)
			}
			if err := d.Object(ui.ID(sendID)).Click(ctx); err != nil {
				s.Fatalf("Failed to click %s button in the testing app: %v", sendID, err)
			}
		}
	}

	// Wait for the last notification to finish creating.
	if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(fmt.Sprintf("Test%sNotification%d", ts[len(ts)-1], n))); err != nil {
		s.Fatal("Failed waiting for notification: ", err)
	}

	ac := uiauto.New(tconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	collapseButton := nodewith.ClassName("CollapseButton")

	// This includes toggle the Status Area button to record input latency of showing Quick Settings/notification centre
	// and toggle the collapsed state of the system tray to record animation smoothness.
	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := uiauto.Combine(
			"open the uber tray, collapse and expand it, then close it",
			ac.LeftClick(statusArea),
			ac.WaitUntilExists(collapseButton),
			ac.LeftClick(collapseButton),
			ac.WaitForLocation(collapseButton),
			ac.LeftClick(collapseButton),
			ac.WaitForLocation(collapseButton),
			ac.LeftClick(statusArea),
			ac.WaitUntilGone(collapseButton),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to open the uber tray, collapse and expand, then close")
		}
		return nil
	},
		"Ash.StatusAreaShowBubble.PresentationTime",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToCollapsed",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded"),
		perfutil.StoreAllWithHeuristics(""))

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
