// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package userutil provides functions that help with management of users
package userutil

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// CreateUser creates a new session with a given name and password, considering extra options. It immediately
// closes the session, so it should be used only for creating new users, not logging in
func CreateUser(ctx context.Context, s *testing.State, username, password string, extraOpts ...chrome.Option) {
	cr := login(ctx, s, username, password, extraOpts...)
	cr.Close(ctx)
}

// Login creates a new user session using provided credentials. If the user doesn't exist it creates a new one.
// It always keeps the current state, and doesn't support other options.
func Login(ctx context.Context, s *testing.State, username, password string) *chrome.Chrome {
	return login(ctx, s, username, password, chrome.KeepState())
}

func login(ctx context.Context, s *testing.State, username, password string, extraOpts ...chrome.Option) *chrome.Chrome {
	creds := chrome.Creds{User: username, Pass: password}
	opts := append([]chrome.Option{chrome.FakeLogin(creds)}, extraOpts...)

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	return cr
}

// GetKnowEmailsFromLocalState returns a map of users that logged in on the device, based on the LoggedInUsers from the LocalState file
func GetKnowEmailsFromLocalState(s *testing.State) map[string]bool {
	// LocalState is a json like structure, from which we will need only LoggedInUsers field.
	type LocalState struct {
		Emails []string `json:"LoggedInUsers"`
	}

	localStateFile, err := os.Open("/home/chronos/Local State")
	if err != nil {
		s.Fatal("Failed to open Local State file: ", err)
	}
	defer localStateFile.Close()

	var localState LocalState
	b, err := ioutil.ReadAll(localStateFile)
	if err != nil {
		s.Fatal("Failed to read Local State file contents: ", err)
	}
	if err := json.Unmarshal(b, &localState); err != nil {
		s.Fatal("Failed to unmarshal Local State: ", err)
	}
	knownEmails := make(map[string]bool)
	for _, email := range localState.Emails {
		knownEmails[email] = true
	}

	return knownEmails
}
