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
		Func:         CheckOverviewCleanup,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Useful for checking if overview is ended by Tast framework when a test finishes",
		Contacts:     []string{"amusbach@chromium.org", "xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params:       []testing.Param{{Name: "a"}, {Name: "b"}},
	})
}

func CheckOverviewCleanup(ctx context.Context, s *testing.State) {
	tconn, err := s.FixtValue().(*chrome.Chrome).TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if ash.WaitForOverviewState(ctx, tconn, ash.Shown, time.Second) == nil {
		s.Fatal("Detected overview mode at start of test")
	}

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview: ", err)
	}
}
