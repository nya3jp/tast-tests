// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
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
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func NotificationScrollingPerf(ctx context.Context, s *testing.State) {
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
	}

	// Wait for the last notification to finish creating.
	if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(fmt.Sprintf("Test%sNotification%d", ts[len(ts)-1], n))); err != nil {
		s.Fatal("Failed waiting for notification: ", err)
	}

	// Variables used to scroll by touchpad.
	var (
		pad      *input.TrackpadEventWriter
		touchPad *input.TouchEventWriter
	)
	if pad, err = input.Trackpad(ctx); err != nil {
		s.Fatal("Failed to create trackpad event writer: ", err)
	}
	defer pad.Close()
	if touchPad, err = pad.NewMultiTouchWriter(2); err != nil {
		s.Fatal("Failed to create trackpad singletouch writer: ", err)
	}
	defer touchPad.Close()

	var (
		x      = pad.Width() / 2
		ystart = pad.Height() / 6
		yend   = pad.Height() / 6 * 5
		d      = pad.Width() / 8 // x-axis distance between two fingers.
	)
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
