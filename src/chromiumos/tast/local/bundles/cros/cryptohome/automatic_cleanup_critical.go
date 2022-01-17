// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"fmt"
	"os"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/cryptohome/cleanup"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/disk"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AutomaticCleanupCritical,
		Desc: "Test critical automatic disk cleanup",
		Contacts: []string{
			"vsavu@google.com",     // Test author
			"gwendal@chromium.com", // Lead for Chrome OS Storage
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: fixture.Enrolled,
	})
}

func AutomaticCleanupCritical(ctx context.Context, s *testing.State) {
	const (
		homedirSize = 100 * cleanup.MiB // 100 Mib, used for testing

		temporaryUser = "tmp-user"
		user1         = "critical-cleanup-user1"
		user2         = "critical-cleanup-user2"
		password      = "1234"
	)

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Start cryptohomed and wait for it to be available.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}
	defer daemonController.Restart(ctx, hwsec.CryptohomeDaemon)

	freeSpace, err := disk.FreeSpace(cleanup.UserHome)
	if err != nil {
		s.Fatal("Failed to get the amount of free space")
	}

	cleanupThreshold := freeSpace + 50*1024*1024
	cleanupThresholdsArgs := fmt.Sprintf("--cleanup_threshold=%d --aggressive_cleanup_threshold=%d --critical_cleanup_threshold=%d --target_free_space=%d", cleanupThreshold, cleanupThreshold, cleanupThreshold, cleanupThreshold)

	// Restart with higher thresholds.
	if err := upstart.RestartJob(ctx, "cryptohomed", upstart.WithArg("VMODULE_ARG", "*=1"), upstart.WithArg("CRYPTOHOMED_ARGS", cleanupThresholdsArgs)); err != nil {
		s.Fatal("Failed to restart cryptohome: ", err)
	}

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := cleanup.RunOnExistingUsers(ctx); err != nil {
		s.Fatal("Failed to perform initial cleanup: ", err)
	}

	// Create users with contents to fill up disk space.
	_, err = cleanup.CreateFilledUserHomedir(ctx, user1, password, "Downloads", homedirSize)
	if err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user1)

	fillFile2, err := cleanup.CreateFilledUserHomedir(ctx, user2, password, "Downloads", homedirSize)
	if err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user2)
	// Unmount all users before removal.
	defer cryptohome.UnmountAll(ctx)

	// Make sure to unmount the second user.
	if err := cryptohome.UnmountVault(ctx, user2); err != nil {
		s.Fatal("Failed to unmount user vault: ", err)
	}

	// Remount the second user. Since space is very low, other user should be cleaned up.
	if err := cryptohome.CreateVault(ctx, user2, password); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	if err := cryptohome.WaitForUserMount(ctx, user2); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	if exists, err := cleanup.UserHomeExists(ctx, user1); err != nil {
		s.Fatal("Failed to dermine if user vault exists: ", err)
	} else if exists {
		s.Error("User vault not cleaned up during login")
	}

	if _, err := os.Stat(fillFile2); err != nil {
		s.Error("Data for user2 lost: ", err)
	}
}
