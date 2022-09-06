// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"bytes"
	"context"
	"crypto/x509"
	"io/ioutil"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/session/ownership"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OwnershipRetaken,
		Desc: "Ensures that ownership is re-taken upon loss of owner's cryptohome",
		Contacts: []string{
			"hidehiko@chromium.org",
		},
		Data:         []string{"testcert.p12"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

func OwnershipRetaken(ctx context.Context, s *testing.State) {
	const (
		testUser = "ownership_test@chromium.org"
		testPass = "testme"
	)

	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// Resets the TPM, system, and user states before running the tests.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}

	privKey, err := session.ExtractPrivKey(s.DataPath("testcert.p12"))
	if err != nil {
		s.Fatal("Failed to parse PKCS #12 file: ", err)
	}

	if err := session.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	if err = cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

	// Pre-configure some owner settings, including initial key.
	settings := ownership.BuildTestSettings(testUser)
	if err := session.StoreSettings(ctx, sm, testUser, privKey, nil, settings); err != nil {
		s.Fatal("Failed to store settings: ", err)
	}

	// Grab key, ensure that it's the same as the known key.
	verifyOwnerKey := func() (bool, error) {
		path := filepath.Join(session.PolicyPath, "owner.key")
		pubKey, err := ioutil.ReadFile(path)
		if err != nil {
			return false, errors.Wrap(err, "failed to read policy")
		}
		pubDer, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
		if err != nil {
			return false, errors.Wrap(err, "failed to marshal public key to DER")
		}
		return bytes.Equal(pubKey, pubDer), nil
	}
	if same, err := verifyOwnerKey(); err != nil {
		s.Fatal("Failed to check owner key: ", err)
	} else if !same {
		s.Fatal("Owner key should not have changed")
	}

	// Start a new session, which will trigger the re-taking of ownership.
	wp, err := sm.WatchPropertyChangeComplete(ctx)
	if err != nil {
		s.Fatal("Failed to start watching PropertyChangeComplete signal: ", err)
	}
	defer wp.Close(ctx)
	ws, err := sm.WatchSetOwnerKeyComplete(ctx)
	if err != nil {
		s.Fatal("Failed to start watching SetOwnerKeyComplete signal: ", err)
	}
	defer ws.Close(ctx)

	// if err = cryptohome.CreateVault(ctx, testUser, testPass); err != nil {
	// 	s.Fatal("Failed to create vault: ", err)
	// }
	// if err = sm.StartSession(ctx, testUser, ""); err != nil {
	// 	s.Fatalf("Failed to start new session for %s: %v", testUser, err)
	// }

	s.Log("MIERSH try loging in with chrome.KeepState()")
	normalChrome, err := chrome.New(ctx, chrome.KeepState(), chrome.FakeLogin(chrome.Creds{User: testUser, Pass: "123"}))
	if err != nil {
		s.Fatal("Failed to log in with Chrome: ", err)
	}
	defer normalChrome.Close(ctx)
	s.Log("MIERSH login done")

	select {
	case <-wp.Signals:
	case <-ws.Signals:
	case <-ctx.Done():
		s.Fatal("Timed out waiting for PropertyChangeComplete or SetOwnerKeyComplete signal: ", ctx.Err())
	}

	// Grab key, ensure that it's different than known key.
	if same, err := verifyOwnerKey(); err != nil {
		s.Fatal("Failed to check owner key: ", err)
	} else if same {
		s.Fatal("Owner key should have changed")
	}

	// Fetch the data from the session_manager.
	ret, err := session.RetrieveSettings(ctx, sm)
	if err != nil {
		s.Fatal("Failed to retrieve settings: ", err)
	}

	// Verify that there's no diff between sent data and fetched data.
	if diff := cmp.Diff(settings, ret, protocmp.Transform()); diff != "" {
		const diffName = "diff.txt"
		if err = ioutil.WriteFile(filepath.Join(s.OutDir(), diffName), []byte(diff), 0644); err != nil {
			s.Error("Failed to write diff: ", err)
		}
		s.Error("Sent data and fetched data has diff, which is found in ", diffName)
	}
}
