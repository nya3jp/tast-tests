// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/cryptohome/cleanup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/disk"
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
		Pre:          chrome.LoggedIn(),
	})
}

func ShowLowDiskSpaceNotification(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}

	fillFile, err := cleanup.FillUntil(cleanup.UserHome, cleanup.MinimalFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill disk space: ", err)
	}
	defer func() {
		if err := os.Remove(fillFile); err != nil {
			s.Errorf("Failed to remove fill file %s: %v", fillFile, err)
		}
	}()

	s.Log("Waiting for notification")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		notifications, err := ash.Notifications(ctx, tconn)
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
	}, &testing.PollOptions{
		Timeout: 80 * time.Second, // Checks for low disk space run once per minute.
	}); err != nil {
		// Check if too much space was made available.
		freeSpace, err := disk.FreeSpace(cleanup.UserHome)
		if err != nil {
			s.Fatal("Failed to read the amount of free space")
		}

		if freeSpace >= cleanup.NotificationThreshold {
			s.Errorf("Space was cleaned without notification, %d bytes available", freeSpace)
		}

		s.Error("Notification not shown: ", err)
	}
}
