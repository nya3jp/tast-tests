// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
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

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Ensure cryptohomed is started and wait for it to be available
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

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
		s.Error("Failed to unmount user vault: ", err)
	}
}
