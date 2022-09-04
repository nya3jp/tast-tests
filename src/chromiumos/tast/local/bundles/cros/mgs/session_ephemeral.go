// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
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
		Timeout:      5 * time.Minute,
	})
}

const accessibilityPage = "osAccessibility"
const accessibilityOptions = "Show accessibility options in Quick Settings"

func SessionEphemeral(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	if err := launchMGSAndToggleAccessibilityOptions(ctx, fdms); err != nil {
		s.Fatal("Failed to modify options in a first MGS session: ", err)
	}

	// First MGS is closed, now start a new one and verify the setting is back to default.

	if err := launchMGSAndCheckAccessibilityOptions(ctx, fdms); err != nil {
		s.Fatal("Failed to verify options go back to default on second MGS session: ", err)
	}
}

func launchMGSAndToggleAccessibilityOptions(ctx context.Context, fdms *fakedms.FakeDMS) error {
	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
	)
	if err != nil {
		return errors.Wrap(err, "failed to start MGS")
	}
	defer mgs.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "getting test API connection failed")
	}

	ui := uiauto.New(tconn)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, accessibilityPage, ui.WaitUntilExists(nodewith.Name(accessibilityOptions).Role(role.ToggleButton)))
	if err != nil {
		return errors.Wrap(err, "failed to open setting page")
	}

	if err := settings.SetToggleOption(cr, accessibilityOptions, true)(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle accessibility settings")
	}

	return nil
}

func launchMGSAndCheckAccessibilityOptions(ctx context.Context, fdms *fakedms.FakeDMS) error {
	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
	)
	if err != nil {
		return errors.Wrap(err, "failed to start MGS")
	}
	defer mgs.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "getting test API connection failed")
	}

	ui := uiauto.New(tconn)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, accessibilityPage, ui.WaitUntilExists(nodewith.Name(accessibilityOptions).Role(role.ToggleButton)))
	if err != nil {
		return errors.Wrap(err, "failed to open setting page")
	}

	if err := settings.WaitUntilToggleOption(cr, accessibilityOptions, false)(ctx); err != nil {
		return errors.Wrap(err, "managed guest session was not ephermeral")
	}

	return nil
}
