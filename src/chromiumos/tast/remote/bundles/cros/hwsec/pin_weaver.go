// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PINWeaver,
		Desc: "Checks that LE credentials work",
		Contacts: []string{
			"cros-hwsec@google.com",
		},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
	})
}

const (
	keyLabel1          = "lecred1"
	keyLabel2          = "lecred2"
	goodPIN            = "123456"
	badPIN             = "000000"
	testPassword       = "~"
	user1              = "user1@example.com"
	user2              = "user2@example.com"
	pinLockoutAttempts = 5
)

func PINWeaver(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// Start with clean state. The vault may not even exist, so ignore the error returned.
	helper.CryptohomeClient().RemoveVault(ctx, user1)
	helper.CryptohomeClient().RemoveVault(ctx, user2)

	// Be sure to cleanup when the test is done.
	defer func() {
		helper.CryptohomeClient().RemoveVault(ctx, user1)
		helper.CryptohomeClient().RemoveVault(ctx, user2)
	}()

	testPINs(ctx, true, r, helper, s)

	// Because Cr50 stores state in the firmware, that persists across reboots, this test
	// needs to run before and after a reboot.
	helper.Reboot(ctx)

	testPINs(ctx, false, r, helper, s)
}

func leCredsFromDisk(ctx context.Context, r *hwsecremote.CmdRunnerRemote) ([]string, error) {
	output, err := r.Run(ctx, "/bin/ls", "/home/.shadow/low_entropy_creds")
	if err != nil {
		return nil, err
	}

	labels := strings.Split(string(output), "\n")
	sort.Strings(labels)
	return labels, nil
}

// testPINs requires user1 and user2 because their le credential state is shored in the same place
// and should not conflict.
func testPINs(ctx context.Context, resetUsers bool, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeClient := helper.CryptohomeClient()

	supportsLE, err := cryptohomeClient.SupportsLECredentials(ctx)
	if err != nil {
		s.Fatal("Failed to get supported policies: ", err)
	} else if !supportsLE {
		s.Fatal("Device does not support PinWeaver")
	}

	if resetUsers {
		if err := helper.DaemonController().Stop(ctx, hwsec.CryptohomeDaemon); err != nil {
			s.Fatal("Failed to stop cryptohomeClient")
		}
		// These are to ensure the machine is in a proper state.
		// Error is not check from these calls because the machine could have no users or le creds yet.
		r.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
		cryptohomeClient.RemoveVault(ctx, user1)
		cryptohomeClient.RemoveVault(ctx, user2)

		if err := helper.DaemonController().Start(ctx, hwsec.CryptohomeDaemon); err != nil {
			s.Fatal("Failed to start cryptohomeClient: ", err)
		}

		if err := cryptohomeClient.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}

		if err := cryptohomeClient.MountVault(ctx, "default", hwsec.NewPassAuthConfig(user1, testPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Failed to create initial user: ", err)
		}
		if err := cryptohomeClient.AddVaultKey(ctx, user1, testPassword, "default", goodPIN, keyLabel1, true); err != nil {
			s.Fatal("Failed to add le credential: ", err)
		}

		output, err := cryptohomeClient.GetKeyData(ctx, user1, keyLabel1)
		if err != nil {
			s.Fatal("Failed to get key data: ", err)
		}
		if strings.Contains(output, "auth_locked: true") {
			s.Fatal("Newly created credential is auth locked")
		}

		if err := cryptohomeClient.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}
	}

	if err := testMountCheckViaPIN(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("PIN failed with freshly created cryptohome: ", err)
	}

	// Run this twice to make sure wrong attempts don't sum up past a good attempt.
	if err := almostLockOutPIN(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("Failed to almost lock out PIN: ", err)
	}
	if err := testMountCheckViaPIN(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("PIN failed after almost locking it out: ", err)
	}
	if err := almostLockOutPIN(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("Failed to almost lock out PIN for the second time: ", err)
	}
	if err := testMountCheckViaPIN(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("PIN failed after almost locking it out for the second time: ", err)
	}

	if err := lockOutPIN(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("Failed to lock out PIN: ", err)
	}
	if err := testPINLockedOut(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("Verification of locked out PIN failed: ", err)
	}
	if err := testMountViaPassword(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("Password failed after locking out PIN: ", err)
	}
	if err := testMountCheckViaPIN(ctx, user1, cryptohomeClient); err != nil {
		s.Fatal("PIN failed after locking it out and resetting via password: ", err)
	}

	// Create a new user to test removing.
	if err := cryptohomeClient.MountVault(ctx, "default", hwsec.NewPassAuthConfig(user2, testPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user2: ", err)
	}

	leCredsBeforeAdd, err := leCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if err := cryptohomeClient.AddVaultKey(ctx, user2, testPassword, "default", goodPIN, keyLabel1, true); err != nil {
		s.Fatalf("Failed to add le credential %s: %v", keyLabel1, err)
	}
	if err := cryptohomeClient.AddVaultKey(ctx, user2, testPassword, "default", goodPIN, keyLabel2, true); err != nil {
		s.Fatalf("Failed to add le credential %s: %v", keyLabel2, err)
	}
	if err := cryptohomeClient.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	leCredsAfterAdd, err := leCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if _, err := cryptohomeClient.RemoveVault(ctx, user2); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	leCredsAfterRemove, err := leCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if diff := cmp.Diff(leCredsAfterAdd, leCredsBeforeAdd); diff == "" {
		s.Fatal("LE cred not added successfully")
	}
	if diff := cmp.Diff(leCredsAfterRemove, leCredsBeforeAdd); diff != "" {
		s.Fatal("LE cred not cleaned up successfully (-got +want): ", diff)
	}
}

func getPINLockState(ctx context.Context, user, label string, cryptohomeClient *hwsec.CryptohomeClient) (bool, error) {
	output, err := cryptohomeClient.GetKeyData(ctx, user, label)
	if err != nil {
		return false, errors.Wrap(err, "failed to get key data")
	}
	exp := regexp.MustCompile("auth_locked: (true|false)\n")
	m := exp.FindStringSubmatch(output)
	if m == nil {
		return false, errors.Errorf("failed to parse auth_locked from key data: %s", output)
	}
	return m[1] == "true", nil
}

func testMountCheckViaPIN(ctx context.Context, user string, cryptohomeClient *hwsec.CryptohomeClient) error {
	if err := cryptohomeClient.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPIN), false, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount with PIN")
	}

	accepted, err := cryptohomeClient.CheckVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPIN))
	if err != nil {
		return errors.Wrap(err, "PIN check failed")
	}
	if !accepted {
		return errors.New("PIN rejected in check")
	}

	locked, err := getPINLockState(ctx, user, keyLabel1, cryptohomeClient)
	if err != nil {
		return errors.Wrap(err, "failed to get PIN lock state")
	}
	if locked {
		return errors.New("PIN marked locked when it shouldn't")
	}

	if err := cryptohomeClient.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount after mounting with PIN")
	}
	return nil
}

func testPINLockedOut(ctx context.Context, user string, cryptohomeClient *hwsec.CryptohomeClient) error {
	if err := cryptohomeClient.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPIN), false, hwsec.NewVaultConfig()); err == nil {
		return errors.New("Mount succeeded but should have failed")
	}

	locked, err := getPINLockState(ctx, user, keyLabel1, cryptohomeClient)
	if err != nil {
		return errors.Wrap(err, "failed to get PIN lock state")
	}
	if !locked {
		return errors.New("PIN marked not locked when it should have been")
	}

	accepted, err := cryptohomeClient.CheckVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPIN))
	if err == nil {
		return errors.New("PIN check succeeded but should have failed")
	}
	if accepted {
		return errors.New("PIN check returned true despite an error")
	}
	return nil
}

func almostLockOutPIN(ctx context.Context, user string, cryptohomeClient *hwsec.CryptohomeClient) error {
	for i := 0; i < pinLockoutAttempts-1; i++ {
		if err := cryptohomeClient.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, badPIN), false, hwsec.NewVaultConfig()); err == nil {
			return errors.New("mount succeeded but should have failed")
		}
		locked, err := getPINLockState(ctx, user, keyLabel1, cryptohomeClient)
		if err != nil {
			return errors.Wrap(err, "failed to get PIN lock state")
		}
		if locked {
			return errors.New("PIN marked locked when it shouldn't")
		}
	}
	return nil
}

func lockOutPIN(ctx context.Context, user string, cryptohomeClient *hwsec.CryptohomeClient) error {
	if err := almostLockOutPIN(ctx, user, cryptohomeClient); err != nil {
		return errors.Wrap(err, "failed to almost lock out PIN")
	}

	if err := cryptohomeClient.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, badPIN), false, hwsec.NewVaultConfig()); err == nil {
		return errors.New("mount succeeded but should have failed")
	}

	// TODO(b/234715681): Remove this extra failing attempt once cryptohome is fixed
	// to return auth_locked on the first failure.
	if err := cryptohomeClient.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, badPIN), false, hwsec.NewVaultConfig()); err == nil {
		return errors.New("extra mount attempt succeeded but should have failed")
	}

	locked, err := getPINLockState(ctx, user, keyLabel1, cryptohomeClient)
	if err != nil {
		return errors.Wrap(err, "failed to get PIN lock state")
	}
	if !locked {
		return errors.New("PIN marked not locked when it should have been")
	}

	return nil
}

func testMountViaPassword(ctx context.Context, user string, cryptohomeClient *hwsec.CryptohomeClient) error {
	if err := cryptohomeClient.MountVault(ctx, "default", hwsec.NewPassAuthConfig(user, testPassword), false, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount user via password")
	}
	locked, err := getPINLockState(ctx, user, keyLabel1, cryptohomeClient)
	if err != nil {
		return errors.Wrap(err, "failed to get PIN lock state")
	}
	if locked {
		return errors.New("PIN marked locked when it shouldn't")
	}
	if err := cryptohomeClient.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount after mounting via password")
	}
	return nil
}
