// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
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
	homeDataNameNycX86 = "data_migration_nyc_x86_64"
	homeDataNamePiX86  = "data_migration_pi_x86_64"
	homeDataNamePiArm  = "data_migration_pi_arm64"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DataMigration,
		Desc:     "Boots ARC with /data created on the previous version of ARC and verifies Play Store can install an app",
		Contacts: []string{"niwa@google.com", "arc-storage@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// "no_qemu" is added for excluding betty from the target board list.
		// TODO(b/179636279): Remove "no_qemu" after making the test pass on betty.
		SoftwareDeps: []string{"chrome", "no_qemu"},
		Timeout:      10 * time.Minute,
		Vars:         []string{"arc.DataMigration.username", "arc.DataMigration.password"},
		Params: []testing.Param{{
			// Launch ARC P with /data created on ARC N (for x86).
			Name:              "n_to_p_x86",
			Val:               homeDataNameNycX86,
			ExtraData:         []string{homeDataNameNycX86},
			ExtraSoftwareDeps: []string{"android_p", "amd64"},
		}, {
			// Launch ARC R with /data created on ARC P (for x86).
			Name:              "p_to_r_x86",
			Val:               homeDataNamePiX86,
			ExtraData:         []string{homeDataNamePiX86},
			ExtraSoftwareDeps: []string{"android_vm", "amd64"},
		}, {
			// Launch ARC R with /data created on ARC P (for arm).
			Name:              "p_to_r_arm",
			Val:               homeDataNamePiArm,
			ExtraData:         []string{homeDataNamePiArm},
			ExtraSoftwareDeps: []string{"android_vm", "arm"},
		}},
	})
}

func DataMigration(ctx context.Context, s *testing.State) {
	const (
		// One of the apps reported by
		// b/173835269 (Can't download or install some apps after P->R data migration).
		appToInstall = "com.roblox.client"
	)

	username := s.RequiredVar("arc.DataMigration.username")
	password := s.RequiredVar("arc.DataMigration.password")
	homeDataPath := s.DataPath(s.Param().(string))

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
	defer func() {
		cryptohome.UnmountVault(cleanupCtx, username)
		cryptohome.RemoveVault(cleanupCtx, username)
	}()

	args := append(arc.DisableSyncFlags(), "--disable-arc-data-wipe")
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{User: username, Pass: password}),
		chrome.ARCSupported(), chrome.KeepState(), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	if err := checkSdkVersionsInPackagesXML(ctx, username); err != nil {
		s.Fatal("Failed to check SDK version in arc.log: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	s.Log("Installing app " + appToInstall)
	if err := playstore.InstallApp(ctx, a, d, appToInstall, -1); err != nil {
		s.Error("Failed to install app: ", err)
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
	success := false
	defer func() {
		if !success {
			cryptohome.UnmountVault(ctx, username)
			cryptohome.RemoveVault(ctx, username)
		}
	}()

	vaultPath, err := cryptohome.MountedVaultPath(ctx, username)
	if err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Unarchiving home data %q under %q", homeDataPath, vaultPath)
	if err := testexec.CommandContext(
		ctx, "tar", "--xattrs", "--selinux", "-C", vaultPath, "-xjf", homeDataPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to unarchive home data under vault")
	}

	success = true
	return nil
}

func checkSdkVersionsInPackagesXML(ctx context.Context, username string) error {
	const (
		packagesXMLPath = "/data/system/packages.xml"
	)

	// SDK version of ARC running currently.
	systemVersionRegexp := regexp.MustCompile(`\<version sdkVersion="(\d+)"`)
	systemVersion := 0

	// SDK version of ARC /data.
	dataVersionRegexp := regexp.MustCompile(`\<version volumeUuid="\w+" sdkVersion="(\d+)"`)
	dataVersion := 0

	testing.ContextLogf(ctx, "Checking SDK versions in %s", packagesXMLPath)

	rootCryptDir, err := cryptohome.SystemPath(username)
	if err != nil {
		return errors.Wrap(err, "failed to get the cryptohome directory for the user")
	}

	// /home/root/<hash>/android-data/data/system/packages.xml
	b, err := ioutil.ReadFile(filepath.Join(rootCryptDir, "android-data", packagesXMLPath))
	if err != nil {
		return errors.Wrap(err, "failed to open packages.xml")
	}

	for _, l := range strings.Split(string(b), "\n") {
		m := systemVersionRegexp.FindStringSubmatch(l)
		if m != nil {
			systemVersion, _ = strconv.Atoi(m[1])
		}
		m = dataVersionRegexp.FindStringSubmatch(l)
		if m != nil {
			dataVersion, _ = strconv.Atoi(m[1])
		}
		if systemVersion > 0 && dataVersion > 0 {
			break
		}
	}

	foundVersionsText := fmt.Sprintf("Found system SDK version: %d, data SDK verson: %d",
		systemVersion, dataVersion)
	testing.ContextLog(ctx, foundVersionsText)
	if systemVersion <= 0 || dataVersion <= 0 {
		return errors.Wrapf(err, "failed to get system SDK version or data SDK version in packages.xml (%s)",
			foundVersionsText)
	}
	if systemVersion <= dataVersion {
		return errors.Wrapf(err, "system SDK version should be higher than data SDK version (%s)",
			foundVersionsText)
	}
	return nil
}
