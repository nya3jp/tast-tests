// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LoginCleanupHack,
		Desc: "Test that logging in removes large Chrome logs",
		Contacts: []string{
			"vsavu@google.com",     // Test author
			"gwendal@chromium.com", // Lead for Chrome OS Storage
			"chromeos-commercial-remote-management@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// LoginCleanupHack tests that cryptohomed removes large logs on login.
// TODO(crbug.com/1287022): Remove in M101.
// The feature will be removed from cryptohome.
func LoginCleanupHack(ctx context.Context, s *testing.State) {
	const (
		chromeLogPath = "/home/chronos/user/log/chrome"

		user     = "logs-user-1"
		password = "1234"
		fillSize = 256 * 1024 * 1024
	)

	if err := cryptohome.CreateVault(ctx, user, password); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user)
	// Unmount all users before removal.
	defer cryptohome.UnmountAll(ctx)

	// Create log dir.
	if err := os.MkdirAll(filepath.Dir(chromeLogPath), 0755); err != nil {
		s.Fatal("Failed to create Chrome log directory: ", err)
	}

	file, err := os.OpenFile(chromeLogPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		s.Fatal("Failed to open Chrome log: ", err)
	}

	if err := syscall.Fallocate(int(file.Fd()), 0, 0, int64(fillSize)); err != nil {
		s.Fatalf("Failed to allocate %v bytes in %s: %v", fillSize, file.Name(), err)
	}
	file.Close()

	if err := cryptohome.UnmountVault(ctx, user); err != nil {
		s.Fatal("Failed to unmount user vault: ", err)
	}

	// Remount the user.
	if err := cryptohome.CreateVault(ctx, user, password); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}

	if err := cryptohome.WaitForUserMount(ctx, user); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	// Check if the file is present.
	if _, err := os.Stat(chromeLogPath); err == nil {
		s.Error("Chrome log still exists")
	} else if !os.IsNotExist(err) {
		s.Fatal("Failed to check if log file exists: ", err)
	}
}
