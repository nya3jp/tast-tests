// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/cryptohome/cleanup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShowLowDiskSpaceNotification,
		Desc: "Test showing the low disk space notification",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func ShowLowDiskSpaceNotification(ctx context.Context, s *testing.State) {
	// Notification is only shown if there are multiple users on the device.22
	if cr, err := chrome.New(ctx, chrome.Auth("user1@managedchrome.com", "test-0000", "gaia-id1")); err != nil {
		s.Fatal("Failed to log in with the first user: ", err)
	} else if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}

	cr, err := chrome.New(ctx, chrome.KeepState(), chrome.Auth("user2@managedchrome.com", "test-0000", "gaia-id2"))
	if err != nil {
		s.Fatal("Failed to log in with the second user: ", err)
	}
	defer cr.Close(ctx)

	fillFile, err := cleanup.FillUntil("/home/user", cleanup.MinimalFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill disk space: ", err)
	}
	defer func() {
		if err := os.Remove(fillFile); err != nil {
			s.Errorf("Failed to remove fill file %s: %v", fillFile, err)
		}
	}()

	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	s.Log("Waiting for notification")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		notifications, err := ash.VisibleNotifications(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get notifications"))
		}

		for _, notification := range notifications {
			// Hardcoded in Chrome.
			if strings.Contains(notification.ID, "low_disk") {
				return nil
			}
		}

		return errors.New("could not find low disk space notification")
	}, nil); err != nil {
		s.Error("Notification not shown: ", err)
	}
}
