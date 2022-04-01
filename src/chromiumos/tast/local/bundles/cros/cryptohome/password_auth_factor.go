// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// Specifies whether the UserSecretStash is going to be used in the test
type keysetSelection struct {
	enableUSSExperiment bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PasswordAuthFactor,
		Desc: "Test user secret stash basic password flow",
		Contacts: []string{
			"emaxx@chromium.org",
			"betuls@google.com",
			"cryptohome-core@chromium.org",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "uss",
			Val: keysetSelection{
				enableUSSExperiment: true,
			},
		}, {
			Name: "vk",
			Val: keysetSelection{
				enableUSSExperiment: false,
			},
		}},
	})
}

func PasswordAuthFactor(ctx context.Context, s *testing.State) {
	const (
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		passwordLabel   = "fake_label"
		testFile        = "file"
		testFileContent = "content"
		ussFlagFile     = "/var/lib/cryptohome/uss_enabled"
		shadow          = "/home/.shadow"
		keysetFile      = "master.0"
		ussFile         = "user_secret_stash/uss";
	)
	keysetSelectionParam :=  s.Param().(keysetSelection)
	enableUSS := keysetSelectionParam.enableUSSExperiment
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

	// Wait for cryptohomed becomes available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up obsolete state, in case there's any.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	if (enableUSS) {
		// Enable the UserSecretStash experiment for the duration of the test by
		// creating a flag file that's checked by cryptohomed.
		if err := os.MkdirAll(path.Dir(ussFlagFile), 0755); err != nil {
			s.Fatal("Failed to create the UserSecretStash flag file directory: ", err)
		}
		if err := ioutil.WriteFile(ussFlagFile, []byte{}, 0644); err != nil {
			s.Fatal("Failed to write the UserSecretStash flag file: ", err)
		}
		defer os.Remove(ussFlagFile)
	}

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer client.RemoveVault(ctxForCleanUp, userName)
	if err := client.PreparePersistentVault(ctx, authSessionID,/*ecryptfs=*/ false); err != nil {
		s.Fatal("Failed to prepare new persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	userPath, err := cryptohome.UserPath(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user vault path: ", err)
	}
	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	// Add a password auth factor to the user.
	//
	// AddAuthFactor API is ready to be used by USS, yet for VK we need to use
	// AddCredentials API untill AddAuthFactor supports VaultKeysets.
	if(enableUSS) {
		if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
			s.Fatal("Failed to create persistent user: ", err)
		}
	} else {
		if err := client.AddCredentialsWithAuthSession(ctx, userName, userPassword, authSessionID, false); err != nil {
			s.Fatal("Failed to add credentials with AuthSession: ", err)
		}
	}

	// Check the correct file exists
	hash, err := cryptohome.UserHash(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	if(enableUSS) {
		_, err := os.Stat(filepath.Join(shadow, hash, ussFile))
		if err != nil {
			s.Fatal("USS file not found: ", err)
		}
	} else {
		_, err := os.Stat(filepath.Join(shadow, hash, keysetFile))
		if err != nil {
			s.Fatal("Keyset file not found: ", err)
		}
  }

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authenticate a new auth session via the auth factor and mount the user.
	authSessionID, err = client.StartAuthSession(ctx, userName /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	if err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}
	if err := client.PreparePersistentVault(ctx, authSessionID /*ecryptfs=*/, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that the test file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}
}
