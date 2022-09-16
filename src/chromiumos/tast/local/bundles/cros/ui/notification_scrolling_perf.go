// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/notification"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type notificationScrollingPerfTestParam struct {
	arc bool
	bt  browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         NotificationScrollingPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures input latency of scrolling through notification list",
		Contacts:     []string{"leandre@chromium.org", "cros-status-area-eng@google.com", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Val: notificationScrollingPerfTestParam{false, browser.TypeAsh},
		}, {
			Name:              "arc",
			ExtraSoftwareDeps: []string{"arc"},
			Val:               notificationScrollingPerfTestParam{true, browser.TypeAsh},
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               notificationScrollingPerfTestParam{false, browser.TypeLacros},
		}, {
			Name:              "arc_lacros",
			ExtraSoftwareDeps: []string{"arc", "lacros"},
			Val:               notificationScrollingPerfTestParam{true, browser.TypeLacros},
		}},
	})
}

func NotificationScrollingPerf(ctx context.Context, s *testing.State) {
	// Reserve a few seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	isArc := s.Param().(notificationScrollingPerfTestParam).arc
	bt := s.Param().(notificationScrollingPerfTestParam).bt

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	var initArcOpt []chrome.Option
	if isArc {
		initArcOpt = []chrome.Option{chrome.ARCEnabled()}
	}

	// Set up the browser.
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(), initArcOpt...)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	atconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API from ash: ", err)
	}
	btconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API from browser: ", err)
	}

	// Minimize opened windows (if exists) to reduce background noise during the measurement.
	if err := ash.ForEachWindow(ctx, atconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, atconn, w.ID, ash.WindowStateMinimized)
	}); err != nil {
		s.Fatal("Failed to set window states: ", err)
	}

	var arcclient *notification.ARCClient
	if isArc {
		// Note that ARC uses the test API from ash-chrome to manage notifications.
		arcclient, err = notification.NewARCClient(ctx, atconn, cr, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARCClient: ", err)
		}
		defer arcclient.Close(cleanupCtx, atconn)
	}

	// Add some notifications so that notification centre shows up when opening Quick Settings.
	// We will add 5 notifications of each type so that it is enough for scrolling.
	const n = 5
	const uiTimeout = 30 * time.Second
	ts := []browser.NotificationType{
		browser.NotificationTypeBasic,
		browser.NotificationTypeImage,
		browser.NotificationTypeProgress,
		browser.NotificationTypeList,
	}
	for i := 0; i <= n; i++ {
		for _, t := range ts {
			title := fmt.Sprintf("Test%sNotification%d", t, i)
			if _, err := browser.CreateTestNotification(ctx, btconn, t, title, "test message"); err != nil {
				s.Fatalf("Failed to create %d-th %s notification: %v", i, t, err)
			}
			// Wait for the notification to finish creating, making sure that it is created.
			if _, err := ash.WaitForNotification(ctx, atconn, uiTimeout, ash.WaitTitle(title)); err != nil {
				s.Fatalf("Failed to wait for %d-th %s notification: %v", i, t, err)
			}
		}

		// Create an ARC notification.
		if isArc {
			if err := arcclient.CreateOrUpdateTestNotification(ctx, atconn, fmt.Sprintf("TestARCNotification%d", i), "test message", fmt.Sprintf("%d", i)); err != nil {
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

	ac := uiauto.New(atconn)
	statusArea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
	messageCenter := nodewith.ClassName("UnifiedMessageCenterView")

	// Note that ash-chrome (cr and atconn) is passed in to take traces and metrics from ash-chrome.
	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(atconn, func(ctx context.Context) error {
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
		"Ash.MessageCenter.Scroll.PresentationTime.MaxLatency")),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
