// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	arcAppName = "YT Music"
	arcPkgName = "com.google.android.apps.youtube.music"

	cwsAppID   = "mkaakpdehdafacodkgkpghoibnmamcme"
	cwsAppName = "Google Drawings"
	cwsAppURL  = "https://chrome.google.com/webstore/detail/google-drawings/mkaakpdehdafacodkgkpghoibnmamcme"

	arcInstallationTimeout = 3 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchInstalledApps,
		Desc:         "Install apps from CWS and ARC++ Play Store and verify that it appears in the launcher",
		Contacts:     []string{"kyle.chen@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "arcBootedWithPlayStore",
		Timeout:      3*time.Minute + arcInstallationTimeout + cws.InstallationTimeout,
	})
}

// SearchInstalledApps tests that apps installed from CWS and ARC++ Play Store appear in launcher.
func SearchInstalledApps(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kw.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	}(cleanupCtx)

	if err := installARCApp(ctx, tconn, a, d); err != nil {
		s.Fatal("Failed to install App from Play Store: ", err)
	}

	if err := installCWSApp(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to install App from Chrome Web Store: ", err)
	}

	for _, appName := range []string{
		arcAppName,
		cwsAppName,
	} {
		appAppeared := uiauto.New(tconn).WaitUntilExists(launcher.AppSearchFinder(appName))

		if err := uiauto.Combine("open launcher and verify app appears in search result",
			launcher.Open(tconn),
			launcher.Search(tconn, kw, appName),
			appAppeared,
		)(ctx); err != nil {
			s.Fatal("Failed to complete test: ", err)
		}

		if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelShiftSearch); err != nil {
			s.Fatal("Failed to switch to close launcher: ", err)
		}
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			s.Fatal("Failed to wait for launcher is closed: ", err)
		}
	}
}

func installARCApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device) error {
	installCtx, cancel := context.WithTimeout(ctx, arcInstallationTimeout)
	defer cancel()

	testing.ContextLogf(ctx, "Try to install ARC app: %q", arcAppName)
	if err := playstore.InstallApp(installCtx, a, d, arcPkgName, -1); err != nil {
		return errors.Wrapf(err, "failed to install %q", arcAppName)
	}

	return apps.Close(ctx, tconn, apps.PlayStore.ID)
}

func installCWSApp(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, tconn, cwsAppID)
	if err != nil {
		return errors.Wrap(err, "failed to check CWS app's existance")
	}

	if isInstalled {
		return nil
	}

	app := cws.App{
		Name:         cwsAppName,
		URL:          cwsAppURL,
		InstalledTxt: "Launch app",
		AddTxt:       "Add to Chrome",
		ConfirmTxt:   "Add app",
	}

	testing.ContextLogf(ctx, "Try to install CWS app: %q", cwsAppName)
	return cws.InstallApp(ctx, cr, tconn, app)
}
