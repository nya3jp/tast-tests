// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type playStoreTestParams struct {
	MaxOptinAttempts int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayStore,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A functional test of the Play Store that installs Google Calendar",
		Contacts:     []string{"arc-core@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		Params: []testing.Param{{
			Val: playStoreTestParams{
				MaxOptinAttempts: 2,
			},
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name: "vm",
			Val: playStoreTestParams{
				MaxOptinAttempts: 1,
			},
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func PlayStore(ctx context.Context, s *testing.State) {
	const (
		pkgName = "com.google.android.calculator"
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

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	maxAttempts := s.Param().(playStoreTestParams).MaxOptinAttempts

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)
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
	if err := playstore.InstallApp(ctx, a, d, pkgName, &playstore.Options{TryLimit: -1}); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
}
