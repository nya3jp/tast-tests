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
		Func: KioskSession,
		Desc: "Ensures that cryptohome correctly mounts kiosk sessions",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func KioskSession(ctx context.Context, s *testing.State) {
	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	if err := cryptohome.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}
	// Unmount Vault.
	cryptohome.UnmountVault(ctx, cryptohome.KioskUser)

	// Test if existing kiosk vault can be signed in with AuthSession.
	if err := cryptohome.AuthSessionMountFlow(ctx, true /*Kiosk User*/, cryptohome.KioskUser, "" /* Empty passkey*/, "fake_label", false /*Create User*/); err != nil {
		s.Fatal("Failed to Mount with AuthSession -: ", err)
	}
}
