// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/cryptohome/disk"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AutomaticCleanupManyUsers,
		Desc: "Test automatic disk cleanup",
		Contacts: []string{
			"vsavu@google.com",     // Test author
			"gwendal@chromium.com", // Lead for Chrome OS Storage
			"chromeos-commercial-stability@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Timeout: 3 * time.Minute,
	})
}

func AutomaticCleanupManyUsers(ctx context.Context, s *testing.State) {
	const (
		userCount         = 10
		homedirSize       = 10 * disk.MiB
		startingFreeSpace = disk.MinimalFreeSpace - userCount*homedirSize // Free space at the beginning of test.

		userPrefix = "cleanup-user"
		password   = "1234"
	)

	// Start cryptohomed and wait for it to be available
	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohomed not running as expected: ", err)
	}
	defer upstart.RestartJob(cleanupCtx, "cryptohomed")

	if err := disk.CleanupExistingUsers(ctx); err != nil {
		s.Fatal("Failed to perform initial cleanup: ", err)
	}

	// Stay above trigger for cleanup
	fillFile, err := disk.FillUntil(disk.UserHome, startingFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill space: ", err)
	}
	defer os.Remove(fillFile)

	if freeSpace, err := disk.FreeSpace(disk.UserHome); err != nil {
		s.Fatal("Failed get free space: ", err)
	} else {
		s.Logf("%v bytes available after fill", freeSpace)
	}

	var fillFiles []string
	// Create user directories.
	for i := 1; i <= userCount; i++ {
		user := fmt.Sprintf("%s-%d", userPrefix, i)

		fillFile, err := disk.CreateFilledUserHomedir(ctx, user, password, "Cache", homedirSize)
		if err != nil {
			s.Fatal("Failed to create user with content: ", err)
		}
		defer cryptohome.RemoveVault(cleanupCtx, user)

		fillFiles = append(fillFiles, fillFile)
	}

	ctx, st := timing.Start(ctx, "cleanup")
	if err := disk.RunAutomaticCleanup(ctx); err != nil {
		s.Fatal("Failed to run automatic cleanup: ", err)
	}
	st.End()

	for _, fillFile := range fillFiles {
		if _, err := os.Stat(fillFile); err == nil {
			s.Error("fillFile still present")
		} else if !os.IsNotExist(err) {
			s.Fatalf("Failed to check if fill file %s exists: %v", fillFile, err)
		}
	}
}
