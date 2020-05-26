// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

// TODO(b/156433022): Parameterize test to run on both clamshell and tablet.

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpApp,
		Desc: "Help app should be launched after OOBE",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"apps.LaunchHelpApp.consumer_username", "apps.LaunchHelpApp.consumer_password"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchHelpApp verifies launching Showoff after OOBE.
func LaunchHelpApp(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("apps.LaunchHelpApp.consumer_username")
	password := s.RequiredVar("apps.LaunchHelpApp.consumer_password")

	cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin(), chrome.DontSkipOOBEAfterLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	tabletEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get current ui mode: ", err)
	}

	// Verify HelpApp (aka Explore) launched in Clamshell mode.
	if !tabletEnabled {
		if _, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: apps.Help.Name}, 20*time.Second); err != nil {
			s.Error("Failed to wait for Help app launched: ", err)
		}

		// Find Overview tab to verify app rendering.
		params := ui.FindParams{
			Name: "Overview",
			Role: ui.RoleTypeTab,
		}
		if _, err = ui.FindWithTimeout(ctx, tconn, params, 20*time.Second); err != nil {
			s.Error("Failed to render Help app: ", err)
		}
	} else {
		// Verify HelpApp (aka Explore) not to launch in Tablet mode.
		isHelpAppLaunched, err := ui.Exists(ctx, tconn, ui.FindParams{Name: apps.Help.Name})
		if err != nil {
			s.Error("Failed to check HelpApp existence: ", err)
		}

		if isHelpAppLaunched {
			s.Error("Help app is launched in Tablet mode")
		}
	}
}
