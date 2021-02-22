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
		Attr:         []string{"group:hwsec_destructive_func"},
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
	utility, err := hwsec.NewCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	output, err := utility.GetSupportedKeyPolicies(ctx)
	if err != nil {
		s.Fatal("GetSupportedKeyPolicies failed: ", err)
	}

	// Parse the text output which looks something like:
	// [cryptohome.GetSupportedKeyPoliciesReply.reply] {
	//	 low_entropy_credentials: true
	// }
	// GetSupportedKeyPolicies success.
	if !strings.Contains(string(output), "low_entropy_credentials: true") {
		return
	}

	if preReboot {
		helper.DaemonController().StopCryptohome(ctx)
		r.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
		utility.Remove(ctx, user1)
		utility.Remove(ctx, user2)

		helper.DaemonController().StartCryptohome(ctx)

		s.Log("Waiting on cryptohomed to start")
		testing.Sleep(ctx, 3*time.Second)

		utility.UnmountAll(ctx)

		s.Log("Setting up LE Credential")

		_, err = utility.MountEx(ctx, user1, testPassword, true, "default", []string{})
		if err != nil {
			s.Fatal("Failed to create initial user: ", err)
		}
		_, err = utility.AddKeyEx(ctx, user1, testPassword, "default", goodPin, keyLabel1, true)
		if err != nil {
			s.Fatal("Failed to add le credential: ", err)
		}
		utility.UnmountAll(ctx)
	}

	s.Log("Testing authentication")
	_, err = utility.MountEx(ctx, user1, goodPin, false, keyLabel1, []string{})
	if err != nil {
		s.Fatal("Failed to mount with PIN: ", err)
	}
	utility.UnmountAll(ctx)

	s.Log("Testing lockout")
	for i := 0; i < 5; i++ {
		_, err = utility.MountEx(ctx, user1, badPin, false, keyLabel1, []string{})
		if err == nil {
			s.Fatal("Mount succeeded but should have failed")
		}
	}

	s.Log("Testing reset")
	_, err = utility.MountEx(ctx, user1, testPassword, false, "default", []string{})
	if err != nil {
		s.Fatal("Failed to mount user: ", err)
	}
	utility.UnmountAll(ctx)

	_, err = utility.MountEx(ctx, user1, goodPin, false, keyLabel1, []string{})
	if err != nil {
		s.Fatal("Failed to mount with PIN: ", err)
	}
	utility.UnmountAll(ctx)

	s.Log("Testing LE cred removal on user removal")
	// Create a new user to test removing.
	_, err = utility.MountEx(ctx, user2, testPassword, true, "default", []string{})
	if err != nil {
		s.Fatal("Failed to create user2: ", err)
	}

	leCredsBeforeAdd := getLECredsFromDisk(ctx, r, s)

	_, err = utility.AddKeyEx(ctx, user2, testPassword, "default", goodPin, keyLabel1, true)
	if err != nil {
		s.Fatal("Failed to add le credential: ", err)
	}
	_, err = utility.AddKeyEx(ctx, user2, testPassword, "default", goodPin, keyLabel2, true)
	if err != nil {
		s.Fatal("Failed to add le credential: ", err)
	}
	utility.UnmountAll(ctx)

	leCredsAfterAdd := getLECredsFromDisk(ctx, r, s)

	utility.Remove(ctx, user2)

	leCredsAfterRemove := getLECredsFromDisk(ctx, r, s)

	if reflect.DeepEqual(leCredsAfterAdd, leCredsBeforeAdd) {
		s.Fatal("LE cred not added successfully")
	}
	if !reflect.DeepEqual(leCredsAfterRemove, leCredsBeforeAdd) {
		s.Fatal("LE cred not cleaned up correctly")
	}

	if !preReboot {
		s.Log("Testing remove credential")
		_, err = utility.RemoveKeyEx(ctx, user1, testPassword, keyLabel1)
		if err != nil {
			s.Fatal("Failed to remove key: ", err)
		}
		_, err = utility.Remove(ctx, user1)
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

	runTest(ctx, user1, user2, true, r, helper, s)

	helper.Reboot(ctx)

	runTest(ctx, user1, user2, false, r, helper, s)
}
