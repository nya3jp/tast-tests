// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
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
		Pre:          chrome.LoggedIn(),
	})
}

// NotificationCentreSmoke tests that notifications appear in notification centre.
func NotificationCentreSmoke(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Creating notification")
	if _, err := ash.CreateTestNotification(ctx, tconn, ash.NotificationTypeBasic, "TestNotification1", "blahhh"); err != nil {
		s.Fatal("Failed to create test notification: ", err)
	}
	s.Log("Checking that notification appears")
	params := ui.FindParams{
		Role:      ui.RoleTypeWindow,
		ClassName: "ash/message_center/MessagePopup",
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, uiTimeout); err != nil {
		s.Fatal("Failed to find notification popup: ", err)
	}
	s.Log("Waiting for notification to auto-dismiss")
	// Depending on DUT state when test is run, the notification could be focused and will not autodismiss.
	// Check for that condition while waiting for notification to dismiss.
	subparams := ui.FindParams{
		State: map[ui.StateType]bool{ui.StateTypeFocused: true},
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		notificationPopup, err := ui.Find(ctx, tconn, params)
		if err != nil {
			// Notification dismissed.
			return nil
		}
		defer notificationPopup.Release(ctx)
		// Notification still exists. Check if it has focus.
		exists, err := notificationPopup.DescendantExists(ctx, subparams)
		if err != nil {
			return errors.Wrap(err, "failed to find notification with focus")
		}
		if !exists {
			return errors.New("focused notification does not exist")
		}
		s.Log("Notification had focus. Opening launcher to change focus")
		if err := launcher.OpenLauncher(ctx, tconn); err != nil {
			s.Fatal("Failed to open launcher: ", err)
		}
		return errors.New("notification exists and was focused")
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed waiting for notification to dismiss: ", err)
	}
	s.Log("Open quick settings")
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show quick settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)
	s.Log("Closing notification from notification centre")
	if err := closeNotification(ctx, tconn); err != nil {
		s.Fatal("Failed to close notification: ", err)
	}
	s.Log("Firing another notification while notification centre is open")
	if _, err := ash.CreateTestNotification(ctx, tconn, ash.NotificationTypeBasic, "TestNotification2", "testttt"); err != nil {
		s.Fatal("Failed to create test notification: ", err)
	}
	s.Log("Closing notification from notification centre")
	if err := closeNotification(ctx, tconn); err != nil {
		s.Fatal("Failed to close notification: ", err)
	}
}

func closeNotification(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{Role: ui.RoleTypeButton, Name: "Notification close"}
	notificationClose, err := ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find the notification close button")
	}
	defer notificationClose.Release(ctx)
	if err := notificationClose.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click notification close button")
	}
	if err := ui.WaitUntilGone(ctx, tconn, params, uiTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for closed notification to disappear")
	}
	return nil
}
