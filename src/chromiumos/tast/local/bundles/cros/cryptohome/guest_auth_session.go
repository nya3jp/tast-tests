// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"io/ioutil"
	"path/filepath"
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
	const (
		testFile        = "file"
		testFileContent = "content"
	)

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
	userPath, err := cryptohome.UserPath(ctx, cryptohome.GuestUser)
	if err != nil {
		s.Fatal("Failed to get user vault path: ", err)
	}

	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	// Unmount and mount again.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	if err := client.PrepareGuestVault(ctx); err != nil {
		s.Fatal("Failed to prepare guest vault: ", err)
	}

	// Verify non-persistentce.
	if _, err := ioutil.ReadFile(filePath); err == nil {
		s.Fatal("File is persisted across guest session boundary")
	}
}
