// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PinWeaverWithAuthFactor,
		Desc: "Checks that LE credentials work with AuthFactor and USS",
		Contacts: []string{
			"hardikgoyal@chromium.org", // Test author
			"cryptohome-core@google.com",
		},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
	})
}

const (
	pinAuthFactorLabel       = "lecred1"
	correctPinSecret         = "123456"
	incorrectPinSecret       = "000000"
	passwordAuthFactorLabel  = "passLabel"
	passwordAuthFactorSecret = "~"
	testUser1                = "testUser1@example.com"
	testUser2                = "testUser2@example.com"
	testFile                 = "file"
	testFileContent          = "content"
	ussFlagFile              = "/var/lib/cryptohome/uss_enabled"
)

func PinWeaverWithAuthFactor(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwsecremote.NewHelper(cmdRunner, s.DUT())

	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	daemonController := helper.DaemonController()

	// Wait for cryptohomed becomes available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	supportsLE, err := client.SupportsLECredentials(ctx)
	if err != nil {
		s.Fatal("Failed to get supported policies: ", err)
	} else if !supportsLE {
		s.Fatal("Device does not support PinWeaver")
	}

	// Clean up obsolete state, in case there's any.
	cmdRunner.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if _, err := client.RemoveVault(ctx, testUser1); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}
	if _, err := client.RemoveVault(ctx, testUser2); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Enable the UserSecretStash experiment for the duration of the test by
	// creating a flag file that's checked by cryptohomed.
	cmdRunner.Run(ctx, "mkdir -p /var/lib/cryptohome")
	cmdRunner.Run(ctx, "touch", ussFlagFile)
	defer cmdRunner.Run(ctx, "rm", ussFlagFile)

	// Setup two users for testing.
	setupUserWithPin(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)
	authenticateWithCorrectPin(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)

	setupUserWithPin(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s)
	authenticateWithCorrectPin(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s)

	testPinLockOut(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)
	removeLeCredential(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s)

	// Because Cr50 stores state in the firmware, that persists across reboots, this test
	// needs to run before and after a reboot.
	helper.Reboot(ctx)

	testPinLockOut(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)
	removeLeCredential(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)

}

// getLeCredsFromDisk gets the LE Credential file from disk.
func getLeCredsFromDisk(ctx context.Context, r *hwsecremote.CmdRunnerRemote) ([]string, error) {
	output, err := r.Run(ctx, "/bin/ls", "/home/.shadow/low_entropy_creds")
	if err != nil {
		return nil, err
	}

	labels := strings.Split(string(output), "\n")
	sort.Strings(labels)
	return labels, nil
}

// setupUserWithPin sets up an user with a password and a pin auth factor.
func setupUserWithPin(ctx, ctxForCleanUp context.Context, userName string, cmdRunner *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	// These are to ensure the machine is in a proper state.
	// Error is not check from these calls because the machine could have no users or le creds yet.
	cmdRunner.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, userName /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctx, authSessionID)

	if err := cryptohomeHelper.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	if err := cryptohomeHelper.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer cryptohomeHelper.Unmount(ctx, userName)

	if err := cryptohomeHelper.AddAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret); err != nil {
		s.Fatal("Failed to add credentials with AuthSession: ", err)
	}

	leCredsBeforeAdd, err := getLeCredsFromDisk(ctx, cmdRunner)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	// Add a pin auth factor to the user.
	if err := cryptohomeHelper.AddPinAuthFactor(ctx, authSessionID, pinAuthFactorLabel, correctPinSecret); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	leCredsAfterAdd, err := getLeCredsFromDisk(ctx, cmdRunner)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if diff := cmp.Diff(leCredsAfterAdd, leCredsBeforeAdd); diff == "" {
		s.Fatal("LE cred not added successfully")
	}

}

// testPinLockOut attempts to lock out a pin and then resets it with password.
func testPinLockOut(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	// Authenticate a new auth session via the new added pin auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for pin authentication: ", err)
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Supply invalid credentials five times to trigger firmware lockout of the credential.
	for i := 0; i < 5; i++ {
		if err := cryptohomeHelper.AuthenticatePinAuthFactor(ctx, authSessionID, pinAuthFactorLabel, incorrectPinSecret); err == nil {
			s.Fatal("Authenticate auth factor succeeded but should have failed")
		}
	}

	// Authenticate the same AuthSession using authSessionID.
	if err := cryptohomeHelper.AuthenticateAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret); err != nil {
		s.Fatal("Authenticate auth factor with password failed")
	}

	if err := cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID); err != nil {
		s.Fatal("Failed to invalidate AuthSession")
	}
}

// authenticateWithCorrectPin authenticates a given user with the correct pin.
func authenticateWithCorrectPin(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	// Authenticate a new auth session via the new added pin auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for pin authentication: ", err)
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := cryptohomeHelper.AuthenticatePinAuthFactor(ctx, authSessionID, pinAuthFactorLabel, correctPinSecret); err != nil {
		s.Fatal("Authenticate auth factor with pin failed: ", err)
	}

	if err := cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID); err != nil {
		s.Fatal("Failed to invalidate AuthSession: ", err)
	}
}

// removeLeCredential removes testUser and checks to see if the leCreds on disk was updated.
func removeLeCredential(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()

	leCredsBeforeRemove, err := getLeCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if _, err := cryptohomeHelper.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	leCredsAfterRemove, err := getLeCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if diff := cmp.Diff(leCredsAfterRemove, leCredsBeforeRemove); diff != "" {
		s.Fatal("LE cred not cleaned up successfully (-got +want): ", diff)
	}
}
