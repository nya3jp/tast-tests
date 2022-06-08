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
		Func: VaultMigration,
		Desc: "Test vault encryption migration from ecryptfs to fscrypt",
		Contacts: []string{
			"dlunev@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "fscrypt_v1",
			ExtraSoftwareDeps: []string{"use_fscrypt_v1"},
		}, {
			Name:              "fscrypt_v2",
			ExtraSoftwareDeps: []string{"use_fscrypt_v2"},
		}},
		Timeout: 60 * time.Second,
	})
}

func VaultMigration(ctx context.Context, s *testing.State) {
	const (
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		testFile        = "file"
		testFileContent = "content"
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Log("Prepare environment")

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

	s.Log("Create ecryptfs vault with a file")
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, false /*ephemeral*/, false /*kiosk*/)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, true /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare ecryptfs vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	userPath, err := cryptohome.UserPath(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user vault path: ", err)
	}

	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	if _, err := ioutil.ReadFile(filePath); err == nil {
		s.Fatal("File is readable after unmount")
	}

	s.Log("Mount for encryption migration")
	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, false /*ephemeral*/, false /*kiosk*/)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PrepareVaultForMigration(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare vault for migration: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if err := client.MigrateToDircrypto(ctx, userName); err != nil {
		s.Fatal("Failed to migrate vault to dircrypto: ", err)
	}

	s.Log("Mount as fscrypt")
	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, false /*ephemeral*/, false /*kiosk*/)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare fscrypt vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}
}
