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
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type notificationCloseTestType int

const (
	clearOneAtATime        notificationCloseTestType = iota // In test, clear notifications one at a time.
	clearOneAtATimeWithARC                                  // In test, clear notifications one at a time, with ARC notification.
	clearAll                                                // In test, clear all notifications at once
	clearAllWithARC                                         // In test, clear all notifications at once, with ARC notification.
)

type notificationClearTestVal struct {
	TestType notificationCloseTestType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         NotificationClosePerf,
		Desc:         "Measures animation performance of the clear all animation or individual notification deletion in the message center",
		Contacts:     []string{"newcomer@chromium.org", "cros-status-area-eng@google.com", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      8 * time.Minute,
		Params: []testing.Param{
			{
				Name: "one_at_a_time",
				Val:  clearOneAtATime,
			},
			{
				Name:              "one_at_a_time_arc",
				ExtraSoftwareDeps: []string{"arc"},
				Val:               clearOneAtATimeWithARC,
			},
			{
				Name: "clear_all",
				Val:  clearAll,
			},
			{
				Name:              "clear_all_arc",
				ExtraSoftwareDeps: []string{"arc"},
				Val:               clearAllWithARC,
			},
		},
	})
}

func NotificationClosePerf(ctx context.Context, s *testing.State) {
	testType := s.Param().(notificationCloseTestType)
	shouldClearOneAtATime := testType == clearOneAtATime || testType == clearOneAtATimeWithARC
	isArc := testType == clearOneAtATimeWithARC || testType == clearAllWithARC

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

	automationController := uiauto.New(tconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	collapseButton := nodewith.ClassName("CollapseButton")

	// Open the status area, then collapse the uber tray which results in an
	// expanded MessageCenter.
	if err := uiauto.Combine(
		"open the uber tray, expand it, then close some notifications",
		automationController.LeftClick(statusArea),
		automationController.WaitUntilExists(collapseButton),
		automationController.LeftClick(collapseButton),
		automationController.WaitForLocation(collapseButton),
	)(ctx); err != nil {
		s.Fatal("Failed to collapse the uber tray: ", err)
	}

	// Ensure no notifications currently exist.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to clear all notifications prior to adding notifications")
	}

	var histogramName string
	if shouldClearOneAtATime {
		histogramName = "Ash.Notification.MoveDown.AnimationSmoothness"
	} else {
		histogramName = "Ash.Notification.ClearAllVisible.AnimationSmoothness"
	}

	const (
		uiTimeout = 30 * time.Second
		// Create 12 notifications, 3 groups of 4 types of notifications. This is
		// enough to force notification overflow to happen.
		n = 3
	)

	// Add some notifications so that notification centre shows up when opening
	// Quick Settings.
	notificationTypes := []ash.NotificationType{
		ash.NotificationTypeBasic,
		ash.NotificationTypeImage,
		ash.NotificationTypeProgress,
		ash.NotificationTypeList,
	}

	// Create 12 notifications (3 groups of 4 different notifications) with 3 ARC notifications if applicable,
	// close them all via either the ClearAll button or one at a time, and record performance metrics.
	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		ids := make([]string, n*len(notificationTypes))
		for i := 0; i <= n-1; i++ {
			for idx, t := range notificationTypes {
				if id, err := ash.CreateTestNotification(ctx, tconn, t, fmt.Sprintf("Test%sNotification%d", t, i), "test message"); err != nil {
					s.Fatalf("Failed to create %d-th %s notification: %v", i, t, err)
				} else {
					var index = i*len(notificationTypes) + idx
					ids[index] = id
					// Wait for each notification to post. This is faster than waiting for
					// the final notification at the end, because sometimes posting 12
					// notifications at once can result in a very long wait.
					if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(fmt.Sprintf("Test%sNotification%d", t, i))); err != nil {
						s.Fatal("Failed waiting for notification: ", err)
					}
				}
			}

			// Create an ARC notification.
			if isArc {
				if err := arcclient.CreateOrUpdateTestNotification(ctx, tconn, fmt.Sprintf("TestARCNotification%d", i), "test message", fmt.Sprintf("%d", i)); err != nil {
					s.Fatalf("Failed to create %d-th ARC notification: %v", i, err)
				}
			}
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// Wait a few seconds, otherwise all notifications will be added and
			// removed very quickly.
			// TODO(crbug/1236150): Replace Sleeps with WaitUntilIdle when implemented.
			if err := testing.Sleep(ctx, 2*time.Second); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			if shouldClearOneAtATime {
				// Clear the notifications one at a time.
				for i := len(ids) - 1; i >= 0; i-- {
					if err := testing.Poll(ctx, func(ctx context.Context) error {
						if err := ash.ClearNotification(ctx, tconn, ids[i]); err != nil {
							s.Fatal("Failed to clear notification: ", err)
						}
						// Wait for stabilization / animation completion, otherwise all
						// notification removals will happen unrealistically fast.
						// TODO(crbug/1236150): Replace Sleeps with WaitUntilIdle when implemented.
						if err := testing.Sleep(ctx, time.Second); err != nil {
							s.Fatal("Failed to wait: ", err)
						}

						return nil
					}, &testing.PollOptions{Timeout: uiTimeout}); err != nil {
						return errors.Wrap(err, "failed to wait for clearing the notification")
					}
				}

				if isArc {
					// Clear ARC notifications.
					for i := n - 1; i >= 0; i-- {
						if err := testing.Poll(ctx, func(ctx context.Context) error {
							if err := arcclient.RemoveNotification(ctx, tconn, fmt.Sprintf("%d", i)); err != nil {
								s.Fatal("Failed to remove notification: ", err)
							}
							if err := testing.Sleep(ctx, time.Second); err != nil {
								s.Fatal("Failed to wait: ", err)
							}
							return nil
						}, &testing.PollOptions{Timeout: uiTimeout}); err != nil {
							return errors.Wrap(err, "failed to wait for clearing the notification")
						}
					}
				}
			} else {
				// Clear all notifications at once via the ClearAll button.
				if err := uiauto.Combine(
					"click the ClearAll button to close all notifications",
					automationController.LeftClick(nodewith.ClassName("StackingBarLabelButton")),
				)(ctx); err != nil {
					s.Fatal("Failed to collapse the uber tray: ", err)
				}

				// Wait a few seconds for notifications to stabilize.
				// TODO(crbug/1236150): Replace Sleeps with WaitUntilIdle when implemented.
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
