// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RetrieveActiveSessions,
		Desc: "Ensures that the session_manager correctly tracks active sessions",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func RetrieveActiveSessions(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart session_manager: ", err)
	}

	const (
		user1 = "first_user@nowhere.com"
		user2 = "second_user@nowhere.com"
	)

	verify := func(as map[string]string, users ...string) {
		if len(as) != len(users) {
			s.Fatalf("Unexpected active sessions: got %v; want %v", as, users)
		}
		for _, u := range users {
			if _, ok := as[u]; !ok {
				s.Fatalf("Unexpected active sessions: got %v; want %v", as, users)
			}
		}
	}

	// Create clean vault.
	if err := cryptohome.RemoveVault(ctx, user1); err != nil {
		s.Fatalf("Failed to clear user dir for %s: %v", user1, err)
	}
	if err := cryptohome.RemoveVault(ctx, user2); err != nil {
		s.Fatalf("Failed to clear user dir for %s: %v", user2, err)
	}
	if err := cryptohome.CreateVault(ctx, user1, ""); err != nil {
		s.Fatalf("Failed to create user dir for %s: %v", user1, err)
	}
	defer cryptohome.RemoveVault(ctx, user1)
	if err := cryptohome.CreateVault(ctx, user2, ""); err != nil {
		s.Fatalf("Failed to create user dir for %s: %v", user2, err)
	}
	defer cryptohome.RemoveVault(ctx, user2)
	// Before removing vaults, let users log out by restarting
	// session_manager.
	defer upstart.RestartJob(ctx, "ui")

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err := session.PrepareChromeForTesting(ctx, sm); err != nil {
		s.Fatal("Failed to prepare Chrome for testing: ", err)
	}

	// Start first session.
	if err = sm.StartSession(ctx, user1, ""); err != nil {
		s.Fatalf("Failed to start new session for %s: %v", user1, err)
	}

	if as, err := sm.RetrieveActiveSessions(ctx); err != nil {
		s.Fatal("Failed to retrieve active sessions: ", err)
	} else {
		verify(as, user1)
	}

	// Add second session.
	if err = sm.StartSession(ctx, user2, ""); err != nil {
		s.Fatalf("Failed to start new session for %s: %v", user2, err)
	}

	if as, err := sm.RetrieveActiveSessions(ctx); err != nil {
		s.Fatal("Failed to retrieve active sessions: ", err)
	} else {
		verify(as, user1, user2)
	}
}
