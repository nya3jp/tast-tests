// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/notification"
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
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val: false,
		}, {
			Name:              "arc",
			ExtraSoftwareDeps: []string{"arc"},
			Val:               true,
		}},
	})
}

func NotificationScrollingPerf(ctx context.Context, s *testing.State) {
	isArc := s.Param().(bool)

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	var initArcOpt []chrome.Option
	if isArc {
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

	var arcclient *notification.ARCClient
	if isArc {
		arcclient, err = notification.NewARCClient(ctx, tconn, cr, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARCClient: ", err)
		}
		defer arcclient.Close(ctx, tconn)
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
			title := fmt.Sprintf("Test%sNotification%d", t, i)
			if _, err := ash.CreateTestNotification(ctx, tconn, t, title, "test message"); err != nil {
				s.Fatalf("Failed to create %d-th %s notification: %v", i, t, err)
			}
			// Wait for the notification to finish creating, making sure that it is created.
			if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(title)); err != nil {
				s.Fatalf("Failed to wait for %d-th %s notification: %v", i, t, err)
			}
		}

		// Create an ARC notification.
		if isArc {
			if err := arcclient.CreateOrUpdateTestNotification(ctx, tconn, fmt.Sprintf("TestARCNotification%d", i), "test message", fmt.Sprintf("%d", i)); err != nil {
				s.Fatalf("Failed to create %d-th ARC notification: %v", i, err)
			}
		}
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
