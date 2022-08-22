// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ToggleCellularFromOSSettings,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Cellular can be enabled and disabled from the OS Settings",
		Contacts: []string{
			"nikhilcn@chromium.org",
			"cros-connectivity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:cellular"},
		Fixture:      "cellular",
	})
}

// ToggleCellularFromOSSettings tests that a user can successfully toggle the
// Cellular state using the Cellular toggle on the OS Settings page.
func ToggleCellularFromOSSettings(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to create a new instance of Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	app, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch OS Settings: ", err)
	} else {
		defer app.Close(ctx)
	}

	if err := app.SetToggleOption(cr, "Mobile data enable", true)(ctx); err != nil {
		s.Fatal("Failed to enable mobile data in UI: ", err)
	}
	if err := helper.WaitForEnabledState(ctx, true); err != nil {
		s.Fatal("Failed to enable Cellular state: ", err)
	}

	state := false
	const iterations = 5
	for i := 0; i < iterations; i++ {
		s.Logf("Toggling Cellular (iteration %d of %d)", i+1, iterations)

		if err := app.SetToggleOption(cr, "Mobile data enable", state)(ctx); err != nil {
			s.Fatal("Failed to enable mobile data in UI: ", err)
		}
		if err := helper.WaitForEnabledState(ctx, state); err != nil {
			s.Fatal("Failed to toggle Cellular state: ", err)
		}
		state = !state
	}
}
