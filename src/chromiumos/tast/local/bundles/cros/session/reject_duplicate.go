// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RejectDuplicate,
		Desc: "Ensures that the session_manager won't start the same session twice",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"derat@chromium.org",    // session_manager owner
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func RejectDuplicate(ctx context.Context, s *testing.State) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart session_manager: ", err)
	}

	const user = "first_user@nowhere.com"

	// Create clean vault.
	if err := cryptohome.RemoveVault(ctx, user); err != nil {
		s.Fatalf("Failed to remove the vault for %s: %v", user, err)
	}
	if err := cryptohome.CreateVault(ctx, user, ""); err != nil {
		s.Fatalf("Failed to create a vault for %s: %v", user, err)
	}
	defer cryptohome.RemoveVault(ctx, user)

	// Start the first session.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to create session_manager binding: ", err)
	}
	if err = sm.StartSession(ctx, user, ""); err != nil {
		s.Fatalf("Failed to start new session for %s: %v", user, err)
	}

	// Second StartSession() should fail with SessionExists.
	if err = sm.StartSession(ctx, user, ""); err == nil {
		s.Fatalf("Unexpectedly succeeded to start session for %s twice", user)
	} else if e, ok := err.(dbus.Error); !ok || e.Name != "org.chromium.SessionManagerInterface.SessionExists" {
		s.Error("Unexpected error: ", err)
	}
}
