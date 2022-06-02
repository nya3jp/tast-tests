// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/hwsec"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinWeaver,
		Desc: "Checks that LE credentials work",
		Contacts: []string{
			"kerrnel@chromium.org", // Test author
			"cros-cryptohome-dev@google.com",
		},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
	})
}

const (
	keyLabel1          = "lecred1"
	keyLabel2          = "lecred2"
	goodPin            = "123456"
	badPin             = "000000"
	testPassword       = "~"
	user1              = "user1@example.com"
	user2              = "user2@example.com"
	pinLockoutAttempts = 5
)

func PinWeaver(ctx context.Context, s *testing.State) {
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
	cryptohomeHelper := helper.CryptohomeClient()

	supportsLE, err := cryptohomeHelper.SupportsLECredentials(ctx)
	if err != nil {
		s.Fatal("Failed to get supported policies: ", err)
	} else if !supportsLE {
		s.Fatal("Device does not support PinWeaver")
	}

	if resetUsers {
		if err := helper.DaemonController().Stop(ctx, hwsec.CryptohomeDaemon); err != nil {
			s.Fatal("Failed to stop cryptohomeHelper")
		}
		// These are to ensure the machine is in a proper state.
		// Error is not check from these calls because the machine could have no users or le creds yet.
		r.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
		cryptohomeHelper.RemoveVault(ctx, user1)
		cryptohomeHelper.RemoveVault(ctx, user2)

		if err := helper.DaemonController().Start(ctx, hwsec.CryptohomeDaemon); err != nil {
			s.Fatal("Failed to start cryptohomeHelper")
		}

		if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}

		if err := cryptohomeHelper.MountVault(ctx, "default", hwsec.NewPassAuthConfig(user1, testPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Failed to create initial user: ", err)
		}
		if err := cryptohomeHelper.AddVaultKey(ctx, user1, testPassword, "default", goodPin, keyLabel1, true); err != nil {
			s.Fatal("Failed to add le credential: ", err)
		}

		output, err := cryptohomeHelper.GetKeyData(ctx, user1, keyLabel1)
		if err != nil {
			s.Fatal("Failed to get key data: ", err)
		}
		if strings.Contains(output, "auth_locked: true") {
			s.Fatal("Newly created credential is auth locked")
		}

		if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}
	}

	s.Log("Checking mount and check via good PIN")
	testMountCheckViaPIN(ctx, user1, helper, s)

	s.Log("Checking good PIN resets wrong PIN counter")
	// Run this twice to make sure wrong attempts don't sum up past a good attempt.
	almostLockOutPIN(ctx, user1, helper, s)
	testMountCheckViaPIN(ctx, user1, helper, s)
	almostLockOutPIN(ctx, user1, helper, s)
	testMountCheckViaPIN(ctx, user1, helper, s)

	s.Log("Checking password resets locked PIN")
	lockOutPIN(ctx, user1, helper, s)
	testPINLockedOut(ctx, user1, helper, s)
	testMountViaPassword(ctx, user1, helper, s)
	testMountCheckViaPIN(ctx, user1, helper, s)

	// Create a new user to test removing.
	s.Log("Checking PIN removal")
	if err := cryptohomeHelper.MountVault(ctx, "default", hwsec.NewPassAuthConfig(user2, testPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user2: ", err)
	}

	leCredsBeforeAdd, err := leCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if err := cryptohomeHelper.AddVaultKey(ctx, user2, testPassword, "default", goodPin, keyLabel1, true); err != nil {
		s.Fatalf("Failed to add le credential %s: %v", keyLabel1, err)
	}
	if err := cryptohomeHelper.AddVaultKey(ctx, user2, testPassword, "default", goodPin, keyLabel2, true); err != nil {
		s.Fatalf("Failed to add le credential %s: %v", keyLabel2, err)
	}
	if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	leCredsAfterAdd, err := leCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if _, err := cryptohomeHelper.RemoveVault(ctx, user2); err != nil {
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

func testPINLockState(ctx context.Context, user, label string, expected bool, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	output, err := cryptohomeHelper.GetKeyData(ctx, user, keyLabel1)
	if err != nil {
		s.Fatal("Failed to get key data: ", err)
	}
	if !strings.Contains(output, fmt.Sprintf("auth_locked: %t", expected)) {
		s.Fatalf("Unexpected auth locked PIN state: expected %t, got %v", expected, output)
	}
}

func testMountCheckViaPIN(ctx context.Context, user string, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	if err := cryptohomeHelper.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPin), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount with PIN: ", err)
	}

	if accepted, err := cryptohomeHelper.CheckVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPin)); err != nil || !accepted {
		s.Fatal("PIN check failed but should have succeeded")
	}

	testPINLockState(ctx, user, keyLabel1, false, helper, s)

	if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}
}

func testPINLockedOut(ctx context.Context, user string, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()

	if err := cryptohomeHelper.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPin), false, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Mount succeeded but should have failed")
	}

	// TODO(b/234659188): Run this assertion after fixing cryptohome.
	// testPINLockState(ctx, user, keyLabel1, true, helper, s)

	if accepted, err := cryptohomeHelper.CheckVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, goodPin)); err == nil || accepted {
		s.Fatal("PIN check succeeded but should have failed")
	}
}

func almostLockOutPIN(ctx context.Context, user string, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	for i := 0; i < pinLockoutAttempts-1; i++ {
		if err := cryptohomeHelper.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, badPin), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mount succeeded but should have failed")
		}
		testPINLockState(ctx, user, keyLabel1, false, helper, s)
	}
}

func lockOutPIN(ctx context.Context, user string, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()

	almostLockOutPIN(ctx, user, helper, s)

	if err := cryptohomeHelper.MountVault(ctx, keyLabel1, hwsec.NewPassAuthConfig(user, badPin), false, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Mount succeeded but should have failed")
	}
	// TODO(b/234659188): Run this assertion after fixing cryptohome.
	// testPINLockState(ctx, user, keyLabel1, true, helper, s)
}

func testMountViaPassword(ctx context.Context, user string, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	if err := cryptohomeHelper.MountVault(ctx, "default", hwsec.NewPassAuthConfig(user, testPassword), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user: ", err)
	}
	testPINLockState(ctx, user, keyLabel1, false, helper, s)
	if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}
}
