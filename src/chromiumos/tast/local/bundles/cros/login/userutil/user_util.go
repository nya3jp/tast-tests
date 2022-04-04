// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package userutil provides functions that help with management of users
package userutil

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// RestrictSignInOption is the name of the option in OS Settings that allows user to
// restrict sign-in to the existing or provided list of users.
const RestrictSignInOption = "Restrict sign-in to the following users:"

// CreateUser creates a new session with a given name and password, considering extra options. It immediately
// closes the session, so it should be used only for creating new users, not logging in.
func CreateUser(ctx context.Context, username, password string, extraOpts ...chrome.Option) error {
	cr, err := login(ctx, username, password, extraOpts...)
	if err != nil {
		return errors.Wrap(err, "failed to create new user")
	}

	cr.Close(ctx)

	return nil
}

// CreateDeviceOwner creates a user like the CreateUser function, but before closing the session
// it waits until the user becomes device owner.
func CreateDeviceOwner(ctx context.Context, username, password string, extraOpts ...chrome.Option) error {
	cr, err := login(ctx, username, password, extraOpts...)
	if err != nil {
		return errors.Wrap(err, "failed to create new user")
	}
	defer cr.Close(ctx)

	return WaitForOwnership(ctx, cr)
}

// Login creates a new user session using provided credentials. If the user doesn't exist it creates a new one.
// It always keeps the current state, and doesn't support other options.
func Login(ctx context.Context, username, password string) (*chrome.Chrome, error) {
	return login(ctx, username, password, chrome.KeepState())
}

func login(ctx context.Context, username, password string, extraOpts ...chrome.Option) (*chrome.Chrome, error) {
	creds := chrome.Creds{User: username, Pass: password}
	opts := append([]chrome.Option{chrome.FakeLogin(creds)}, extraOpts...)

	return chrome.New(ctx, opts...)
}

// GetKnownEmailsFromLocalState returns a map of users that logged in on the device, based on the LoggedInUsers from the LocalState file.
func GetKnownEmailsFromLocalState() (map[string]bool, error) {
	// Local State is a json-like structure, from which we will need only LoggedInUsers field.
	type LocalState struct {
		Emails []string `json:"LoggedInUsers"`
	}
	var localState LocalState
	if err := localstate.Unmarshal(browser.TypeAsh, &localState); err != nil {
		return nil, errors.Wrap(err, "failed to extract Local State")
	}
	knownEmails := make(map[string]bool)
	for _, email := range localState.Emails {
		knownEmails[email] = true
	}
	return knownEmails, nil
}

// WaitForOwnership waits for up to 20 seconds for the current user to become device owner.
// Normally this should take less then a few seconds.
func WaitForOwnership(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating login test API connection failed")
	}

	testing.ContextLog(ctx, "Waiting for the user to become device owner")

	var pollOpts = &testing.PollOptions{Interval: 1 * time.Second, Timeout: 20 * time.Second}
	var status struct {
		IsOwner bool
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &status); err != nil {
			// This is caused by failure to run javascript to get login status
			// quit polling.
			return testing.PollBreak(err)
		}
		testing.ContextLogf(ctx, "User is owner: %t", status.IsOwner)

		if !status.IsOwner {
			return errors.New("user did not become device owner yet")
		}

		return nil
	}, pollOpts)
}

// GetUsernameFromEmail returns the part of the email before '@'.
func GetUsernameFromEmail(email string) string {
	return email[:strings.IndexByte(email, '@')]
}

// OpenManageOtherPeople opens "Manage other people" section in OS Settings.
func OpenManageOtherPeople(ctx, cleanUpCtx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*ossettings.OSSettings, error) {
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
