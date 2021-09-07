// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"os"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

type testParam struct {
	pinWeaverSupported bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeCorruptedKeys,
		Desc: "Checks that the mount and keys works when part of the vaultkeys corrupted",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				// We only support the pin_weaver corrupted version now.
				Name:              "pin_weaver",
				ExtraSoftwareDeps: []string{"pinweaver"},
				Val:               testParam{pinWeaverSupported: true},
			},
		},
	})
}

// CryptohomeCorruptedKeys checks that the mount and keys works when part of the vaultkeys corrupted.
func CryptohomeCorruptedKeys(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()
	daemonController := helper.DaemonController()
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	const (
		user         = util.FirstUsername
		goodPassword = util.FirstPassword
		goodPin      = util.FirstPin
		leCredFiles  = "/home/.shadow/low_entropy_creds/"
	)

	pinSupported := s.Param().(testParam).pinWeaverSupported

	passConfig := hwsec.NewPassAuthConfig(user, goodPassword)

	pinConfig := hwsec.NewPassAuthConfig(user, goodPin)

	defer func(ctx context.Context) {
		// Ensure we remove the user account after the test.
		if err := mountInfo.CleanUpMount(ctx, user); err != nil {
			s.Error("Failed to cleanup: ", err)
		}
	}(ctx)

	// Ensure clean cryptohome.
	if err := mountInfo.CleanUpMount(ctx, user); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}

	// Create the account.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, passConfig, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}

	// Check the mount point information.
	if err := checkExpectUserMountInfo(ctx, mountInfo, user, true); err != nil {
		s.Fatal("User mount point check failed after create account: ", err)
	}

	// Add a PIN login.
	if err := cryptohome.AddVaultKey(ctx, user, goodPassword, util.PasswordLabel, goodPin, util.PinLabel, pinSupported); err != nil {
		s.Fatal("Failed to add pin user vault: ", err)
	}

	// Unmount the vault.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

	// Check the mount point information.
	if err := checkExpectUserMountInfo(ctx, mountInfo, user, false); err != nil {
		s.Fatal("User mount point check failed after unmount: ", err)
	}

	// Mount with PIN.
	if err := cryptohome.MountVault(ctx, util.PinLabel, pinConfig, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}

	// Check the mount point information.
	if err := checkExpectUserMountInfo(ctx, mountInfo, user, true); err != nil {
		s.Fatal("User mount point check failed after login with PIN: ", err)
	}

	// Unmount the vault.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

	// Check the mount point information.
	if err := checkExpectUserMountInfo(ctx, mountInfo, user, false); err != nil {
		s.Fatal("User mount point check failed after unmount: ", err)
	}

	if pinSupported {
		func() {
			if err := daemonController.Stop(ctx, hwsec.CryptohomeDaemon); err != nil {
				s.Fatal("Failed to stop cryptohomed: ", err)
			}
			defer func() {
				if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
					s.Fatal("Failed to start cryptohomed: ", err)
				}
			}()

			// Emulate the corrupted LE Credential.
			if err := os.RemoveAll(leCredFiles); err != nil {
				s.Fatal("Failed to remove the LE creds files: ", err)
			}
		}()

		// Mount with PIN should fail.
		err := cryptohome.MountVault(ctx, util.PinLabel, pinConfig, false, hwsec.NewVaultConfig())
		var exitErr *hwsec.CmdExitError
		if !errors.As(err, &exitErr) {
			s.Fatalf("Unexpected mount error: got %q; want *hwsec.CmdExitError", err)
		}
		if exitErr.ExitCode == 0 {
			s.Fatal("The exit code shouldn't be zero")
		}

		// Check the mount point information.
		if err := checkExpectUserMountInfo(ctx, mountInfo, user, false); err != nil {
			s.Fatal("User mount point check failed after corrupted PIN: ", err)
		}
	}

	// Mount with Password should success.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, passConfig, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}

	// Check the mount point information.
	if err := checkExpectUserMountInfo(ctx, mountInfo, user, true); err != nil {
		s.Fatal("User mount point check failed after login with password: ", err)
	}
}

// checkExpectUserMountInfo checks the mount point information for user and checks the mount status with wantMounted.
func checkExpectUserMountInfo(ctx context.Context, mountInfo *hwsec.CryptohomeMountInfo, user string, wantMounted bool) error {
	mounted, err := mountInfo.IsMounted(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get mount info")
	}
	if mounted != wantMounted {
		return errors.Errorf("unexpected mounted: got %t; want %t", mounted, wantMounted)
	}
	return nil
}
