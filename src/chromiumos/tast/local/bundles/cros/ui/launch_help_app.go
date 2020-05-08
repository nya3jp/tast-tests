// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpApp,
		Desc: "Help app should be launched after OOBE",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ui.LaunchHelpApp.consumer_username", "ui.LaunchHelpApp.consumer_password"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchHelpApp verifies launching Showoff after OOBE
func LaunchHelpApp(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("ui.LaunchHelpApp.consumer_username")
	password := s.RequiredVar("ui.LaunchHelpApp.consumer_password")

	cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin(), chrome.NotSkipOOBEPostLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Verify HelpApp (aka Discover) launched in background
	_, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: apps.Help.Name}, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to wait for Help app launched in background: ", err)
	}

	// Find Overview tab to verify app rendering.
	params := ui.FindParams{
		Name: "Overview",
		Role: ui.RoleTypeTab,
	}
	_, err = ui.FindWithTimeout(ctx, tconn, params, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to render Help app: ", err)
	}
}
