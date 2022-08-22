// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	ui := uiauto.New(tconn)

	app, err := ossettings.Launch(ctx, tconn)
	defer app.Close(ctx)

	if err != nil {
		s.Fatal("Failed to launch OS Settings: ", err)
	}

	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable Cellular: ", err)
	}

	var OsSettingsCellularToggleButton = nodewith.NameContaining("Mobile data").Role(role.ToggleButton)

	state := false
	const iterations = 20
	for i := 0; i < iterations; i++ {
		s.Logf("Toggling Cellular (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(OsSettingsCellularToggleButton)(ctx); err != nil {
			s.Fatal("Failed to click the Cellular toggle: ", err)
		}
		if err := helper.WaitForEnabledState(ctx, state); err != nil {
			s.Fatal("Failed to toggle Cellular state: ", err)
		}
		state = !state
	}
}
