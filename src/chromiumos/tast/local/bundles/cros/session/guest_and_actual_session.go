// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"

	"github.com/golang/protobuf/proto"

	"chromiumos/policy/enterprise_management"
	"chromiumos/tast/local/bundles/cros/session/ownership"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GuestAndActualSession,
		Desc: "Ensures that the session_manager correctly handles ownership when a guest signs in before user",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"derat@chromium.org",    // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
	})
}

func GuestAndActualSession(ctx context.Context, s *testing.State) {
	const testUser = "first_user@nowhere.com"

	if err := ownership.SetUpDevice(ctx); err != nil {
		s.Fatal("Failed to reset device ownership: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}

	if err := cryptohome.MountGuest(ctx); err != nil {
		s.Fatal("Failed to mount guest: ", err)
	}

	if err := sm.StartSession(ctx, cryptohome.GuestUser, ""); err != nil {
		s.Fatal("Failed to start guest session: ", err)
	}

	// Note: presumably the guest session would actually stop and the
	// guest dir would be unmounted before a regular user would sign in.
	// This should be revisited later.
	if err := cryptohome.CreateVault(ctx, testUser, ""); err != nil {
		s.Fatalf("Failed to create a vault for %s: %v", testUser, err)
	}

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

	if err := sm.StartSession(ctx, testUser, ""); err != nil {
		s.Fatalf("Failed to start session for %s: %v", testUser, err)
	}

	select {
	case <-wp.Signals:
	case <-ws.Signals:
	case <-ctx.Done():
		s.Fatal("Timed out waiting for PropertyChangeComplete or SetOwnerKeyComplete signal: ", ctx.Err())
	}

	ret, err := sm.RetrievePolicyEx(ctx, ownership.DevicePolicyDescriptor())
	if err != nil {
		s.Fatal("Failed to retrieve policy: ", err)
	}

	pol := &enterprise_management.PolicyData{}
	if err = proto.Unmarshal(ret.PolicyData, pol); err != nil {
		s.Fatal("Failed to parse PolicyData: ", err)
	}

	if *pol.Username != testUser {
		s.Errorf("Unexpected user name: got %s; want %s", *pol.Username, testUser)
	}
}
