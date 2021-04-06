// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// type fileServer struct {
// 	cloudStorage *testing.CloudStorage
// }

func init() {
	testing.AddTest(&testing.Test{
		Func:     PlayStoreUprev,
		Desc:     "A functional test of the Play Store that installs Google Calendar",
		Contacts: []string{"arc-core@google.com", "cros-arc-te@google.com"},
		Attr:     []string{"group:mainline", "group:arc-functional"},
		//Data:     []string{"phonesky_classic_base_signed.apk"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 10 * time.Minute,
		Vars:    []string{"ui.gaiaPoolDefault"},
	})
}

func PlayStoreUprev(ctx context.Context, s *testing.State) {
	const (
		pkgName = "com.google.android.apps.photos"
		apkName = "phonesky_classic_base_signed.apk"
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

	// Optin to PlayStore and Close
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	apkPath := filepath.Join(s.OutDir(), "phonesky_classic_base_signed.apk")
	//apkPath := filepath.Join(s.OutDir(), "phonesky_classic_base-armeabi_v7a_signed.apk")

	r, err := s.CloudStorage().Open(ctx, "gs://chromeos-test-assets-private/tast/arc/playstore-builds/latest/phonesky_classic_base_signed.apk")
	//r, err := s.CloudStorage().Open(ctx, "gs://chromeos-test-assets-private/tast/arc/playstore-builds/phonesky_classic_base_signed.apk")
	//r, err := s.CloudStorage().Open(ctx, "gs://chromeos-playstore-builds/latest/phonesky_classic_base_signed.apk")

	if err != nil {
		s.Fatal("Failed to download apk: ", err)
	}

	if fd, err := os.Create(apkPath); err != nil {
		s.Error("Failed to create file: ", err)
	} else {
		w := bufio.NewWriter(fd)
		copied, err := io.Copy(w, r)
		if err != nil {
			s.Error("Failed to copy file: ", err)
		}
		s.Logf("%d byte(s) Copied", copied)
	}

	s.Log("Installing ")
	if err := a.Install(ctx, apkPath); err != nil {
		s.Fatal("Failed installing latest playstore app: ", err)
	}

	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			}
			if err := a.PullFile(ctx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, -1); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable Play Store app : ", err)
	}

}
