// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NotificationCentreSmoke,
		Desc: "Checks that notifications appear in notification centre and can be interacted with",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// NotificationCentreSmoke tests that screenshots notifications appear in notification centre.
func NotificationCentreSmoke(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Take a screenshot to show a notification. Using the virtual keyboard is required since
	// different physical keyboards can require different key combinations to take a screenshot.
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()

	s.Log("Taking screenshot")
	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}
	s.Log("Checking that screenshot notification appears")
	params := ui.FindParams{
		Role:      ui.RoleTypeWindow,
		ClassName: "ash/message_center/MessagePopup",
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to find notification center: ", err)
	}
	s.Log("Waiting for notification to disappear")
	// The notification should hide itself after a short delay.
	if err := ui.WaitUntilGone(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to find notification center: ", err)
	}
	s.Log("Open quick settings")
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quick settings: ", err)
	}
	s.Log("Closing notification from notificaiton centre")
	params = ui.FindParams{Role: ui.RoleTypeButton, Name: "Notification close"}
	notificationClose, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	defer notificationClose.Release(ctx)
	if err := notificationClose.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click notification close button: ", err)
	}
	if err := ui.WaitUntilGone(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for closed notification to disappear: ", err)
	}
	quicksettings.Hide(ctx, tconn)

	s.Log("Taking another screenshot")
	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		s.Fatal("Failed to take a screenshot: ", err)
	}
	s.Log("Waiting for screenshot notification to appear")
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to find notification center: ", err)
	}
	s.Log("Clicking notification")
	notification, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeStaticText, Name: "Show in folder"}, 10*time.Second)
	defer notification.Release(ctx)
	if err := notification.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click notification: ", err)
	}
	s.Log("Waiting for Files app to open")
	if err := ash.WaitForApp(ctx, tconn, apps.Files.ID); err != nil {
		s.Fatal("Files app didn't open after clicking screenshot notification: ", err)
	}
}
