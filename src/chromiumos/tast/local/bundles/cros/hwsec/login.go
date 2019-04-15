// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Login,
		Desc: "Verifies the cryptohome is mounted only after login",
		Contacts: []string{
			"achuith@chromium.org",  // Original autotest author
			"hidehiko@chromium.org", // Tast port author
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Login(ctx context.Context, s *testing.State) {
	const (
		testUser = "cryptohome_test@chromium.org"
		testPass = "testme"
	)

	// Set up a vault for testUser, which is not the login user, and
	// create a file in it. The file should be hidden from the login user.
	if err := cryptohome.CreateVault(ctx, testUser, testPass); err != nil {
		s.Fatal("Failed to create a vault for the test user: ", err)
	}
	defer cryptohome.RemoveVault(ctx, testUser)

	var testFile string
	func() {
		defer cryptohome.UnmountVault(ctx, testUser)
		userPath, err := cryptohome.UserPath(testUser)
		if err != nil {
			s.Fatal("Failed to get user path: ", err)
		}

		testFile = filepath.Join(userPath, "hello")
		if err = ioutil.WriteFile(testFile, nil, 0666); err != nil {
			s.Fatal("Failed to create a test file: ", err)
		}
	}()

	var user string
	func() {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to log in by Chrome: ", err)
		}
		defer cr.Close(ctx)

		user = cr.User()
		if mounted, err := cryptohome.IsMounted(ctx, user); err != nil {
			s.Errorf("Failed to check mounted vault for %q: %v", user, err)
		} else if !mounted {
			s.Errorf("No mounted vault for %q", user)
		}

		_, err = os.Stat(testFile)
		if !os.IsNotExist(err) {
			s.Errorf("File should not exist at %s: %v", testFile, err)
		}
	}()

	// Emulate logout. chrome.Chrome.Close() does not log out. So, here,
	// manually restart "ui" job for the emulation.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Conceptually, this should be declared at the timing of the vault
	// creation. However, anyway removing the vault wouldn't work while
	// the user logs in. So, this is the timing to declare.
	defer cryptohome.RemoveVault(ctx, user)

	if mounted, err := cryptohome.IsMounted(ctx, user); err != nil {
		s.Errorf("Failed to check mounted vault for %q: %v", user, err)
	} else if mounted {
		s.Errorf("Mounted vault for %q is still found after logout", user)
	}
}
