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
		Func: LaunchShowoffAfterOOBE,
		Desc: "Showoff should be launched after OOBE",
		Contacts: []string{
			"show-off@google.com",
			"shengjun@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"ui.LaunchShowoffAfterOOBE.consumer_username", "ui.LaunchShowoffAfterOOBE.consumer_password"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchShowoffAfterOOBE verifies launching Showoff after OOBE
func LaunchShowoffAfterOOBE(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("ui.LaunchShowoffAfterOOBE.consumer_username")
	password := s.RequiredVar("ui.LaunchShowoffAfterOOBE.consumer_password")

	cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin(), chrome.NotSkipOOBEPostLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Verify Discover lauched in background
	_, err = ui.FindWithTimeout(ctx, tconn, ui.FindParams{Name: apps.Discover.Name}, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to wait for Discover launched in background: ", err)
	}

	// Find Overview tab to verify app rendering.
	params := ui.FindParams{
		Name: "Overview",
		Role: ui.RoleTypeTab,
	}
	_, err = ui.FindWithTimeout(ctx, tconn, params, 20*time.Second)
	if err != nil {
		s.Fatal("Failed to render Discover: ", err)
	}
}
