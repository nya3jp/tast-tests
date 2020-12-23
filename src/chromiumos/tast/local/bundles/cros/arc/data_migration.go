// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	homeDataNamePiX86 = "data_migration_pi_x86_64"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataMigration,
		Desc:         "Boots ARC with /data created on the previous version of ARC and verifies Play Store can install an app",
		Contacts:     []string{"niwa@google.com", "arc-storage@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars:         []string{"arc.DataMigration.username", "arc.DataMigration.password"},
		Params: []testing.Param{{
			// Launch ARC R with /data created on ARC P.
			Name: "p_to_r",
			// TODO(b/155123165): Add ARM support.
			ExtraData:         []string{homeDataNamePiX86},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func DataMigration(ctx context.Context, s *testing.State) {
	const (
		// One of the apps reported by
		// b/173835269 (Can't download or install some apps after P->R data migration).
		appToInstall = "com.roblox.client"
	)

	homeDataPath := s.DataPath(homeDataNamePiX86)
	username := s.RequiredVar("arc.DataMigration.username")
	password := s.RequiredVar("arc.DataMigration.password")

	// Ensure to sign out before executing mountVaultWithArchivedHomeData().
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to sign out: ", err)
	}

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// Unarchive the home data under vault before signing in.
	if err := mountVaultWithArchivedHomeData(ctx, homeDataPath, username, password); err != nil {
		s.Fatal("Failed to mount home with archived data: ", err)
	}
	defer cryptohome.RemoveVault(cleanupCtx, username)

	args := append(arc.DisableSyncFlags(), "--disable-arc-data-wipe")
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(),
		chrome.KeepState(), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err := checkSdkVersionInArcLog(ctx); err != nil {
		s.Fatal("Failed to check SDK version in arc.log: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	s.Log("Installing app " + appToInstall)
	if err := playstore.InstallApp(ctx, a, d, appToInstall, -1); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
}

func mountVaultWithArchivedHomeData(ctx context.Context, homeDataPath, username, password string) error {
	// Unmount and mount vault for the user.
	if err := cryptohome.UnmountVault(ctx, username); err != nil {
		return err
	}
	if err := cryptohome.RemoveVault(ctx, username); err != nil {
		return err
	}
	if err := cryptohome.CreateVault(ctx, username, password); err != nil {
		return err
	}

	vaultPath, err := cryptohome.MountedVaultPath(ctx, username)
	if err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Unarchiving home data %q under %q", homeDataPath, vaultPath)
	if err := testexec.CommandContext(
		ctx, "tar", "--xattrs", "--selinux", "-C", vaultPath, "-xjf", homeDataPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to unarchive home data under vault")
	}
	return nil
}

func checkSdkVersionInArcLog(ctx context.Context) error {
	const (
		arcLogPath = "/var/log/arc.log"
	)

	// SDK version of ARC running currently.
	systemVersionRegexp := regexp.MustCompile(`System SDK version: (\d+)`)
	systemVersion := 0

	// SDK version of ARC /data.
	dataVersionRegexp := regexp.MustCompile(`Data SDK version: (\d+)`)
	dataVersion := 0

	testing.ContextLogf(ctx, "Obtaining SDK version from %s", arcLogPath)
	out, err := testexec.CommandContext(ctx, "tail", arcLogPath, "-n", "500").Output()
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", arcLogPath)
	}
	for _, l := range strings.Split(string(out), "\n") {
		// If there are multiple matching lines, take the last one.
		m := systemVersionRegexp.FindStringSubmatch(l)
		if m != nil {
			systemVersion, _ = strconv.Atoi(m[1])
			continue
		}
		m = dataVersionRegexp.FindStringSubmatch(l)
		if m != nil {
			dataVersion, _ = strconv.Atoi(m[1])
			continue
		}
	}

	testing.ContextLogf(ctx, "System SDK version: %d, Data SDK verson: %d", systemVersion, dataVersion)
	if systemVersion <= 0 || dataVersion <= 0 {
		return errors.Wrapf(err, "failed to get System SDK version or Data SDK version in %s", arcLogPath)
	}
	if systemVersion <= dataVersion {
		return errors.Wrap(err, "System SDK version should be higher than Data SDK version")
	}
	return nil
}
