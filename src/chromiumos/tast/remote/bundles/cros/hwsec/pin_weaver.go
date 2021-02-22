// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"math/rand"
	"reflect"
	"strings"
	"time"

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
		SoftwareDeps: []string{"tpm", "reboot"},
	})
}

const (
	characters   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
	keyLabel1    = "lecred1"
	keyLabel2    = "lecred2"
	goodPin      = "123456"
	badPin       = "000000"
	testPassword = "~"
)

func randomUsername() string {
	bytes := make([]byte, 10)
	for i := range bytes {
		bytes[i] = characters[rand.Intn(len(characters))]
	}
	return string(bytes) + "@gmail.com"
}

func getLECredsFromDisk(ctx context.Context, r *hwsecremote.CmdRunnerRemote, s *testing.State) map[string]bool {
	outputMap := make(map[string]bool)
	output, err := r.Run(ctx, "/bin/ls", "/home/.shadow/low_entropy_creds")
	if err != nil {
		s.Fatal("Failed to get le creds on disk: ", err)
	}

	labelsStr := string(output)
	labels := strings.Split(labelsStr, "\n")
	for _, label := range labels {
		outputMap[label] = true
	}
	return outputMap
}

func runTest(ctx context.Context, user1, user2 string, preReboot bool, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.HelperRemote, s *testing.State) {
	utility := helper.CryptohomeClient()

	supportsLE, err := utility.SupportsLECredentials(ctx)
	if err != nil {
		s.Fatal("Failed to get supported policies: ", err)
	}
	if !supportsLE {
		s.Fatal("Device does not support PinWeaver")
	}

	if preReboot {
		helper.DaemonController().StopCryptohome(ctx)
		r.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
		utility.RemoveVault(ctx, user1)
		utility.RemoveVault(ctx, user2)

		helper.DaemonController().StartCryptohome(ctx)

		s.Log("Waiting on cryptohomed to start")
		testing.Sleep(ctx, 3*time.Second)

		err = utility.UnmountAll(ctx)
		if err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}

		err = utility.MountVault(ctx, user1, testPassword, "default", true, hwsec.NewVaultConfig())
		if err != nil {
			s.Fatal("Failed to create initial user: ", err)
		}
		err = utility.AddVaultKey(ctx, user1, testPassword, "default", goodPin, keyLabel1, true)
		if err != nil {
			s.Fatal("Failed to add le credential: ", err)
		}
		utility.UnmountAll(ctx)
		if err != nil {
			s.Fatal("Failed to unmountAll: ", err)
		}
	}

	err = utility.MountVault(ctx, user1, goodPin, keyLabel1, false, hwsec.NewVaultConfig())
	if err != nil {
		s.Fatal("Failed to mount with PIN: ", err)
	}
	utility.UnmountAll(ctx)
	if err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	for i := 0; i < 5; i++ {
		err = utility.MountVault(ctx, user1, badPin, keyLabel1, false, hwsec.NewVaultConfig())
		if err == nil {
			s.Fatal("Mount succeeded but should have failed")
		}
	}

	err = utility.MountVault(ctx, user1, testPassword, "default", false, hwsec.NewVaultConfig())
	if err != nil {
		s.Fatal("Failed to mount user: ", err)
	}
	utility.UnmountAll(ctx)
	if err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	err = utility.MountVault(ctx, user1, goodPin, keyLabel1, false, hwsec.NewVaultConfig())
	if err != nil {
		s.Fatal("Failed to mount with PIN: ", err)
	}
	utility.UnmountAll(ctx)
	if err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	// Create a new user to test removing.
	err = utility.MountVault(ctx, user2, testPassword, "default", true, hwsec.NewVaultConfig())
	if err != nil {
		s.Fatal("Failed to create user2: ", err)
	}

	leCredsBeforeAdd := getLECredsFromDisk(ctx, r, s)

	err = utility.AddVaultKey(ctx, user2, testPassword, "default", goodPin, keyLabel1, true)
	if err != nil {
		s.Fatal("Failed to add le credential: ", err)
	}
	err = utility.AddVaultKey(ctx, user2, testPassword, "default", goodPin, keyLabel2, true)
	if err != nil {
		s.Fatal("Failed to add le credential: ", err)
	}
	utility.UnmountAll(ctx)
	if err != nil {
		s.Fatal("Failed to unmountAll: ", err)
	}

	leCredsAfterAdd := getLECredsFromDisk(ctx, r, s)

	_, err = utility.RemoveVault(ctx, user2)
	if err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	leCredsAfterRemove := getLECredsFromDisk(ctx, r, s)

	if reflect.DeepEqual(leCredsAfterAdd, leCredsBeforeAdd) {
		s.Fatal("LE cred not added successfully")
	}
	if !reflect.DeepEqual(leCredsAfterRemove, leCredsBeforeAdd) {
		s.Fatal("LE cred not cleaned up correctly")
	}
}

func PinWeaver(ctx context.Context, s *testing.State) {
	rand.Seed(time.Now().UTC().UnixNano())

	user1 := randomUsername()
	user2 := randomUsername()

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

	runTest(ctx, user1, user2, true, r, helper, s)

	// Because Cr50 stores state in the firmware, that persists across reboots, this test
	// needs to run before and after a reboot.
	helper.Reboot(ctx)

	runTest(ctx, user1, user2, false, r, helper, s)
}
