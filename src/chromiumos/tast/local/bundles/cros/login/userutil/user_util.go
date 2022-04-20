// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package userutil provides functions that help with management of users
package userutil

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/testing"
)

// TakingOwnershipTimeout is the maximum amount of time for taking ownership of the device after login.
const TakingOwnershipTimeout = 20 * time.Second

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

// WaitForOwnership waits for up to TakingOwnershipTimeout for the current user to become device owner.
// Normally this should take less then a few seconds.
func WaitForOwnership(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating login test API connection failed")
	}

	testing.ContextLog(ctx, "Waiting for the user to become device owner")

	var pollOpts = &testing.PollOptions{Interval: time.Second, Timeout: TakingOwnershipTimeout}
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
