// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AuthSession,
		Desc: "Ensures that cryptohome correctly creates and authenticates an qAuth session",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func AuthSession(ctx context.Context, s *testing.State) {
	const (
		testUser = "cryptohome_test@chromium.org"
	)
	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, testUser)
	if err != nil {
		s.Error("Failed to start Auth session: ", err)
	}
	testing.ContextLogf(ctx, "Auth session ID = %s", authSessionID)
	// Authenticate the same Auth session using authSessionID.
	authenticated, err := cryptohome.AuthenticateAuthSession(ctx, authSessionID)
	if err != nil {
		s.Error("Failed to authenticate Auth session: ", err)
	}
	testing.ContextLogf(ctx, "User authenticated - %t", authenticated)
}
