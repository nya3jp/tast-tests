// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
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
		SoftwareDeps: []string{"gsc", "reboot"},
	})
}

const (
	characters   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
	keyLabel1    = "lecred1"
	keyLabel2    = "lecred2"
	goodPin      = "123456"
	badPin       = "000000"
	testPassword = "~"
	user1        = "user1"
	user2        = "user2"
)

func PinWeaver(ctx context.Context, s *testing.State) {
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// Start with clean state.
	helper.CryptohomeClient().RemoveVault(ctx, user1)
	helper.CryptohomeClient().RemoveVault(ctx, user2)

	// Be sure to cleanup when the test is done.
	defer func() {
		helper.CryptohomeClient().RemoveVault(ctx, user1)
		helper.CryptohomeClient().RemoveVault(ctx, user2)
	}()

	testPINs(ctx, user1, user2, true, r, helper, s)

	// Because Cr50 stores state in the firmware, that persists across reboots, this test
	// needs to run before and after a reboot.
	helper.Reboot(ctx)

	testPINs(ctx, user1, user2, false, r, helper, s)
}

func leCredsFromDisk(ctx context.Context, r *hwsecremote.CmdRunnerRemote, s *testing.State) []string {
	outputSlice := make([]string, 10)
	output, err := r.Run(ctx, "/bin/ls", "/home/.shadow/low_entropy_creds")
	if err != nil {
		s.Fatal("Failed to get le creds on disk: ", err)
	}

	labelsStr := string(output)
	labels := strings.Split(labelsStr, "\n")
	for _, label := range labels {
		outputSlice = append(outputSlice, label)
	}

	sort.Strings(outputSlice)
	return outputSlice
}

// testPINs requires |user1| and |user2| because their le credential state is shored in the same place
// and should not conflict.
func testPINs(ctx context.Context, user1, user2 string, resetUsers bool, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
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
		r.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
		cryptohomeHelper.RemoveVault(ctx, user1)
		cryptohomeHelper.RemoveVault(ctx, user2)

		if err := helper.DaemonController().Start(ctx, hwsec.CryptohomeDaemon); err != nil {
			s.Fatal("Failed to start cryptohomeHelper")
		}

		if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}

		if err := cryptohomeHelper.MountVault(ctx, user1, testPassword, "default", true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Failed to create initial user: ", err)
		}
		if err := cryptohomeHelper.AddVaultKey(ctx, user1, testPassword, "default", goodPin, keyLabel1, true); err != nil {
			s.Fatal("Failed to add le credential: ", err)
		}
		if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}
	}

	if err := cryptohomeHelper.MountVault(ctx, user1, goodPin, keyLabel1, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount with PIN: ", err)
	}
	if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	// Supply invalid credentials five times to trigger firmware lockout of the credential.
	for i := 0; i < 5; i++ {
		if err := cryptohomeHelper.MountVault(ctx, user1, badPin, keyLabel1, false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Mount succeeded but should have failed")
		}
	}

	if err := cryptohomeHelper.MountVault(ctx, user1, testPassword, "default", false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user: ", err)
	}
	if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	if err := cryptohomeHelper.MountVault(ctx, user1, goodPin, keyLabel1, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount with PIN: ", err)
	}
	if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	// Create a new user to test removing.
	if err := cryptohomeHelper.MountVault(ctx, user2, testPassword, "default", true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user2: ", err)
	}

	leCredsBeforeAdd := leCredsFromDisk(ctx, r, s)

	if err := cryptohomeHelper.AddVaultKey(ctx, user2, testPassword, "default", goodPin, keyLabel1, true); err != nil {
		s.Fatalf("Failed to add le credential %s, err: %+v", keyLabel1, err)
	}
	if err := cryptohomeHelper.AddVaultKey(ctx, user2, testPassword, "default", goodPin, keyLabel2, true); err != nil {
		s.Fatalf("Failed to add le credential %s, err: %+v", keyLabel2, err)
	}
	if err := cryptohomeHelper.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	leCredsAfterAdd := leCredsFromDisk(ctx, r, s)

	if _, err := cryptohomeHelper.RemoveVault(ctx, user2); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	leCredsAfterRemove := leCredsFromDisk(ctx, r, s)

	if diff := cmp.Diff(leCredsAfterAdd, leCredsBeforeAdd); diff == "" {
		s.Fatal("LE cred not added successfully (-got +want): ", diff)
	}
	if diff := cmp.Diff(leCredsAfterRemove, leCredsBeforeAdd); diff != "" {
		s.Fatal("LE cred not cleaned up successfully (-got +want): ", diff)
	}
}
