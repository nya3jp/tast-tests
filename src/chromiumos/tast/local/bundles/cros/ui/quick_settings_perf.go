// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/notification"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuickSettingsPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures animation smoothness of quick settings expand and collapse animations",
		Contacts:     []string{"amehfooz@chromium.org", "leandre@chromium.org", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
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

func QuickSettingsPerf(ctx context.Context, s *testing.State) {
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
	const uiTimeout = 30 * time.Second
	const n = 5
	ts := []browser.NotificationType{
		browser.NotificationTypeBasic,
		browser.NotificationTypeImage,
		browser.NotificationTypeProgress,
		browser.NotificationTypeList,
	}
	for i := 0; i <= n; i++ {
		for _, t := range ts {
			title := fmt.Sprintf("Test%sNotification%d", t, i)
			if _, err := browser.CreateTestNotification(ctx, tconn, t, title, "test message"); err != nil {
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

	ac := uiauto.New(tconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	collapseButton := nodewith.ClassName("CollapseButton")

	// This includes toggle the Status Area button to record input latency of showing Quick Settings/notification centre
	// and toggle the collapsed state of the system tray to record animation smoothness.
	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
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
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded",
		"Ash.Window.AnimationSmoothness.Hide")),
		perfutil.StoreAllWithHeuristics(""))

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
