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
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type notificationClearTestType string

const (
	clearOneAtATime notificationClearTestType = "ClearOneAtATime" // In test, clear notifications one at a time.
	clearAll        notificationClearTestType = "ClearAll"        // In test, clear all notifications at once
)

type notificationClearTestVal struct {
	TestType notificationClearTestType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         NotificationClearAllPerf,
		Desc:         "Measures animation performance of the clear all animation in the message center",
		Contacts:     []string{"newcomer@chromium.org", "cros-status-area-eng@google.com", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "one_at_a_time",
				Val:  clearOneAtATime,
			},
			{
				Name: "clear_all",
				Val:  clearAll,
			},
		},
	})
}
func NotificationClearAllPerf(ctx context.Context, s *testing.State) {
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
	// Create 12 notifications, 3 groups of 4 types of notifications. This is enough to force notification overflow to happen.
	const n = 3
	ts := []ash.NotificationType{
		ash.NotificationTypeBasic,
		ash.NotificationTypeImage,
		ash.NotificationTypeProgress,
		ash.NotificationTypeList,
	}

	ac := uiauto.New(tconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	collapseButton := nodewith.ClassName("CollapseButton")

	// Open the status area, then collapse the uber tray which results in an expanded MessageCenter.
	if err := uiauto.Combine(
		"open the uber tray, expand it, then close some notifications",
		ac.LeftClick(statusArea),
		ac.WaitUntilExists(collapseButton),
		ac.LeftClick(collapseButton),
		ac.WaitForLocation(collapseButton),
	)(ctx); err != nil {
		s.Fatal("Failed to collapse the uber tray: ", err)
	}

	// Ensure no notifications currently exist.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to clear all notifications prior to adding notifications")
	}

	var histogramName string
	var shouldClearOneAtATime = s.Param().(notificationClearTestType) == clearOneAtATime
	if shouldClearOneAtATime {
		histogramName = "Ash.Notification.MoveDown.AnimationSmoothness"
	} else {
		histogramName = "Ash.Notification.ClearAllVisible.AnimationSmoothness"
	}

	// Create 12 notifications (3 groups of 4 different notifications), close them all via the ClearAll button, and record performance metrics.
	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		ids := make([]string, 12)
		for i := 1; i <= n; i++ {
			for idx, t := range ts {
				if id, err := ash.CreateTestNotification(ctx, tconn, t, fmt.Sprintf("Test%sNotification%d", t, i-1), "test message"); err != nil {
					s.Fatalf("Failed to create %d-th %s notification: %v", i-1, t, err)
				} else {
					var index = (i-1)*len(ts) + idx
					ids[index] = id
					// Wait for each notification to post. This is faster than waiting for the final notification at the end, because sometimes posting 12 notifications at once can result in a very long wait.
					if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(fmt.Sprintf("Test%sNotification%d", t, i-1))); err != nil {
						s.Fatal("Failed waiting for notification: ", err)
					}
				}
			}
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {

			// Wait a few seconds, otherwise all notifications will be added and removed very quickly.
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			if shouldClearOneAtATime {
				// Clear the notifications one at a time.
				for i := len(ids) - 1; i >= 0; i-- {
					if err := testing.Poll(ctx, func(ctx context.Context) error {
						if err := ash.ClearNotification(ctx, tconn, ids[i]); err != nil {
							return nil
						}
						// Wait for stabilization / animation completion, otherwise all notification removals will happen unrealistically fast.
						if err := testing.Sleep(ctx, 1*time.Second); err != nil {
							s.Fatal("Failed to wait: ", err)
						}

						return nil
					}, &testing.PollOptions{Timeout: uiTimeout}); err != nil {
						return errors.Wrap(err, "failed to wait for clearing the notification")
					}
				}
			} else {
				// Clear all notifications at once via the ClearAll button.
				if err := uiauto.Combine(
					"click the ClearAll button to close all notifications",
					ac.LeftClick(nodewith.ClassName("StackingBarLabelButton")),
				)(ctx); err != nil {
					s.Fatal("Failed to collapse the uber tray: ", err)
				}

				// Wait a few seconds for notifications to stabilize.
				if err := testing.Sleep(ctx, 3*time.Second); err != nil {
					s.Fatal("Failed to wait: ", err)
				}
			}

			return nil
		}, &testing.PollOptions{Timeout: uiTimeout}); err != nil {
			return errors.Wrap(err, "failed to wait for notification")
		}
		return nil
	},
		histogramName),
		perfutil.StoreAllWithHeuristics(""))

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
