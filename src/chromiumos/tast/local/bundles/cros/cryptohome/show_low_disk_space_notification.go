// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/bundles/cros/cryptohome/cleanup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowLowDiskSpaceNotification,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test showing the low disk space notification",
		Contacts: []string{
			"vsavu@google.com",     // Test author
			"gwendal@chromium.com", // Lead for ChromeOS Storage
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

	fillFile, err := disk.FillUntil(cleanup.UserHome, cleanup.MinimalFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill disk space: ", err)
	}
	defer func() {
		if err := os.Remove(fillFile); err != nil {
			s.Errorf("Failed to remove fill file %s: %v", fillFile, err)
		}
	}()

	const notificationWaitTime = 150 * time.Second // Timeout for checking for low disk space notification.
	const notificationID = "low_disk"              // Hardcoded in Chrome.

	s.Logf("Waiting %d seconds for %v notification", notificationWaitTime/time.Second, notificationID)
	if _, err := ash.WaitForNotification(ctx, tconn, notificationWaitTime, ash.WaitIDContains(notificationID)); err != nil {
		// Check if too much space was made available.
		freeSpace, fErr := disk.FreeSpace(cleanup.UserHome)
		if fErr != nil {
			s.Fatal("Failed to read the amount of free space", fErr)
		}

		if freeSpace >= cleanup.NotificationThreshold {
			s.Errorf("Space was cleaned without notification: got %d; want < %d", freeSpace, cleanup.NotificationThreshold)
		}

		s.Error("Notification not shown: ", err)
	}
}
