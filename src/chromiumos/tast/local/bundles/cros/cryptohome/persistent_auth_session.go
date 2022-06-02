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
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type vaultType int64

type params struct {
	VaultType vaultType
}

const (
	noneVaultType vaultType = iota
	ecryptfsVaultType
	fscryptV1VaultType
	defaultVaultType
)

const (
	cleanupTime = 20 * time.Second
	testTime    = 60 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PersistentAuthSession,
		Desc: "Test new create/prepare API for persistent vault with auth session",
		Contacts: []string{
			"dlunev@chromium.org",
			"hardikgoyal@chromium.org",
			"cryptohome-core@chromium.org",
		},
		Attr: []string{"group:mainline"},
		Params: []testing.Param{{
			Name: "default",
			Val:  &params{VaultType: defaultVaultType},
		}, {
			Name: "ecryptfs",
			Val:  &params{VaultType: ecryptfsVaultType},
		}, {
			Name:              "fscrypt_v1",
			ExtraSoftwareDeps: []string{"use_fscrypt_v2"},
			Val:               &params{VaultType: fscryptV1VaultType},
		}},
		Timeout: testTime,
	})
}

func PersistentAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		testFile        = "file"
		testFileContent = "content"
	)

	vtype := s.Param().(*params).VaultType
	isEcryptfs := vtype == ecryptfsVaultType

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
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

	if vtype == fscryptV1VaultType {
		s.Log("Switch cryptohome to fscryptv1 ")
		if err := upstart.RestartJob(ctx, "cryptohomed", upstart.WithArg("CRYPTOHOMED_ARGS", "--negate_fscrypt_v2_for_test")); err != nil {
			s.Fatal("Can't disable fscryptv2: ", err)
		}
		defer upstart.RestartJob(ctx, "cryptohomed")
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

	if err := client.PreparePersistentVault(ctx, authSessionID, isEcryptfs); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if err := client.PreparePersistentVault(ctx, authSessionID, isEcryptfs); err == nil {
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

	if _, err := ioutil.ReadFile(filePath); err == nil {
		s.Fatal("File is readable after unmount")
	}

	if vtype == fscryptV1VaultType {
		if err := upstart.RestartJob(ctx, "cryptohomed"); err != nil {
			s.Fatal("Can't reset cryptohomed")
		}
	}

	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, "fake_label", false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, isEcryptfs); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}
}
