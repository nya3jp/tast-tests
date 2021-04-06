// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PlayStoreUprev,
		Desc:     "A test which installs latest play store and verifies basic functionality",
		Contacts: []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:     []string{"group:mainline", "informational", "group:arc-functional"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("juniper", "kohaku")),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("juniper", "kohaku")),
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"ui.gaiaPoolDefault"},
	})
}

func PlayStoreUprev(ctx context.Context, s *testing.State) {
	const (
		pkgName       = "com.google.android.apps.photos"
		baseapkName   = "phonesky_classic_base_signed.apk"
		armapkName    = "phonesky_classic_base-armeabi_v7a_signed.apk"
		gsBaseApkPath = "gs://chromeos-test-assets-private/tast/arc/playstore-builds/latest/" + baseapkName
		gsArmApkPath  = "gs://chromeos-test-assets-private/tast/arc/playstore-builds/latest/" + armapkName
	)

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin to PlayStore and Close.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	// Get the PlayStore Version before UpRev.
	var versionBeforeUpRev, versionAfterUpRev string
	if versionBeforeUpRev, err = getAppVersion(ctx, a); err != nil {
		s.Fatal("Failed to get Version: ", err)
	}

	baseapkPath := filepath.Join(s.OutDir(), baseapkName)
	armapkPath := filepath.Join(s.OutDir(), armapkName)

	apks := []string{
		baseapkPath, armapkPath,
	}

	// Open the GS Bucket and copy the Base APK to DUT.
	r, err := s.CloudStorage().Open(ctx, gsBaseApkPath)
	if err != nil {
		s.Fatal("Failed to download base apk: ", err)
	}

	fd, err := os.Create(baseapkPath)
	if err != nil {
		s.Fatal("Failed to create baseapk file: ", err)
	}
	w := bufio.NewWriter(fd)
	baseapkcopied, err := io.Copy(w, r)
	if err != nil {
		s.Fatal("Failed to copy base apk file: ", err)
	}
	s.Logf("%d byte(s) Copied", baseapkcopied)
	defer fd.Close()
	defer r.Close()
	defer w.Flush()

	// Open the GS Bucket and copy the ARM APK to DUT.
	r, err = s.CloudStorage().Open(ctx, gsArmApkPath)
	if err != nil {
		s.Fatal("Failed to download arm apk: ", err)
	}
	fd, err = os.Create(armapkPath)
	if err != nil {
		s.Fatal("Failed to create arm apk file: ", err)
	}
	w = bufio.NewWriter(fd)
	armapkcopied, err := io.Copy(w, r)
	if err != nil {
		s.Fatal("Failed to copy arm apk file: ", err)
	}
	s.Logf("%d byte(s) Copied", armapkcopied)

	// Install the apks copied.
	if err := a.InstallMultiple(ctx, apks); err != nil {
		s.Fatal("Failed installing playstore app: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Get the PlayStore Version Post UpRev.
	if versionAfterUpRev, err = getAppVersion(ctx, a); err != nil {
		s.Fatal("Failed to get Version: ", err)
	}

	if versionBeforeUpRev == versionAfterUpRev {
		s.Fatal("PlayStore Version is Same")
	}

	if err := launcher.LaunchApp(tconn, apps.PlayGames.Name)(ctx); err != nil {
		s.Fatal("Failed to Launch Play Games App")
	}

	// Install an ARC app with new Play Store.
	s.Log("Installing arc app with latest play store")
	if err := playstore.InstallApp(ctx, a, d, pkgName, -1); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// Disable the Play Store to reset the Play Store Uprev.
	if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable Play Store app : ", err)
	}

}
func getAppVersion(ctx context.Context, a *arc.ARC) (version string, err error) {

	out, err := a.Command(ctx, "dumpsys", "package", "com.android.vending").Output()
	if err != nil {
		return "", errors.Wrap(err, "could not get dumpsys package")
	}
	versionNamePrefix := "versionName="
	output := string(out)
	splitOutput := strings.Split(output, "\n")
	for splitLine := range splitOutput {
		if strings.Contains(splitOutput[splitLine], versionNamePrefix) {
			versionNameAfterSplit := strings.Split(splitOutput[splitLine], "=")[1]
			testing.ContextLogf(ctx, "Version name of Play Store is : %s ", versionNameAfterSplit)
			return versionNameAfterSplit, err
		}
	}
	return "", errors.Wrap(err, "couldn't  get app version")
}
