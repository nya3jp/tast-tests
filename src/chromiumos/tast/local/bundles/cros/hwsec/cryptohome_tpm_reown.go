// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/ready"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	testUser     = "this_is_a_local_test_account@chromium.org"
  testPassword = "this_is_a_test_password"

  testFileName    = "TESTFILE"
)

var testFileContent = []byte("TEST_CONTENT")

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeTPMReown,
		Desc: "Verifies that cryptohome recreates user's vault directory when the TPM is re-owned",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"garryxiao@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:			3 * time.Minute,
	})
}

func CryptohomeTPMReown(ctx context.Context, s *testing.State) {
	if err := hwsec.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}

	// Waits for TPM to be owned.
	// TODO: replace this call with a hwsec util call when the util is ready before sending the CL for review.
	if err := ready.Wait(ctx, func(msg string) { s.Log(msg) }); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatalf("Failed to remove vault of user %s: %v", testUser, err)
	}

	s.Log("Phase 1: mounts vault for the test user")

	if err := mountUserVaultAndCheckKeyset(ctx, testUser, testPassword); err != nil {
		s.Fatal(err)
	}

	if err := writeUserTestContent(ctx, testUser, testFileContent); err != nil {
		s.Fatalf("Failed to write test content for user %s: %v", testUser, err)
	}

	if err := cryptohome.UnmountVault(ctx, testUser); err != nil {
		s.Fatalf("Failed to umount vault for user %s: %v", testUser, err)
	}

	s.Log("Phase 2: restarts TPM daemons and mounts user vault")

	if err := hwsec.RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM-related daemons: ", err)
	}

	if err := mountUserVaultAndCheckKeyset(ctx, testUser, testPassword); err != nil {
		s.Fatal(err)
	}

	// User vault should already exist and shouldn't be recreated.
	if content, err := readUserTestContent(ctx, testUser); err != nil {
		s.Fatalf("Failed to read test content for user %s: %v", testUser, err)
	} else if bytes.Compare(content, testFileContent) != 0 {
		s.Fatal("Unexpected test file content for user ", testUser)
	}

	if err := cryptohome.UnmountVault(ctx, testUser); err != nil {
		s.Fatalf("Failed to umount vault for user %s: %v", testUser, err)
	}

	s.Log("Phase 3: clears TPM and mounts user vault again")

	if err := hwsec.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}

	// Waits for TPM to be owned.
	// TODO: replace this call with a hwsec util call when the util is ready before sending the CL for review.
	if err := ready.Wait(ctx, func(msg string) { s.Log(msg) }); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	if err := mountUserVaultAndCheckKeyset(ctx, testUser, testPassword); err != nil {
		s.Fatal(err)
	}

	// User vault should be recreated after TPM is cleared.
	if exists, err := doesUserTestFileExist(ctx, testUser); err != nil {
		s.Fatal(err)
	} else if exists {
		s.Fatalf("Cryptohome didn't recreate vault for user %s; original test file still exists", testUser)
	}
}

func writeUserTestContent(ctx context.Context, user string, content []byte) error {
	testFile, err := getUserTestFilePath(ctx, user)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(testFile, content, 0644)
}

func readUserTestContent(ctx context.Context, user string) ([]byte, error) {
	testFile, err := getUserTestFilePath(ctx, user)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadFile(testFile)
}

func doesUserTestFileExist(ctx context.Context, user string) (bool, error) {
	testFile, err := getUserTestFilePath(ctx, user)
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

func getUserTestFilePath(ctx context.Context, user string) (string, error) {
	userPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", userPath, testFileName), nil
}

func mountUserVaultAndCheckKeyset(ctx context.Context, user string, password string) error {
	ctx, st := timing.Start(ctx, "mount")
	if err := cryptohome.CreateVault(ctx, user, password); err != nil {
		return errors.Wrapf(err, "failed to create vault for user %s", user)
	}
	st.End()

	if output, err := testexec.CommandContext(
		ctx, "cryptohome", "--action=dump_keyset", "--user="+user).Output(); err != nil {
		return errors.Errorf("failed to dump keyset for user ", user)
	} else if !strings.Contains(string(output), "TPM_WRAPPED") {
		return errors.Errorf("cryptohome didn't create a TPM-wrapped keyset for user %s", user)
	}

	return nil
}
