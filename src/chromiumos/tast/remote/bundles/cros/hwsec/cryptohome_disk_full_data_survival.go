// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
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
	testSpaceFillerFilenamePrefix = "filler"

	// initialFillIterations is the number of times we'll try to fill the user's home directory.
	// There will be a gap of 2s between each iteration. This setting depends on how quick Chrome OS
	// is able to clean up a user's home directory. If it takes longer, then this setting should be
	// larger.
	initialFillIterations = 45

	// waitForFilledTimeout is the amount of time we'll wait until the disk space drop to 0.
	waitForFilledTimeout = 30 * time.Second
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

// getHomeAvailableSpace retrieves the available disk space in MiB.
func getHomeAvailableSpace(ctx context.Context, utility *hwsec.CryptohomeClient, r hwsec.CmdRunner, username string) (int, error) {
	homedir, err := utility.GetHomeUserPath(ctx, username)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to get home directory for %s", username)
	}

	raw, err := r.Run(ctx, "df", "--output=avail", "-BM", homedir)
	if err != nil {
		return -1, errors.Errorf("failed to run df on %q", homedir)
	}

	out := string(raw)
	arr := strings.Split(out, "\n")
	if len(arr) < 2 {
		return -1, errors.Errorf("df output is less than 2 line: %q", out)
	}
	spaceStr := strings.TrimSpace(arr[1])
	if len(spaceStr) == 0 || spaceStr[len(spaceStr)-1] != 'M' {
		return -1, errors.Errorf("Space from df is incorrect: %q", spaceStr)
	}
	space, err := strconv.Atoi(spaceStr[0 : len(spaceStr)-1])
	if err != nil {
		return -1, errors.Wrapf(err, "Space from df is not integer: %q", spaceStr)
	}

	return space, nil
}

// getFillerPath will return the path to the ith filler file in the user's homedir.
func getFillerPath(homedir string, i int) string {
	return filepath.Join(homedir, fmt.Sprintf("%s%02d", testSpaceFillerFilenamePrefix, i))
}

// fillFile will write data to the filler file i in user's homedir until it's full.
func fillFile(ctx context.Context, homedir string, i int, r hwsec.CmdRunner) {
	// Run command to fill the file up.
	cmd := fmt.Sprintf("openssl enc -aes-128-ctr -iter 1 -pass pass:CrOS4tw -nosalt </dev/zero | dd of=%s", shutil.Escape(getFillerPath(homedir, i)))
	r.Run(ctx, "sh", "-c", cmd)

	// Run sync to flush it to disk. We don't care if it fails.
	r.Run(ctx, "sync")
}

// fillVaultAndClear fills the user's vault to the brim then removes the filler.
func fillVaultAndClear(ctx context.Context, utility *hwsec.CryptohomeClient, r hwsec.CmdRunner, username string) error {
	homedir, err := utility.GetHomeUserPath(ctx, username)
	if err != nil {
		return errors.Wrapf(err, "failed to get home directory for %s", username)
	}

	// We do this many times so that we can hammer it hard.
	// If users in the field are out of space, they could be out of space not just once but
	// many times, that is what this is aiming to simulate here.
	for i := 0; i < initialFillIterations; i++ {
		fillFile(ctx, homedir, i, r)

		// Note: We don't care if it succeeds or not. If we're filling it to the brim so the command could fail.
		// Wait for a while so if there's any space reclamation mechanism in place, we can let it do what it wants.
		testing.Sleep(ctx, 2*time.Second)
	}

	// Check if the disk indeed filled.
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		avail, err := getHomeAvailableSpace(ctx, utility, r, username)
		if err != nil {
			return errors.Wrapf(err, "failed to get disk available space of %q after filling it up", username)
		}
		if avail != 0 {
			// Try to fill again if it's not full.
			fillFile(ctx, homedir, initialFillIterations, r)

			return errors.Errorf("didn't manage to fill up the disk, %d MiB is still available", avail)
		}

		return nil
	}, &testing.PollOptions{Timeout: waitForFilledTimeout}); err != nil {
		return errors.Wrap(err, "failed to wait till the home is filled")
	}

	// Remove the filler so that we now have free space again.
	for i := 0; i < initialFillIterations+1; i++ {
		fillerPath := getFillerPath(homedir, i)
		if _, err := r.Run(ctx, "rm", "-f", fillerPath); err != nil {
			return errors.Wrapf(err, "failed to remove filler file %q", fillerPath)
		}
	}

	return nil
}

// newHomedirFilesAndClear create HomedirFiles object and reset their state to what is needed for testing.
func newHomedirFilesAndClear(ctx context.Context, utility *hwsec.CryptohomeClient, r hwsec.CmdRunner, username string) (*files.HomedirFiles, error) {
	res, err := files.NewHomedirFiles(ctx, utility, r, username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create HomedirFiles for testing files in %q's home directory", username)
	}
	if err = res.Clear(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to clear test files in the %q's home directory", username)
	}
	if err = res.Step(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to initialize the test files in the %q's home directory", username)
	}
	return res, nil
}

// verifyAndStep runs Verify() and Step() on the given test files.
func verifyAndStep(ctx context.Context, hf *files.HomedirFiles) error {
	if err := hf.Verify(ctx); err != nil {
		return errors.Wrap(err, "failed to verify test files")
	}
	if err := hf.Step(ctx); err != nil {
		return errors.Wrap(err, "failed to step test files")
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
		s.Fatal("Failed to create user2: ", err)
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
	hf1, err := newHomedirFilesAndClear(shortenedCtx, utility, r, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for user1: ", err)
	}
	hf2, err := newHomedirFilesAndClear(shortenedCtx, utility, r, util.SecondUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for user2: ", err)
	}

	// ROUND 1: Test if anything's affected when both users are mounted and disk is full.

	// While both users are mounted, fill the vault once and check everything works.
	if err = fillVaultAndClear(shortenedCtx, utility, r, util.FirstUsername); err != nil {
		s.Fatal("Failed to fill user1's vault: ", err)
	}
	if err = verifyAndStep(shortenedCtx, hf1); err != nil {
		s.Fatal("Failed to verify/step test files in the user1's home directory in first round: ", err)
	}
	if err = verifyAndStep(shortenedCtx, hf2); err != nil {
		s.Fatal("Failed to verify/step test files in the user2's home directory in first round: ", err)
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
	if err = verifyAndStep(shortenedCtx, hf2); err != nil {
		s.Fatal("Failed to verify/step test files in the user2's home directory in second round: ", err)
	}
	if err := utility.MountVault(shortenedCtx, util.Password1Label, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword1), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user1 in second round: ", err)
	}
	if err = verifyAndStep(shortenedCtx, hf1); err != nil {
		s.Fatal("Failed to verify/step test files in the user1's home directory in second round: ", err)
	}
}
