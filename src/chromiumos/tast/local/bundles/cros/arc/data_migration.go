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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// How to create archived home data to be used by this test:
// 1) Flash the previous version of ARC++ (e.g. ARC++ P).
// 2) Sign in with the specified test account (See arc.DataMigration.yaml for username/password).
// 3) Wait until ARC++ boots and uninstall all unnecessary apps.
// 4) (optional) Populate files under /data/ or install apps.
// 5) ssh to DUT and create .tbz2 file by
//    `cd /home/.shadow/<hash>/mount && tar --xattrs --selinux -cjf /tmp/<dest_file_name>.tbz2 .`
// 6) Upload the tbz2 file into gs://chromiumos-test-assets-public/tast/cros/arc/ and update
//    the .external file (See tast/local/bundles/cros/arc/data/data_migration_pi_x86_64.external).
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
		VarDeps:      []string{"arc.DataMigration.username", "arc.DataMigration.password"},
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

// DataMigration checks regressions for the following bugs:
// b/173835269 Can't download or install some apps after P->R data migration.
// b/183690158 vold hangs while processing fixupAllAppDirs() if there are thousands of files to fix.
//             (Home data data_migration_pi_* contains 5000 dirs under
//              /sdcard/Android/data/com.android.vending/files/ for reproducing this bug.)
// b/190293594 GMSCore for Pi is picked up on ARC R after P->R upgrade.
func DataMigration(ctx context.Context, s *testing.State) {
	const (
		// One of the apps reported by b/173835269.
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
	defer a.Close(ctx)

	systemSdkVersion, err := checkSdkVersionsInPackagesXML(ctx, username)
	if err != nil {
		s.Fatal("Failed to check SDK version in packages.xml: ", err)
	}

	// Regression check for b/190293594.
	if err := checkGmsCoreVersion(ctx, a, systemSdkVersion); err != nil {
		// Log error and continue testing.
		s.Error("Failed to check GMSCore version: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	// Regression check for b/173835269.
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

// checkSdkVersionsInPackagesXML checks if system SDK version is higher than data SDK version and
// returns system SDK version.
func checkSdkVersionsInPackagesXML(ctx context.Context, username string) (int, error) {
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
		return 0, errors.Wrap(err, "failed to get the cryptohome directory for the user")
	}

	// /home/root/<hash>/android-data/data/system/packages.xml
	b, err := ioutil.ReadFile(filepath.Join(rootCryptDir, "android-data", packagesXMLPath))
	if err != nil {
		return 0, errors.Wrap(err, "failed to open packages.xml")
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
		return 0, errors.Wrapf(err, "failed to get system SDK version or data SDK version in packages.xml (%s)",
			foundVersionsText)
	}
	if systemVersion <= dataVersion {
		return 0, errors.Wrapf(err, "system SDK version should be higher than data SDK version (%s)",
			foundVersionsText)
	}
	return systemVersion, nil
}

// checkGmsCoreVersion checks if ARC is using an expected version of GMSCore.
func checkGmsCoreVersion(ctx context.Context, a *arc.ARC, systemSdkVersion int) error {
	// Regexp for matching a GMSCore version string in logcat.
	// e.g. "com.google.android.gms@212013032@21.20.13 (100800-374639054)"
	gmscoreVersionRegexp := regexp.MustCompile(
		`com\.google\.android\.gms@\d+@\d+\.\d+\.\d+ \((\d{6})-\d+\)`)

	out, err := a.Command(ctx, "logcat", "-d").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to run logcat")
	}

	var fullVersionString string
	var versionCode int
	for _, line := range strings.Split(string(out), "\n") {
		m := gmscoreVersionRegexp.FindStringSubmatch(line)
		if len(m) == 2 {
			fullVersionString = m[0]
			versionCode, _ = strconv.Atoi(m[1])
			break
		}
	}
	if len(fullVersionString) == 0 {
		return errors.New("failed to find GMSCore version string in logcat")
	}
	testing.ContextLogf(ctx, "ARC is using GMSCore of version %q", fullVersionString)

	// Checks "variant" in GMSCore version string. Reference: go/gmscore-decoder-ring
	var expectedVariant int
	switch systemSdkVersion {
	case 28:
		// Skip checking variant for ARC P.
		return nil
	case 30:
		expectedVariant = 15 // PROD_RVC
	case 31:
		expectedVariant = 19 // PROD_SC
	default:
		return errors.Errorf("unexpected system SDK version: %d", systemSdkVersion)
	}
	variant := versionCode / 10000
	if variant != expectedVariant {
		return errors.Errorf("ARC is using GMSCore of unexpected variant: got %d; want %d. Found GMSCore version: %q",
			variant, expectedVariant, fullVersionString)
	}

	return nil
}
