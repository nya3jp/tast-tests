// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GuestAuthSession,
		Desc: "Test guest sessions with auth session API",
		Contacts: []string{
			"dlunev@chromium.org",
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func GuestAuthSession(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Ensure cryptohomed is started and wait for it to be available.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	// Set up a guest session
	if err := client.PrepareGuestVault(ctx); err != nil {
		s.Fatal("Failed to prepare guest vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if err := client.PrepareGuestVault(ctx); err == nil {
		s.Fatal("Secondary guest attempt should fail, but succeeded")
	}

	// Write a test file to verify non-persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.GuestUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	// Unmount and mount again.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	if err := client.PrepareGuestVault(ctx); err != nil {
		s.Fatal("Failed to prepare guest vault: ", err)
	}

	// Verify non-persistence.
	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.GuestUser); err != nil {
		s.Fatal("File is persisted across guest session boundary")
	}
}
