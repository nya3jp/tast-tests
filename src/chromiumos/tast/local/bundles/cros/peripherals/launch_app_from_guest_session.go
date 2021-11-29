// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// guestModeTest contains all the data needed to run a single test iteration.
type guestModeTest struct {
	app apps.App
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchAppFromGuestSession,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Peripherals app can be found and launched from guest mode",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInGuest",
		Params: []testing.Param{
			{
				Name: "diagnostics",
				Val: guestModeTest{
					app: apps.Diagnostics,
				},
			},
		},
	})
}

// LaunchAppFromGuestSession verifies launching an app from guest mode.
func LaunchAppFromGuestSession(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	// Attempt to open the Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API Connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	params := s.Param().(guestModeTest)

	err = apps.Launch(ctx, tconn, params.app.ID)
	if err != nil {
		s.Fatal("Failed to launch app: ", err)
	}

	err = ash.WaitForApp(ctx, tconn, params.app.ID, time.Minute)
	if err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}
}
