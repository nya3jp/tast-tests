// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KioskWithLegacyMount,
		Desc: "Ensures that cryptohome correctly mounts kiosk sessions through various Auth APIs",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// KioskWithLegacyMount tests the following case:
// User Created with Legacy mountEx call (this will involve KEY_TYPE_PASSWORD):
// Ensure that the user can login with mountEx call
// Ensure that the user can login with Credential APIs
// Ensure that the user can login with AuthFactor APIs
// Ensure that the user can login with AuthFactor APIs with USS Enabled
func KioskWithLegacyMount(ctx context.Context, s *testing.State) {
	const (
		testFile        = "file"
		testFileContent = "content"
		cleanupTime     = 20 * time.Second
	)

	ctxForCleanUp := ctx
	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	// Ensure the vault had been removed.
	if err := cryptohome.RemoveVault(ctx, cryptohome.KioskUser); err != nil {
		s.Log("Failed to remove vault before test starts: ", err)
	}

	// Create and mount the kiosk for the first time.
	if err := cryptohome.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, cryptohome.KioskUser)
	defer cryptohome.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	userPath, err := cryptohome.UserPath(ctx, cryptohome.KioskUser)
	if err != nil {
		s.Fatal("Failed to get kiosk user vault path: ", err)
	}
	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}

	if _, err := ioutil.ReadFile(filePath); err == nil {
		s.Fatal("File is readable after unmount")
	}

	//	Ensure that the user can login with mountEx call
	if err := cryptohome.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}

	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}

	// Authenticate a new auth session via the new added pin auth factor and mount the user.
	authSessionID, err := client.StartAuthSession(ctx, cryptohome.KioskUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)

	if err = client.AuthenticateAuthSession(ctx, "", "public_mount", authSessionID, true /*=isKioskUser*/); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}

	if err := mountAndVerify(ctx, client, authSessionID, filePath); err != nil {
		s.Fatal("Failed to mount and verify persistence: ", err)
	}

	// Ensure that Kiosk login works when USS flag is disabled, but should
	// still work with AuthFactor API.
	authSessionID, err = client.StartAuthSession(ctx, cryptohome.KioskUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)
	if err = client.AuthenticateKioskAuthFactor(ctx, authSessionID, "public_mount"); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}

	if err := mountAndVerify(ctx, client, authSessionID, filePath); err != nil {
		s.Fatal("Failed to mount and verify persistence: ", err)
	}

	// Enable the UserSecretStash experiment for the remainder of the test by
	// creating a flag file that's checked by cryptohomed.
	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment()

	// Ensure that Kiosk login works when USS flag is enabled.
	authSessionID, err = client.StartAuthSession(ctx, cryptohome.KioskUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	if err = client.AuthenticateKioskAuthFactor(ctx, authSessionID, "public_mount"); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)

	if err := mountAndVerify(ctx, client, authSessionID, filePath); err != nil {
		s.Fatal("Failed to mount and verify persistence: ", err)
	}
}

func mountAndVerify(ctx context.Context, client *hwsec.CryptohomeClient, authSessionID, filePath string) error {
	const (
		testFile        = "file"
		testFileContent = "content"
	)
	if err := client.PreparePersistentVault(ctx, authSessionID /*ecryptfs=*/, false); err != nil {
		return errors.Wrap(err, "failed to prepare persistent vault")
	}

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		return errors.Wrap(err, "failed to read test file")
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		return errors.Wrap(err, "incorrect tests file content")
	}

	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}
	return nil
}
