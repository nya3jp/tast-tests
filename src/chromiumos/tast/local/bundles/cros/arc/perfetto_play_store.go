// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoPlayStore,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "A test to detect jank when the Play Store installs, search for, and launches Google Calculator",
		Contacts:     []string{"yukashu@chromium.org", "sstan@chromium.org", "brpol@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		Params: []testing.Param{{
			Val: playStoreSearchAndLaunchTestParams{
				MaxOptinAttempts: 2,
			},
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name: "vm",
			Val: playStoreSearchAndLaunchTestParams{
				MaxOptinAttempts: 2,
			},
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Data:    []string{"config.pbtxt"},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func PerfettoPlayStore(ctx context.Context, s *testing.State) {

	const (
		pkgName = "com.google.android.calculator"
		appName = "Calculator"
	)
	// Give cleanup actions a minute to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// Optin to Play Store.
	s.Log("Opting into Play Store")
	maxAttempts := s.Param().(playStoreSearchAndLaunchTestParams).MaxOptinAttempts

	if err := optin.PerformWithRetry(ctx, cr, maxAttempts); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)
	defer func() {
		if s.HasError() {
			if err := a.Command(cleanupCtx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			}
			if err := a.PullFile(cleanupCtx, "/sdcard/window_dump.xml", filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	// Install app.
	s.Log("Installing app")
	if err := playstore.InstallApp(ctx, a, d, pkgName, &playstore.Options{TryLimit: -1}); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	ui := uiauto.New(tconn)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	//Search for and launch the app.
	if err := uiauto.Combine("search for calculator in launcher",
		launcher.SearchAndLaunchWithQuery(tconn, kb, appName, appName),
		ui.WaitUntilExists(nodewith.Name(appName).ClassName("Widget")),
	)(ctx); err != nil {
		s.Fatal("Failed to search and launch: ", err)
	}

	if err := a.ForceEnableTrace(ctx); err != nil {
		s.Fatal("Error on enabling perfetto trace")
	}

	if err := a.RunPerfettoTrace(ctx, s.DataPath("config.pbtxt"), filepath.Join(s.OutDir(), "pulledtrace")); err != nil {
		s.Fatal("Error on run perfetto trace")
	}

}
