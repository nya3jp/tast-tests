// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11/pkcs11test"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	libhwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// pkcs11InitUnderErrorsWithAuthAPIParam contains the test parameters that specifies the type of storage.
type pkcs11InitUnderErrorsWithAuthAPIParam struct {
	// Specifies whether to use secret stash.
	useUserSecretStash bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Pkcs11InitUnderErrors,
		Desc: "Tests pkcs11 initialization under various system states",
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "uss",
			Val: pkcs11InitUnderErrorsWithAuthAPIParam{
				useUserSecretStash: true,
			},
		}, {
			Name: "vk",
			Val: pkcs11InitUnderErrorsWithAuthAPIParam{
				useUserSecretStash: false,
			},
		}},
		Contacts: []string{
			"chenyian@google.com",
			"cros-hwsec@chromium.org",
		},
		Timeout: 4 * time.Minute,
	})
}

// testToken test the token stores in chapsPath.
func testToken(ctx context.Context, r hwsec.CmdRunner, chapsPath string) error {
	slot, err := pkcs11test.LoadP11TestToken(ctx, r, chapsPath, "auth")
	if err != nil {
		return errors.Wrap(err, "failed to load token using chaps_client")
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--generate", "--replay_wifi"); err != nil {
		return errors.Wrap(err, "p11_replay generate failed")
	}
	if _, err := r.Run(ctx, "p11_replay", "--slot="+slot, "--cleanup", "--replay_wifi"); err != nil {
		return errors.Wrap(err, "p11_replay cleanup failed")
	}
	if err := pkcs11test.UnloadP11TestToken(ctx, r, chapsPath); err != nil {
		return errors.Wrap(err, "failed to unload token using chaps_client")
	}
	return nil
}

// Pkcs11InitUnderErrors test the chapsd pkcs11 initialization under various system states.
func Pkcs11InitUnderErrors(ctx context.Context, s *testing.State) {
	userParam := s.Param().(pkcs11InitUnderErrorsWithAuthAPIParam)

	r := libhwseclocal.NewCmdRunner()

	helper, err := libhwseclocal.NewHelper(r)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()
	cryptohome.SetMountAPIParam(&hwsec.CryptohomeMountAPIParam{MountAPI: hwsec.AuthFactorMountAPI})

	if userParam.useUserSecretStash {
		// Enable the UserSecretStash experiment for the duration of the test by
		// creating a flag file that's checked by cryptohome.
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment(ctx)
	}

	// Ensure that the user directory is unmounted and does not exist.
	if err := util.CleanupUserMount(ctx, cryptohome); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}
	// Create a user vault for the slot/token.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}

	cleanupCtx := ctx
	// Give cleanup function 20 seconds to remove scratchpad.
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		// Remove the user account after the test.
		if err := util.CleanupUserMount(ctx, cryptohome); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}(cleanupCtx)

	sanitizedUsername, err := cryptohome.GetSanitizedUsername(ctx, util.FirstUsername, true)
	if err != nil {
		s.Fatal("Failed to get sanitized username: ", err)
	}
	userChapsPath := filepath.Join("/run/daemon-store/chaps/", sanitizedUsername)
	userDbPath := filepath.Join(userChapsPath, "/database")

	// Make sure the test token is functional after generated.
	if err := testToken(ctx, r, userChapsPath); err != nil {
		s.Error("Test Token not working after generated: ", err)
	}

	// Erase the chaps database directory. Chaps should regenerate the database and carry on the test.
	if err := os.RemoveAll(userDbPath); err != nil {
		s.Error("Failed to remove the database directory: ", err)
	}
	if err := testToken(ctx, r, userChapsPath); err != nil {
		s.Error("Test Token not working after removed database: ", err)
	}

	// Deliberately corrupt the files in the chaps database directory. Chaps should detect the error, regenerate the database, and carry on the test.
	files, err := ioutil.ReadDir(userDbPath)
	if err != nil {
		s.Error("Read db directory error: ", err)
	}
	for _, file := range files {
		if _, err := r.Run(ctx, "dd", "if=/dev/zero", "of="+userDbPath+file.Name(), "bs=1", "count=1000"); err != nil {
			s.Error("Failed to corrupt the database file: ", err)
		}
	}
	if err := testToken(ctx, r, userChapsPath); err != nil {
		s.Error("Test Token not working after database corrupted: ", err)
	}
}
