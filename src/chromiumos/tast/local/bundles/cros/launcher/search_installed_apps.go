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
	"chromiumos/tast/local/arc/optin"
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

type appSource int

const (
	chromeWebStore appSource = iota
	googlePlayStore
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
		Func: SearchInstalledApps,
		Desc: "Install apps from CWS or ARC++ Play Store and verify that it appears in the launcher",
		Contacts: []string{
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Name:    "cws",
			Val:     chromeWebStore,
			Timeout: 3*time.Minute + cws.InstallationTimeout,
		}, {
			Name:              "arc",
			Val:               googlePlayStore,
			ExtraSoftwareDeps: []string{"arc"},
			Timeout:           3*time.Minute + arcInstallationTimeout,
		}},
	})
}

// SearchInstalledApps tests that apps installed from CWS and ARC++ Play Store appear in launcher.
func SearchInstalledApps(ctx context.Context, s *testing.State) {
	source := s.Param().(appSource)

	var extraOpts []chrome.Option
	if source == googlePlayStore {
		extraOpts = []chrome.Option{
			chrome.ExtraArgs(arc.DisableSyncFlags()...),
			chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			chrome.ARCSupported(),
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, extraOpts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kw.Close()

	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	}(cleanupCtx)

	var appName string
	switch source {
	case chromeWebStore:
		if err := installCWSApp(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to install App from Chrome Web Store: ", err)
		}
		appName = cwsAppName
	case googlePlayStore:
		if err := optin.Perform(ctx, cr, tconn); err != nil {
			s.Fatal("Failed to optin to Play Store: ", err)
		}
		if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
			s.Fatal("Failed to close Play Store: ", err)
		}

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Could not start ARC: ", err)
		}
		defer a.Close(cleanupCtx)

		d, err := a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed to create new ARC UI device: ", err)
		}
		defer d.Close(cleanupCtx)

		if err := installARCApp(ctx, tconn, a, d); err != nil {
			s.Fatal("Failed to install App from Play Store: ", err)
		}
		appName = arcAppName
	}

	if err := uiauto.Combine("open launcher and verify app appears in search result",
		launcher.Open(tconn),
		launcher.Search(tconn, kw, appName),
		uiauto.New(tconn).WaitUntilExists(launcher.AppSearchFinder(appName)),
	)(ctx); err != nil {
		s.Fatal("Failed to complete test: ", err)
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
