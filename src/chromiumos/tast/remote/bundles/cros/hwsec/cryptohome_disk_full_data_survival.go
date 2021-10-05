// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/storage/files"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	// testSpaceFillerFilename is the filename of the file in homedir for filling it up.
	testSpaceFillerFilename = "filler01"

	// fillerMinSize is the minimum change in disk space usage after filling it up to consider the "fill-up operation" as successful.
	// The DUT should have more than this amount of free space for this test to complete successfully.
	fillerMinSize = 1024 * 1024 * 1024 // 1GiB
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeDiskFullDataSurvival,
		Desc: "Checks when user's vault is full, user's data are not lost",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm"},
		// Note: Running this test can be expensive because it writes the entire disk twice.
		Timeout: 30 * time.Minute,
	})
}

func fillVaultAndClear(ctx context.Context, utility *hwsec.CryptohomeClient, r hwsec.CmdRunner, username string) error {
	homedir, err := utility.GetHomeUserPath(ctx, username)
	if err != nil {
		return errors.Wrapf(err, "failed to get home directory for %s", username)
	}

	usageBefore, err := utility.GetAccountDiskUsage(ctx, username)
	if err != nil {
		return errors.Wrapf(err, "failed to get disk usage of %q before filling it up", username)
	}

	fillerPath := filepath.Join(homedir, testSpaceFillerFilename)
	cmd := fmt.Sprintf("openssl enc -aes-128-ctr -pass pass:CrOS4tw -nosalt </dev/zero | dd of=%s", shutil.Escape(fillerPath))
	// We do this many times so that we can hammer it hard.
	// If users in the field are out of space, they could be out of space not just once but
	// many times, that is what this is aiming to simulate here.
	for i := 0; i < 30; i++ {
		r.Run(ctx, "sh", "-c", cmd)
		// Note: We don't care if it suceeds or not. If we're filling it to the brim so the command could fail.
		// Wait for a while so if there's any space reclaimation mechanism in place, we can let it do what it wants.
		testing.Sleep(ctx, 1*time.Second)
	}
	// Run sync to flush it to disk. We don't care if it fails.
	r.Run(ctx, "sync")

	usageAfter, err := utility.GetAccountDiskUsage(ctx, username)
	if err != nil {
		return errors.Wrapf(err, "failed to get disk usage of %q after filling it up", username)
	}

	if usageAfter-usageBefore < fillerMinSize {
		return errors.Errorf("didn't manage to fill up the disk, usage before: %d bytes, usage after: %d bytes", usageBefore, usageAfter)
	}

	// Remove the filler so that we now have free space again.
	if _, err := r.Run(ctx, "rm", "-f", fillerPath); err != nil {
		return errors.Wrapf(err, "failed to remove filler file %q", fillerPath)
	}

	return nil
}

func CryptohomeDiskFullDataSurvival(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()

	// Clear any remnant data on the DUT.
	utility.UnmountAndRemoveVault(ctx, util.FirstUsername)
	utility.UnmountAndRemoveVault(ctx, util.SecondUsername)

	// Create two user vault for testing.
	if err := utility.MountVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user1: ", err)
	}
	defer func() {
		if err := utility.UnmountAndRemoveVault(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to remove user1 vault: ", err)
		}
	}()
	if err := utility.MountVault(ctx, util.Password1Label, hwsec.NewPassAuthConfig(util.SecondUsername, util.SecondPassword1), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user1: ", err)
	}
	defer func() {
		if err := utility.UnmountAndRemoveVault(ctx, util.SecondUsername); err != nil {
			s.Error("Failed to remove user2 vault: ", err)
		}
	}()

	// Give the cleanup 10 seconds to finish.
	shortenedCtx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Create test files for both users.
	hf1, err := files.NewHomedirFiles(shortenedCtx, utility, r, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for testing files in user1's home directory: ", err)
	}
	if err = hf1.Clear(shortenedCtx); err != nil {
		s.Fatal("Failed to clear test files in the user1's home directory: ", err)
	}
	if err = hf1.Step(shortenedCtx); err != nil {
		s.Fatal("Failed to initialize the test files in the user1's home directory: ", err)
	}
	hf2, err := files.NewHomedirFiles(shortenedCtx, utility, r, util.SecondUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for testing files in user2's home directory: ", err)
	}
	if err = hf2.Clear(shortenedCtx); err != nil {
		s.Fatal("Failed to clear test files in the user2's home directory: ", err)
	}
	if err = hf2.Step(shortenedCtx); err != nil {
		s.Fatal("Failed to initialize the test files in the user2's home directory: ", err)
	}

	// ROUND 1: Test if anything's affected when both users are mounted and disk is full.

	// While both users are mounted, fill the vault once and check everything works.
	if err = fillVaultAndClear(shortenedCtx, utility, r, util.FirstUsername); err != nil {
		s.Fatal("Failed to fill user1's vault: ", err)
	}
	if err = hf1.Verify(shortenedCtx); err != nil {
		s.Fatal("Failed to verify test files in the user1's home directory in first round: ", err)
	}
	if err = hf1.Step(shortenedCtx); err != nil {
		s.Fatal("Failed to step the test files in the user1's home directory in first round: ", err)
	}
	if err = hf2.Verify(shortenedCtx); err != nil {
		s.Fatal("Failed to verify test files in the user2's home directory in first round: ", err)
	}
	if err = hf2.Step(shortenedCtx); err != nil {
		s.Fatal("Failed to step the test files in the user2's home directory in first round: ", err)
	}

	// Unmount everything, mount only user 2, so that we can test unmounted vaults are not affected when disks are full.
	if err = utility.UnmountAll(shortenedCtx); err != nil {
		s.Fatal("Failed to unmount all: ", err)
	}
	if err := utility.MountVault(shortenedCtx, util.Password1Label, hwsec.NewPassAuthConfig(util.SecondUsername, util.SecondPassword1), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user2 in second round: ", err)
	}

	// Fill up user2's vault while user1's not mounted and check user2.
	if err = fillVaultAndClear(shortenedCtx, utility, r, util.SecondUsername); err != nil {
		s.Fatal("Failed to fill user2's vault: ", err)
	}

	// Check everything's OK.
	if err = hf2.Verify(shortenedCtx); err != nil {
		s.Fatal("Failed to verify test files in the user2's home directory in second round: ", err)
	}
	if err = hf2.Step(shortenedCtx); err != nil {
		s.Fatal("Failed to step the test files in the user2's home directory in second round: ", err)
	}
	if err := utility.MountVault(shortenedCtx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user1 in second round: ", err)
	}
	if err = hf1.Verify(shortenedCtx); err != nil {
		s.Fatal("Failed to verify test files in the user1's home directory in second round: ", err)
	}
	if err = hf1.Step(shortenedCtx); err != nil {
		s.Fatal("Failed to step the test files in the user1's home directory in second round: ", err)
	}
}
