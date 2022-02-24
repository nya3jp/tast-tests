// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/apputil/calendar"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CalendarNotifications,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies calendar events pop up and auto hides in 6s",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc"},
		Timeout:      5*time.Minute + apputil.InstallationTimeout,
		Fixture:      "arcBootedWithPlayStore",
	})
}

// CalendarNotifications verify calendar events pop up and auto hides in 6s.
func CalendarNotifications(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	app, newAppErr := calendar.NewApp(ctx, kb, tconn, a)
	if newAppErr != nil {
		s.Fatal("Failed to create calendar app instance: ", err)
	}
	defer app.Close(cleanupCtx, cr, s.HasError, s.OutDir())
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "calendar_notifications")

	if err := app.Install(ctx); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	if _, err := app.Launch(ctx); err != nil {
		s.Fatal("Failed to launch calendar: ", err)
	}

	if err := app.SkipPrompt(ctx); err != nil {
		s.Fatal("Failed to skip start prompt: ", err)
	}

	// Make sure the presentation is month.
	if err := app.SelectMonth(ctx); err != nil {
		s.Fatal("Failed to select month: ", err)
	}

	// Make sure the date is now.
	if err := app.JumpToToday(ctx); err != nil {
		s.Fatal("Failed to jump to today: ", err)
	}

	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close all notifications: ", err)
	}

	eventName := "Notifications test"

	if err := app.CreateEvent(ctx, eventName); err != nil {
		s.Fatal("Failed to create new event: ", err)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		if err := app.DeleteEvent(ctx, eventName); err != nil {
			s.Logf("Failed to delete event made in case: %q", err)
		}

		// Reset state.
		if err := app.SelectMonth(ctx); err != nil {
			s.Logf("Failed to select month: %q", err)
		}
	}(cleanupCtx)

	ui := uiauto.New(tconn)
	notification := nodewith.NameContaining(eventName).Role(role.Button).HasClass("ArcNotificationContentView")

	if err := ui.WaitUntilExists(notification)(ctx); err != nil {
		s.Fatal("Failed to wait notification pop up: ", err)
	}

	ts := time.Now()
	if err := ui.WaitUntilGone(notification)(ctx); err != nil {
		s.Fatal("Failed to wait notification disappear: ", err)
	}
	goneDuration := time.Since(ts)

	// The calendar event notification must pops up and auto hides in 6s.
	// Two extra seconds is allowed for performance and latency concerns.
	if goneDuration > 8*time.Second {
		error := errors.Errorf("the calendar event notification pops up and auto hides in %s", goneDuration)
		s.Fatal("Failed to verify notification pops up and auto hides in 6s: ", error)
	}
}
