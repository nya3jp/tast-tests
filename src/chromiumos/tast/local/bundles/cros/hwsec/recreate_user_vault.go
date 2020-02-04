// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

const (
	testUser     = "this_is_a_local_test_account@chromium.org"
	testPassword = "this_is_a_test_password"
	testLabel    = "example"
	testFileName = "TESTFILE"
)

var testFileContent = []byte("TEST_CONTENT")

func init() {
	testing.AddTest(&testing.Test{
		Func: RecreateUserVault,
		Desc: "Verifies that cryptohome recreates user's vault directory when the TPM is re-owned",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"garryxiao@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
	})
}

// RecreateUserVault is ported from the autotest test platform_CryptohomeTPMReOwn and renamed to
// reflects what's being tested. It avoids reboots in the original test by using the soft-clearing
// TPM utils and restarting TPM-related daemons.
func RecreateUserVault(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}

	helper, err := hwseclocal.NewHelper(utility)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// Resets the TPM, system, and user states before running the tests.
	if err := hwseclocal.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}
	if _, err := utility.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 1: mounts vault for the test user")

	if err := utility.MountVault(ctx, testUser, testPassword, testLabel, true); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, testUser); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}
	if err := writeUserTestContent(ctx, testUser, testFileName, testFileContent); err != nil {
		s.Fatal("Failed to write user test content: ", err)
	}
	if _, err := utility.Unmount(ctx, testUser); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 2: restarts TPM daemons and mounts user vault")

	// Restarts all TPM daemons to simulate a reboot in the original autotest test.
	if err := hwseclocal.RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM-related daemons: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := utility.MountVault(ctx, testUser, testPassword, testLabel, false); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, testUser); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}

	// User vault should already exist and shouldn't be recreated.
	if content, err := readUserTestContent(ctx, testUser, testFileName); err != nil {
		s.Fatal("Failed to read user test content: ", err)
	} else if !bytes.Equal(content, testFileContent) {
		s.Fatalf("Unexpected test file content: got %q, want %q", content, testFileContent)
	}

	if _, err := utility.Unmount(ctx, testUser); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 3: clears TPM and mounts user vault again")

	if err := hwseclocal.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}
	if err := utility.MountVault(ctx, testUser, testPassword, testLabel, true); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, testUser); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}

	// User vault should be recreated after TPM is cleared.
	if exists, err := doesUserTestFileExist(ctx, testUser, testFileName); err != nil {
		s.Fatal("Failed to check user test file: ", err)
	} else if exists {
		s.Fatal("Cryptohome didn't recreate user vault; original test file still exists")
	}

	// Cleanup.
	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		s.Error("Failed to cleanup after the test: ", err)
	}
}

// writeUserTestContent writes the given content to the given file into the given user's home dir.
// The file is created if it doesn't exist.
func writeUserTestContent(ctx context.Context, user string, fileName string, content []byte) error {
	testFile, err := getUserTestFilePath(ctx, user, fileName)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(testFile, content, 0644)
}

// readUserTestContent reads content from the given file under the given user's home dir.
// Returns the file contents if the read succeeded or an error if there's anything wrong.
func readUserTestContent(ctx context.Context, user string, fileName string) ([]byte, error) {
	testFile, err := getUserTestFilePath(ctx, user, fileName)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadFile(testFile)
}

// doesUserTestFileExist checks and returns if the given test file exists in the given user's home dir.
func doesUserTestFileExist(ctx context.Context, user string, fileName string) (bool, error) {
	testFile, err := getUserTestFilePath(ctx, user, fileName)
	if err != nil {
		return false, err
	}

	fileInfo, err := os.Stat(testFile)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if fileInfo.IsDir() {
		return false, errors.Errorf("%s is a dir", testFile)
	}

	return true, nil
}

// getUserTestFilePath returns the full path of the given file under the given user's home dir.
func getUserTestFilePath(ctx context.Context, user string, fileName string) (string, error) {
	userPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return "", err
	}

	return filepath.Join(userPath, fileName), nil
}
