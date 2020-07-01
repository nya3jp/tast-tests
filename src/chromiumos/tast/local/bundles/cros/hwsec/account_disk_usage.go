// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/session/ownership"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AccountDiskUsage,
		Desc: "Verifies that GetAccountDiskUsage API works as expected",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		Attr:    []string{"group:mainline", "informational"},
		Data:    []string{"testcert.p12"},
		Timeout: 3 * time.Minute,
	})
}

// setUpVaultAndUserAsOwner will setup a user and its vault, and setup the policy to make the user the owner of the device.
// Caller of this assumes the responsibility of umounting/cleaning up the vault regardless of whether the function returned an error.
func setUpVaultAndUserAsOwner(ctx context.Context, certpath, username, password, label string, utility *hwsec.UtilityCryptohomeBinary) error {
	// We need the policy/ownership related stuff because we want to set the owner, so that we can create ephemeral mount.
	privKey, err := session.ExtractPrivKey(certpath)
	if err != nil {
		return errors.Wrap(err, "failed to parse PKCS #12 file")
	}

	if err := session.SetUpDevice(ctx); err != nil {
		return errors.Wrap(err, "failed to reset device ownership")
	}

	// Setup the owner policy.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create session_manager binding")
	}
	if err := session.PrepareChromeForPolicyTesting(ctx, sm); err != nil {
		return errors.Wrap(err, "failed to prepare Chrome for testing")
	}

	// Pre-configure some owner settings, including initial key.
	settings := ownership.BuildTestSettings(username)
	if err := session.StoreSettings(ctx, sm, username, privKey, nil, settings); err != nil {
		return errors.Wrap(err, "failed to store settings")
	}

	// Start a new session, which will trigger the re-taking of ownership.
	wp, err := sm.WatchPropertyChangeComplete(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start watching PropertyChangeComplete signal")
	}
	defer wp.Close(ctx)
	ws, err := sm.WatchSetOwnerKeyComplete(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start watching SetOwnerKeyComplete signal")
	}
	defer ws.Close(ctx)

	// Now create the vault.
	if err := utility.MountVault(ctx, username, password, label, true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to create user vault for testing")
	}
	// Note: Caller of this method is responsible for cleaning up the

	if err = sm.StartSession(ctx, username, ""); err != nil {
		return errors.Wrapf(err, "failed to start new session for %s", username)
	}

	select {
	case <-wp.Signals:
	case <-ws.Signals:
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "timed out waiting for PropertyChangeComplete or SetOwnerKeyComplete signal")
	}

	return nil
}

// setUpEphemeralVaultAndUser will setup the vault and user of the specified username, password and label, as an ephemeral user/mount.
// Caller of this assumes the responsibility of umounting/cleaning up the vault regardless of whether the function returned an error.
func setUpEphemeralVaultAndUser(ctx context.Context, username, password, label string, utility *hwsec.UtilityCryptohomeBinary) error {
	config := hwsec.NewVaultConfig()
	config.Ephemeral = true
	if err := utility.MountVault(ctx, username, password, label, true, config); err != nil {
		return errors.Wrap(err, "failed to create ephemeral user vault for testing")
	}

	return nil
}

// setUpEcryptfsVaultAndUser will setup the vault and user of the specified username, password and label, with the vault backed by ecryptfs.
// Caller of this assumes the responsibility of umounting/cleaning up the vault regardless of whether the function returned an error.
func setUpEcryptfsVaultAndUser(ctx context.Context, username, password, label string, utility *hwsec.UtilityCryptohomeBinary) error {
	config := hwsec.NewVaultConfig()
	config.Ecryptfs = true
	if err := utility.MountVault(ctx, username, password, label, true, config); err != nil {
		return errors.Wrap(err, "failed to create user vault for testing")
	}

	return nil
}

// createSparseFile creates a sparse file of the given size in bytes at location path.
func createSparseFile(path string, size int64) error {
	// Create the file. Note that if the file exist, it'll be truncated to 0 bytes.
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "failed to create or reduce filesize of file %q to 0", path)
	}

	if err := f.Truncate(size); err != nil {
		return errors.Wrapf(err, "failed to set filesize of file %q to %d", path, size)
	}

	if err := f.Close(); err != nil {
		return errors.Wrapf(err, "failed to close file %q", path)
	}
	return nil
}

// testAccountUsage will test a given account and see if GetAccountDiskUsage() works correctly with it or not.
// If testSparseFile is true, then it'll test the sparse file test case in addition to what's normally tested, otherwise, spare file case is not tested.
// Note that the account's home directory should be empty or nearly empty before calling this.
// This method doesn't return anything, it'll just output the error or abort the test.
func testAccountUsage(ctx context.Context, s *testing.State, cmdRunner hwsec.CmdRunner, username string, utility *hwsec.UtilityCryptohomeBinary, testSparseFile bool) {
	const (
		// The size of the test file in MiB.
		testFileSize = 256
		// The size of the sparse file in MiB.
		testSparseFileSize = 4096
		// The margin we'll have for testing in MiB.
		// The test will pass if the result is +/- this margin.
		testFileMargin = 32
	)

	usageBefore, err := utility.GetAccountDiskUsage(ctx, username)
	if err != nil {
		s.Fatal("Failed to get the account disk usage before writing data: ", err)
	}

	testFilePath, err := util.GetUserTestFilePath(ctx, username, util.TestFileName1)
	if err != nil {
		s.Fatal("Failed to get user test file path: ", err)
	}

	testSparseFilePath, err := util.GetUserTestFilePath(ctx, username, util.TestFileName2)
	if err != nil {
		s.Fatal("Failed to get user test sparse file path: ", err)
	}

	// Write a test file.
	// Note that we want the file to be random so that transparent filesystem
	// compression (if any is implemented) doesn't affect this test.
	// OpenSSL is used instead of /dev/urandom because it's much faster.
	if _, err := cmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("openssl enc -aes-128-ctr -pass pass:CrOS4tw -nosalt </dev/zero | dd of=%s bs=1M count=%d iflag=fullblock", shutil.Escape(testFilePath), testFileSize)); err != nil {
		s.Fatal("Failed to write the test file: ", err)
	}

	if testSparseFile {
		// Write a sparse test file. *1024*1024 because 1MiB is 1024*1024 bytes.
		if err := createSparseFile(testSparseFilePath, testSparseFileSize*1024*1024); err != nil {
			s.Fatal("Failed to create sparse test file: ", err)
		}
	}

	// Synchronize cached writes to persistent storage before we query again.
	if _, err := cmdRunner.Run(ctx, "sync"); err != nil {
		s.Fatal("Failed to synchronize cached writes to persistent storage: ", err)
	}

	usageAfter, err := utility.GetAccountDiskUsage(ctx, username)
	if err != nil {
		s.Fatal("Failed to get the account disk usage after writing data: ", err)
	}

	// *1024*1024 because 1MiB is 1024*1024 bytes.
	expectedAfter := usageBefore + testFileSize*1024*1024
	expectedAfterUpperLimit := expectedAfter + testFileMargin*1024*1024
	expectedAfterLowerLimit := expectedAfter - testFileMargin*1024*1024
	if expectedAfterLowerLimit > usageAfter || expectedAfterUpperLimit < usageAfter {
		// Convert the unit to MiB so that it's easier to read.
		usageAfterFloat := float64(usageAfter) / (1024 * 1024)
		expectedAfterLowerLimitFloat := float64(expectedAfterLowerLimit) / (1024 * 1024)
		expectedAfterUpperLimitFloat := float64(expectedAfterUpperLimit) / (1024 * 1024)
		s.Errorf("Disk usage for user %q after writing data is out of range, got %g MiB, want %g MiB ~ %g Mib", username, usageAfterFloat, expectedAfterLowerLimitFloat, expectedAfterUpperLimitFloat)
	}
}

func AccountDiskUsage(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}

	// Cleanup before we start.
	if err := utility.UnmountAll(ctx); err != nil {
		s.Log("Fails to unmount all before test starts: ", err)
	}
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		s.Log("Fails to remove vault before test starts: ", err)
	}
	if _, err := utility.RemoveVault(ctx, util.SecondUsername); err != nil {
		s.Log("Fails to remove vault before test starts: ", err)
	}
	if _, err := utility.RemoveVault(ctx, util.ThirdUsername); err != nil {
		s.Log("Fails to remove vault before test starts: ", err)
	}

	// Set up the first user as the owner and test the dircrypto mount.
	err = setUpVaultAndUserAsOwner(ctx, s.DataPath("testcert.p12"), util.FirstUsername, util.FirstPassword, util.PasswordLabel, utility)
	defer func() {
		// Remember to logout and delete vault.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to logout during cleanup: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to cleanup first user after the test: ", err)
		}
	}()
	if err != nil {
		s.Fatal("Failed to setup vault and user as owner: ", err)
	}

	testAccountUsage(ctx, s, cmdRunner, util.FirstUsername, utility, true /* test sparse file case */)

	// Set up the second user as ephemeral mount and test the ephemeral mount.
	// Note: This need to be second because ephemeral mount is only possible after owner is established.
	err = setUpEphemeralVaultAndUser(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, utility)
	defer func() {
		// Remember to logout and delete vault.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to logout during cleanup: ", err)
		}
		// Yeah, it's ephemeral but we are going to clean it anyway just to be safe.
		if _, err := utility.RemoveVault(ctx, util.SecondUsername); err != nil {
			s.Error("Failed to cleanup second user after the test: ", err)
		}
	}()
	if err != nil {
		s.Fatal("Failed to setup vault and user for second user: ", err)
	}

	testAccountUsage(ctx, s, cmdRunner, util.SecondUsername, utility, true /* test sparse file case */)

	// Set up the third user as a user with ecryptfs backed vault and test it.
	err = setUpEcryptfsVaultAndUser(ctx, util.ThirdUsername, util.ThirdPassword, util.PasswordLabel, utility)
	defer func() {
		// Remember to log out and delete vault.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to logout during cleanup: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.ThirdUsername); err != nil {
			s.Error("Failed to cleanup third user after the test: ", err)
		}
	}()
	if err != nil {
		s.Fatal("Failed to setup vault and user for third user: ", err)
	}

	// Note that sparse file in ecryptfs doesn't result in sparse file in underlying disk.
	testAccountUsage(ctx, s, cmdRunner, util.ThirdUsername, utility, false /* do not test sparefile with ecryptfs */)
}
