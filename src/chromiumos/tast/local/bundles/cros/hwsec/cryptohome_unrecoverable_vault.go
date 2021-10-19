// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/storage/files"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	// keysetTestFile1 is a test file used to verify that the vault indeed removed.
	keysetTestFile1 = "/home/.shadow/%s/test1234.bin"
	// keysetTestFile2 is the same as above.
	keysetTestFile2 = "/home/user/%s/test9876.bin"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeUnrecoverableVault,
		Desc: "Verifies that when a vault is unrecoverable, it'll be removed when we try to mount it",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// checkIsMounted verifies if we're mounted or not.
func checkIsMounted(ctx context.Context, utility *hwsec.CryptohomeClient, expected bool) error {
	actuallyMounted, err := utility.IsMounted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to call IsMounted()")
	}
	if actuallyMounted != expected {
		return errors.Errorf("incorrect IsMounted() state %t, expected %t", actuallyMounted, expected)
	}
	return nil
}

// corruptUserKeyset corrupts the keyset for the given user so that the mount will fail.
func corruptUserKeyset(ctx context.Context, r hwsec.CmdRunner, sanitizedUsername string) error {
	// "nocheck" because the key file is not controlled by this test, so we can't get rid of the blocked keyword.
	keysetPath := fmt.Sprintf("/home/.shadow/%s/master.0", sanitizedUsername) // nocheck
	if _, err := r.Run(ctx, "dd", "if=/dev/zero", "of="+keysetPath, "bs=1", "count=200", "seek=100", "conv=notrunc"); err != nil {
		return errors.Wrapf(err, "failed to corrupt the keyset file %q", keysetPath)
	}

	return nil
}

// CryptohomeUnrecoverableVault verifies that when a vault is unrecoverable, it'll be removed when we try to mount it.
func CryptohomeUnrecoverableVault(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	utility := hwsec.NewCryptohomeClient(cmdRunner)

	// Clear any remnant data on the DUT.
	utility.UnmountAndRemoveVault(ctx, util.FirstUsername)

	// Retrieve the sanitized username for later use.
	cred := chrome.Creds{User: util.FirstUsername, Pass: util.FirstPassword}
	sanitizedUsername, err := utility.GetSanitizedUsername(ctx, cred.User, true /* useDBus */)
	if err != nil {
		s.Fatal("Failed to get sanitized username for the test user: ", err)
	}

	// Login with the test user.
	cr, err := chrome.New(ctx, chrome.FakeLogin(cred))
	if err != nil {
		s.Fatal("Failed to login with Chrome for user creation: ", err)
	}

	// Ensure we're mounted.
	if err = checkIsMounted(ctx, utility, true); err != nil {
		s.Fatal("Invalid IsMounted state during user creation: ", err)
	}

	// Plant some test files:
	fi1, err := files.NewFileInfo(ctx, fmt.Sprintf(keysetTestFile1, sanitizedUsername), cmdRunner)
	if err != nil {
		s.Fatal("Failed to create test file 1")
	}
	if err = fi1.Clear(ctx); err != nil {
		s.Fatal("Failed to clear test file 1")
	}
	if err = fi1.Step(ctx); err != nil {
		s.Fatal("Failed to step test file 1")
	}
	fi2, err := files.NewFileInfo(ctx, fmt.Sprintf(keysetTestFile2, sanitizedUsername), cmdRunner)
	if err != nil {
		s.Fatal("Failed to create test file 2")
	}
	if err = fi2.Clear(ctx); err != nil {
		s.Fatal("Failed to clear test file 2")
	}
	if err = fi2.Step(ctx); err != nil {
		s.Fatal("Failed to step test file 2")
	}

	// Logout.
	if err = cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome after user creation: ", err)
	}
	// Need to restart ui to logout. See b/159063029.
	if err = upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui to logout: ", err)
	}

	// Ensure we're not mounted.
	if err = checkIsMounted(ctx, utility, false); err != nil {
		s.Fatal("Invalid IsMounted state after user creation: ", err)
	}

	// Disrupt the keyset.
	if err = corruptUserKeyset(ctx, cmdRunner, sanitizedUsername); err != nil {
		s.Fatal("Failed to corrupt user keyset: ", err)
	}

	// Try to mount again.
	cr, err = chrome.New(ctx, chrome.FakeLogin(cred))
	if err != nil {
		s.Fatal("Failed to login with Chrome after keyset corruption: ", err)
	}

	// Ensure we're mounted.
	if err = checkIsMounted(ctx, utility, true); err != nil {
		s.Fatal("Invalid IsMounted state after keyset corruption: ", err)
	}

	// The test file should be gone.
	if err = fi1.Verify(ctx); err == nil {
		s.Error("Vault is not removed after corrupting keyset, test file 1 remaining")
	}
	if err = fi2.Verify(ctx); err == nil {
		s.Error("Vault is not removed after corrupting keyset, test file 2 remaining")
	}
}
