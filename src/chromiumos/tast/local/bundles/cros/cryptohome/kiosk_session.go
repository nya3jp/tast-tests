// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KioskSession,
		Desc: "Ensures that cryptohome correctly mounts kiosk sessions",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func KioskSession(ctx context.Context, s *testing.State) {
	const (
		testFile        = "file"
		testFileContent = "content"
		cleanupTime     = 20 * time.Second
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	// Create and mount the kiosk for the first time.
	if err := cryptohome.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}
	defer cryptohome.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	userPath, err := cryptohome.UserPath(ctx, cryptohome.KioskUser)
	if err != nil {
		s.Fatal("Failed to get kiosk user vault path: ", err)
	}
	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	// Unmount and mount again.
	cryptohome.UnmountVault(ctx, cryptohome.KioskUser)
	if _, err := ioutil.ReadFile(filePath); err == nil {
		s.Fatal("File is readable after unmount")
	}
	if err := cryptohome.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}

	cryptohome.UnmountVault(ctx, cryptohome.KioskUser)

	// Test if existing kiosk vault can be signed in with AuthSession.
	if err := cryptohome.AuthSessionMountFlow(ctx, true /*Kiosk User*/, cryptohome.KioskUser, "" /* Empty passkey*/, false /*Create User*/); err != nil {
		s.Fatal("Failed to Mount with AuthSession: ", err)
	}
}
