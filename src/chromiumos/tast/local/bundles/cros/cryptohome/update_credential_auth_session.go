// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
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
		Func: UpdateCredentialAuthSession,
		Desc: "Test if credentials are updated with AuthSession and that the user can mount post update",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@chromium.org",
		},
		Attr: []string{"group:mainline"},
	})
}

func UpdateCredentialAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		updatedPassword = "updatedsecret"
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

	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	if err := cryptohome.CreateUserWithAuthSession(ctx, userName, userPassword, false); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	// Mount the vault for the first time.
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, "fake_label", false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err == nil {
		s.Fatal("Secondary prepare attempt for the same user should fail, but succeeded")
	}

	// Write a test file to verify persistence.
	userPath, err := cryptohome.UserPath(ctx, userName)
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

	// Update credential for the user.
	authSessionID, err = cryptohome.UpdateUserCredentialWithAuthSession(ctx, userName, userPassword, updatedPassword, false, false)
	if err != nil {
		s.Fatal("Failed to update credential: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Authenticate again with new credential.
	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, updatedPassword, "fake_label", false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user with new credential: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault with new credential: ", err)
	}

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}
}
