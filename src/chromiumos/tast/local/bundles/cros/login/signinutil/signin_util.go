// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package signinutil provides functions that help with management of sign-in restrictions
package signinutil

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// RestrictSignInOption is the name of the option in OS Settings that allows user to
// restrict sign-in to the existing or provided list of users.
const RestrictSignInOption = "Restrict sign-in to the following users:"

// GetUsernameFromEmail returns the part of the email before '@'.
func GetUsernameFromEmail(email string) string {
	return email[:strings.IndexByte(email, '@')]
}

// OpenManageOtherPeople opens "Manage other people" section in OS Settings.
func OpenManageOtherPeople(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*ossettings.OSSettings, error) {
	ui := uiauto.New(tconn)

	const subsettingsName = "Manage other people"

	// Open settings, Manage Other People.
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrivacy", ui.WaitUntilExists(nodewith.Name(subsettingsName)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the settings page")
	}

	if err := ui.LeftClick(nodewith.Name(subsettingsName))(ctx); err != nil {
		return settings, errors.Wrap(err, "failed to open Manage other people subsettings")
	}

	if err := ui.WaitUntilExists(nodewith.Name(RestrictSignInOption).Role(role.ToggleButton))(ctx); err != nil {
		return settings, errors.Wrap(err, "failed to wait for the toggle to show the list of users")
	}

	return settings, nil
}
