// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SessionEphemeral,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that managed guest session (MGS) is ephermeral by checking that a toggled setting is lost upon session exit",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

const accessibilityPage = "osAccessibility"
const accessibilityOptions = "Always show accessibility options in the system menu"

func SessionEphemeral(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
	)
	defer mgs.Close(ctx)
	if err != nil {
		s.Fatal("Failed to start MGS: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	ui := uiauto.New(tconn)
	button := nodewith.Name(accessibilityOptions).Role(role.ToggleButton)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, accessibilityPage, ui.WaitUntilExists(button))
	if err != nil {
		s.Fatal("Failed to open settings page: ", err)
	}

	if err := settings.SetToggleOption(cr, accessibilityOptions, true)(ctx); err != nil {
		s.Fatal("Failed to toggle accessibility settings: ", err)
	}

	opts := []chrome.Option{
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
		chrome.NoLogin(),
	}
	cr, err = chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to restart Chrome to simulate ending session: ", err)
	}
	defer cr.Close(ctx)

	tconn, err = cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	settings, err = ossettings.LaunchAtPageURL(ctx, tconn, cr, accessibilityPage, ui.WaitUntilExists(button))
	if err != nil {
		s.Fatal("Failed to open settings page in the new session: ", err)
	}

	if err := settings.WaitUntilToggleOption(cr, accessibilityOptions, false)(ctx); err != nil {
		s.Fatal("Failed to wait for setting to be false: ", err)
	}
}
