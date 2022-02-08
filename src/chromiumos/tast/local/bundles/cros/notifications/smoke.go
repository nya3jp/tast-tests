// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package notifications

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

const uiTimeout = 30 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         Smoke,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that notifications appear in notification centre and can be interacted with",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"amehfooz@chromium.org",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

// Smoke tests that notifications appear in notification centre.
func Smoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Setup a browser.
	bt := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)
	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatalf("Failed to create Test API connection for %v browser: %v", bt, err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure DUT is not in tablet mode: ", err)
	}
	defer cleanup(ctx)

	s.Log("Creating notification")
	if _, err := ash.CreateTestNotification(ctx, bTconn, ash.NotificationTypeBasic, "TestNotification1", "blahhh"); err != nil {
		s.Fatal("Failed to create test notification: ", err)
	}
	s.Log("Checking that notification appears")
	ui := uiauto.New(tconn)
	notification := nodewith.Role(role.Window).ClassName("ash/message_center/MessagePopup")
	if err := ui.WithTimeout(uiTimeout).WaitUntilExists(notification)(ctx); err != nil {
		s.Fatal("Failed to find notification popup: ", err)
	}
	s.Log("Waiting for notification to auto-dismiss")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ui.Exists(notification)(ctx); err != nil {
			// Notification dismissed.
			return nil
		}
		// Notification still exists.
		// Depending on DUT state when test is run, the notification could be focused and will not autodismiss.
		// Check for that condition while waiting for notification to dismiss.
		if err := ui.Exists(nodewith.State(state.Focused, true).Ancestor(notification))(ctx); err != nil {
			return errors.New("focused notification does not exist")
		}
		s.Log("Notification had focus. Opening launcher to change focus")
		if err := launcher.Open(tconn)(ctx); err != nil {
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
	if _, err := ash.CreateTestNotification(ctx, bTconn, ash.NotificationTypeBasic, "TestNotification2", "testttt"); err != nil {
		s.Fatal("Failed to create test notification: ", err)
	}
	s.Log("Closing notification from notification centre")
	if err := closeNotification(ctx, tconn); err != nil {
		s.Fatal("Failed to close notification: ", err)
	}
}

func closeNotification(ctx context.Context, tconn *browser.TestConn) error {
	ui := uiauto.New(tconn).WithTimeout(uiTimeout)
	notificationClose := nodewith.Role(role.Button).Name("Notification close")
	if err := ui.LeftClick(notificationClose)(ctx); err != nil {
		return errors.Wrap(err, "failed to click notification close button")
	}
	if err := ui.WaitUntilGone(notificationClose)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for closed notification to disappear")
	}
	return nil
}
