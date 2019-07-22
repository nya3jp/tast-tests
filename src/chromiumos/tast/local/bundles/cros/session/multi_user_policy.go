// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"

	"github.com/golang/protobuf/proto"

	"chromiumos/policy/enterprise_management"
	"chromiumos/tast/local/bundles/cros/session/ownership"
	"chromiumos/tast/local/bundles/cros/session/util"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MultiUserPolicy,
		Desc: "Verifies that storing and retrieving user policy works with multiple profiles signed-in",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		Data: []string{"testcert.p12"},
	})
}

func MultiUserPolicy(ctx context.Context, s *testing.State) {
	const (
		user1 = "user1@somewhere.com"
		user2 = "user2@somewhere.com"
	)
	desc1 := ownership.UserPolicyDescriptor(user1)
	desc2 := ownership.UserPolicyDescriptor(user2)

	privKey, err := ownership.ExtractPrivKey(s.DataPath("testcert.p12"))
	if err != nil {
		s.Fatal("Failed to parse PKCS #12 file: ", err)
	}

	var settings enterprise_management.ChromeDeviceSettingsProto
	policy, err := ownership.BuildPolicy("", privKey, nil, &settings)
	if err != nil {
		s.Fatal("Failed to build test policy data: ", err)
	}
	empty := &enterprise_management.PolicyFetchResponse{}

	if err := ownership.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	// Clear the users' vault to make sure the test starts without any
	// policy or key lingering around. At this stage, the session isn't
	// started and there's no user signed in.
	if err := cryptohome.RemoveVault(ctx, user1); err != nil {
		s.Fatalf("Failed to remove vault for %s: %v", user1, err)
	}
	if err := cryptohome.CreateVault(ctx, user1, ""); err != nil {
		s.Fatalf("Failed to create vault for %s: %v", user1, err)
	}
	if err := cryptohome.RemoveVault(ctx, user2); err != nil {
		s.Fatalf("Failed to remove vault for %s: %v", user2, err)
	}
	if err := cryptohome.CreateVault(ctx, user2, ""); err != nil {
		s.Fatalf("Failed to create vault for %s: %v", user2, err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := util.PrepareChromeForTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

	// Start a session for the first user, and verify that no policy
	// exists for that user yet.
	if err := sm.StartSession(ctx, user1, ""); err != nil {
		s.Fatalf("Failed to start session for %s: %v", user1, err)
	}
	if ret, err := sm.RetrievePolicyEx(ctx, desc1); err != nil {
		s.Fatalf("Failed to retrieve policy for %s: %v", user1, err)
	} else if !proto.Equal(ret, empty) {
		s.Fatal("Unexpected policy is fetched for ", user1)
	}

	// Then, store the policy.
	if err := sm.StorePolicyEx(ctx, desc1, policy); err != nil {
		s.Fatalf("Failed to store policy for %s: %v", user1, err)
	}

	// Storing policy for the second user fails before the session starts.
	if err := sm.StorePolicyEx(ctx, desc2, policy); err == nil {
		s.Fatalf("Unexpectedly succeeded to store policy for %s: %v", user2, err)
	}

	// Starts the second user's session, and verify that it has no
	// policy stored yet.
	if err := sm.StartSession(ctx, user2, ""); err != nil {
		s.Fatalf("Failed to start session for %s: %v", user1, err)
	}
	if ret, err := sm.RetrievePolicyEx(ctx, desc2); err != nil {
		s.Fatalf("Failed to retrieve policy for %s: %v", user2, err)
	} else if !proto.Equal(ret, empty) {
		s.Fatal("Unexpected policy is fetched for ", user2)
	}

	// Strong the policy for the second user should work now.
	if err := sm.StorePolicyEx(ctx, desc2, policy); err != nil {
		s.Fatalf("Failed to store policy for %s: %v", user2, err)
	}

	// Verify that retrieving policy for the second user works, too.
	if _, err := sm.RetrievePolicyEx(ctx, desc2); err != nil {
		s.Fatalf("Failed to retrieve policy for %s: %v", user2, err)
	}
}
