// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"bytes"
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OwnershipNotRetaken,
		Desc: "Subsequent logins after the owner must not clobber the owner's key",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func OwnershipNotRetaken(ctx context.Context, s *testing.State) {
	const (
		ownerKeyFile = "/var/lib/whitelist/owner.key"
		testUser     = "example@chromium.org"
		testPass     = "testme"
		testGAIAID   = "7583"

		uiSetupTimeout = 90 * time.Second
	)

	// Clear the device ownership info.
	if err := func() error {
		s.Log("Clearing device ownership info")
		sctx, cancel := context.WithTimeout(ctx, uiSetupTimeout)
		defer cancel()

		if err := upstart.StopJob(sctx, "ui"); err != nil {
			return err
		}
		// Run with original context in case of errors, so ignore the
		// error of EnsureJobRunning here.
		// If following procedures succeed, this is almost no-op.
		defer upstart.EnsureJobRunning(ctx, "ui")

		if err := session.ClearDeviceOwnership(sctx); err != nil {
			return err
		}
		return upstart.EnsureJobRunning(sctx, "ui")
	}(); err != nil {
		s.Fatal("Failed to set up the device: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session_manager: ", err)
	}

	// Initial login. The ownership file should be created.
	// Send the data to session_manager.
	func() {
		s.Log("Logging in to Chrome, first time")
		w, err := sm.WatchPropertyChangeComplete(ctx)
		if err != nil {
			s.Fatal("Failed to start watching PropertyChangeComplete signal: ", err)
		}
		defer w.Close(ctx)

		// Log in with Chrome, but do not clear the device ownership info.
		c, err := chrome.New(ctx, chrome.KeepState(), chrome.FetchPolicy())
		if err != nil {
			s.Fatal("Failed to log in with Chrome: ", err)
		}
		defer func() {
			c.Close(ctx)
			err = upstart.RestartJob(ctx, "ui")
			if err != nil {
				s.Fatal("Failed to restart ui: ", err)
			}
		}()

		select {
		case <-w.Signals:
		case <-ctx.Done():
			s.Fatal("Timed out waiting for PropertyChangeComplete signal: ", ctx.Err())
		}
	}()

	key, err := ioutil.ReadFile(ownerKeyFile)
	if err != nil {
		s.Fatalf("Failed to read %s: %v", ownerKeyFile, err)
	}

	// Second login.
	s.Log("Logging in to Chrome, second time")
	c, err := chrome.New(ctx, chrome.Auth(testUser, testPass, testGAIAID), chrome.KeepState(), chrome.FetchPolicy())
	if err != nil {
		s.Fatalf("Failed to log in %s with Chrome: %v", testUser, err)
	}
	defer cryptohome.RemoveVault(ctx, testUser)
	// Ignore error on Close(), because anyways restart "ui" just below
	// logs out.
	c.Close(ctx)
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out from Chrome: ", err)
	}

	// Compare the file content.
	if key2, err := ioutil.ReadFile(ownerKeyFile); err != nil {
		s.Fatalf("Failed to read %s second time: %v", ownerKeyFile, err)
	} else if !bytes.Equal(key, key2) {
		s.Fatalf("%s is changed on second login", ownerKeyFile)
	}
}
