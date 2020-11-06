// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

const uiTimeout = 30 * time.Second

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

// NotificationCentreSmoke tests that notifications appear in notification centre.
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

	s.Log("Creating notification")
	if _, err := ash.CreateTestNotification(ctx, tconn, "TestNotification1", "blahhh"); err != nil {
		s.Fatal("Failed to create test notification: ", err)
	}
	s.Log("Checking that notification appears")
	params := ui.FindParams{
		Role:      ui.RoleTypeWindow,
		ClassName: "ash/message_center/MessagePopup",
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, uiTimeout); err != nil {
		s.Fatal("Failed to find notification center: ", err)
	}
	s.Log("Waiting for notification to auto-dismiss")
	if err := ui.WaitUntilGone(ctx, tconn, params, uiTimeout); err != nil {
		s.Fatal("Failed to find notification center: ", err)
	}
	s.Log("Open quick settings")
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quick settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)
	closeNotification(ctx, s, tconn)
	s.Log("Firing another notification while notification centre is open")
	if _, err := ash.CreateTestNotification(ctx, tconn, "TestNotification2", "testttt"); err != nil {
		s.Fatal("Failed to create test notification: ", err)
	}
	closeNotification(ctx, s, tconn)
}

func closeNotification(ctx context.Context, s *testing.State, tconn *chrome.TestConn) {
	s.Log("Closing notification from notification centre")
	params := ui.FindParams{Role: ui.RoleTypeButton, Name: "Notification close"}
	notificationClose, err := ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		s.Fatal("Failed to find the notification close button")
	}
	defer notificationClose.Release(ctx)
	if err := notificationClose.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click notification close button: ", err)
	}
	if err := ui.WaitUntilGone(ctx, tconn, params, uiTimeout); err != nil {
		s.Fatal("Failed to wait for closed notification to disappear: ", err)
	}
}
