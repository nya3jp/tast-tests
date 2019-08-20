// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io/ioutil"
	"path/filepath"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/local/bundles/cros/session/ownership"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemoteOwnership,
		Desc: "Verifies that Ownership API can be used to set device policies (as an enterprise might do)",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		Data: []string{"testcert.p12"},
	})
}

func RemoteOwnership(ctx context.Context, s *testing.State) {
	if err := session.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	privKey, err := session.ExtractPrivKey(s.DataPath("testcert.p12"))
	if err != nil {
		s.Fatal("Failed to parse PKCS #12 file: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

	// Initial policy set up.
	settings := ownership.BuildTestSettings("")
	if err := session.StoreSettings(ctx, sm, "", privKey, nil, settings); err != nil {
		s.Fatal("Failed to store settings: ", err)
	}
	if retrieved, err := session.RetrieveSettings(ctx, sm); err != nil {
		s.Fatal("Failed to retrieve settings: ", err)
	} else if diff := cmp.Diff(settings, retrieved); diff != "" {
		const diffName = "diff.txt"
		if err = ioutil.WriteFile(filepath.Join(s.OutDir(), diffName), []byte(diff), 0644); err != nil {
			s.Error("Failed to write diff: ", err)
		}
		s.Fatal("Unexpected settings were retrieved. Diff is found in ", diffName)
	}

	// Force re-key the device.
	privKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		s.Fatal("Failed to generate RSA key: ", err)
	}
	if err := session.StoreSettings(ctx, sm, "", privKey, nil, settings); err != nil {
		s.Fatal("Failed to store rekeyed settings: ", err)
	}
	if retrieved, err := session.RetrieveSettings(ctx, sm); err != nil {
		s.Fatal("Failed to retrieve rekeyed settings: ", err)
	} else if diff := cmp.Diff(settings, retrieved); diff != "" {
		const diffName = "diff-rekeyed.txt"
		if err = ioutil.WriteFile(filepath.Join(s.OutDir(), diffName), []byte(diff), 0644); err != nil {
			s.Error("Failed to write diff: ", err)
		}
		s.Fatal("Unexpected rekeyed settings were retrieved. Diff is found in ", diffName)
	}

	// Rotate key gracefully.
	const (
		testUser = "test@foo.com"
		testPass = "test_password"
	)
	// Create clean vault for the test user.
	if err = cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}
	if err = cryptohome.CreateVault(ctx, testUser, testPass); err != nil {
		s.Fatal("Failed to create vault: ", err)
	}
	newPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		s.Fatalf("Failed to generate RSA key for user %s: %v", testUser, err)
	}
	// Start a session for the user, then store the settings.
	if err = sm.StartSession(ctx, testUser, ""); err != nil {
		s.Fatal("Failed to start session: ", err)
	}
	if err := session.StoreSettings(ctx, sm, "", newPrivKey, privKey, settings); err != nil {
		s.Fatal("Failed to store user settings: ", err)
	}
	if retrieved, err := session.RetrieveSettings(ctx, sm); err != nil {
		s.Fatal("Failed to retrieve user settings: ", err)
	} else if diff := cmp.Diff(settings, retrieved); diff != "" {
		const diffName = "diff-user.txt"
		if err = ioutil.WriteFile(filepath.Join(s.OutDir(), diffName), []byte(diff), 0644); err != nil {
			s.Error("Failed to write diff: ", err)
		}
		s.Fatal("Unexpected user settings were retrieved. Diff is found in ", diffName)
	}
}
