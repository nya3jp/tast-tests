// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Screenshot2,
		Desc: "Behavior of the Screenshot policy",
		Contacts: []string{
			"lamzin@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoggedIn(),
	})
}

func Screenshot2(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}

	if err := keyboard.Accel(ctx, "Ctrl+Scale"); err != nil {
		s.Fatal("Failed to press buttons: ", err)
	}

	time.Sleep(1 * time.Second)

	nots, err := ash.Notifications(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get notifs: ", err)
	}
	if len(nots) > 0 {
		for _, not := range nots {
			s.Log("Notification: ", not)
		}
	}

	time.Sleep(20 * time.Second)

	nots, err = ash.Notifications(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get notifs: ", err)
	}
	if len(nots) > 0 {
		for _, not := range nots {
			s.Log("Notification: ", not)
		}
	}
}
