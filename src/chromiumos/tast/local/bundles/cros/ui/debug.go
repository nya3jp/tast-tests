// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Debug,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Useful for reproducing a bug where if you call ash.RemoveActiveDesk twice in overview, the second call hangs",
		Contacts:     []string{"amusbach@chromium.org", "afakhry@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Second,
	})
}

func Debug(ctx context.Context, s *testing.State) {
	tconn, err := s.FixtValue().(*chrome.Chrome).TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to ensure overview mode: ", err)
	}

	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create new desk: ", err)
	}

	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create new desk: ", err)
	}

	if err := ash.RemoveActiveDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to remove active desk: ", err)
	}

	if err := ash.RemoveActiveDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to remove active desk: ", err)
	}
}
