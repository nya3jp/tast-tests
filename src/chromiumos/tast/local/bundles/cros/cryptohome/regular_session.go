// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func: RegularSession,
		Desc: "Ensures that cryptohome correctly mounts and unmounts regular user sessions",
		Contacts: []string{
			"betuls@chromium.org",
			"jorgelo@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func RegularSession(ctx context.Context, s *testing.State) {
	const (
		testUser = "cryptohome_test@chromium.org"
		testPass = "testme"
	)
	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}
	// Mount user cryptohome for test user.
	if err := cryptohome.CreateVault(ctx, testUser, testPass); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	// Unmount user vault directory and daemon-store directories.
	if err := cryptohome.UnmountVault(ctx, testUser); err != nil {
		s.Log("Failed to unmount user vault: ", err)
	}
}
