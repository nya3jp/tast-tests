// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MessageNotification,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that Android message notifications appear in Phone Hub and that inline reply works",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboardedAllFeatures",
		Timeout:      3 * time.Minute,
	})
}

// MessageNotification tests receiving message notifications in Phone Hub and replying to them.
func MessageNotification(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice

	// Reserve time for deferred cleanup functions.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Clear any notifications that are currently displayed.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to clear notifications")
	}

	// Generate a message notification on the Android device.
	title := "Hello!"
	text := "Notification test"
	waitForReply, err := androidDevice.GenerateMessageNotification(ctx, 1 /*id*/, title, text)
	if err != nil {
		s.Fatal("Failed to generate Android message notification: ", err)
	}

	// Wait for the notification on Chrome OS.
	n, err := ash.WaitForNotification(ctx, tconn, 10*time.Second, ash.WaitTitle(title))
	if err != nil {
		s.Fatal("Failed waiting for the message notification to appear on CrOS: ", err)
	}
	if n.Message != text {
		s.Fatalf("Notification text does not match: wanted %v, got %v", text, n.Message)
	}

	// Open Quick Settings to make sure the notification is visible.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Quick Settings to check notifications: ", err)
	}
	defer quicksettings.Hide(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Reply using the notification's inline reply field.
	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to set up virtual keyboard: ", err)
	}
	replyText := "Goodbye!"
	ui := uiauto.New(tconn)
	if err := ui.LeftClick(nodewith.Role(role.Button).Name("REPLY"))(ctx); err != nil {
		s.Fatal("Failed to click notification's REPLY button: ", err)
	}
	if err := kb.Type(ctx, replyText+"\n"); err != nil {
		s.Fatal("Failed to type a reply in the notification: ", err)
	}

	// Wait for the Android device to receive the reply and verify the text matches.
	received, err := waitForReply(ctx)
	if err != nil {
		s.Fatal("Failed waiting to receive a reply on the Android device: ", err)
	}

	if received != replyText {
		s.Fatalf("Reply received by the snippet does not match: wanted %v, got %v", replyText, received)
	}
}
