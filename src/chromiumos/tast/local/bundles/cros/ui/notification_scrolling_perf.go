// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NotificationScrollingPerf,
		Desc:         "Measures input latency of scrolling through notification list",
		Contacts:     []string{"leandre@chromium.org", "cros-status-area-eng@google.com", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
	})
}

func NotificationScrollingPerf(ctx context.Context, s *testing.State) {
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

	var de *ui.Device
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

		de, err = a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed to initialize UI Automator: ", err)
		}
		defer de.Close(ctx)
	}

	// Add some notifications so that notification centre shows up when opening Quick Settings.
	// We will add 5 notifications of each type so that it is enough for scrolling.
	const n = 5
	const uiTimeout = 30 * time.Second
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
			if err := de.Object(ui.ID(idID)).SetText(ctx, fmt.Sprintf("%d", i)); err != nil {
				s.Fatal("Failed to set message ID in the testing app: ", err)
			}
			if err := de.Object(ui.ID(sendID)).Click(ctx); err != nil {
				s.Fatalf("Failed to click %s button in the testing app: %v", sendID, err)
			}
		}
	}

	// Wait for the last notification to finish creating.
	if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(fmt.Sprintf("Test%sNotification%d", ts[len(ts)-1], n))); err != nil {
		s.Fatal("Failed waiting for notification: ", err)
	}

	pad, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to create trackpad event writer: ", err)
	}
	defer pad.Close()
	touchPad, err := pad.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Failed to create trackpad singletouch writer: ", err)
	}
	defer touchPad.Close()

	x := pad.Width() / 2
	ystart := pad.Height() / 6
	yend := pad.Height() / 6 * 5
	d := pad.Width() / 8 // x-axis distance between two fingers.

	// Double swipe the touchpad for scroll up.
	swipeScrollUp := func(ctx context.Context) error {
		if err := touchPad.DoubleSwipe(ctx, x, yend, x, ystart, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe up")
		}
		if err := touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}
	// Double swipe the touchpad for scroll down.
	swipeScrollDown := func(ctx context.Context) error {
		if err := touchPad.DoubleSwipe(ctx, x, ystart, x, yend, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe down")
		}
		if err := touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	ac := uiauto.New(tconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	messageCenter := nodewith.ClassName("UnifiedMessageCenterView")

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := uiauto.Combine(
			"open the uber tray, scroll up and down the notification list, then close it",
			ac.LeftClick(statusArea),
			ac.WaitUntilExists(messageCenter),
			ac.MouseMoveTo(messageCenter, 0),
			swipeScrollUp,
			swipeScrollUp,
			swipeScrollDown,
			swipeScrollDown,
			ac.LeftClick(statusArea),
			ac.WaitUntilGone(messageCenter),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to open the uber tray, scroll the notification list, then close")
		}
		return nil
	},
		"Ash.MessageCenter.Scroll.PresentationTime",
		"Ash.MessageCenter.Scroll.PresentationTime.MaxLatency"),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
