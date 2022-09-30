// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type appSource int

const (
	fromCWS appSource = iota
	fromPlayStore
)

type appInfo struct {
	name         string
	id           string
	source       appSource
	isTabletMode bool

	cwsApp cws.App

	arcAppPkgName string
}

const installationTimeout = apputil.InstallationTimeout

func init() {
	testing.AddTest(&testing.Test{
		Func:         UninstallApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies app can be deleted from the list of app in launcher",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Timeout:      3*time.Minute + installationTimeout,
		Params: []testing.Param{
			{
				Name: "cws",
				Val: &appInfo{
					name:         "Wicked Good Unarchiver",
					id:           "mljpablpddhocfbnokacjggdbmafjnon",
					source:       fromCWS,
					isTabletMode: false,
					cwsApp: cws.App{
						Name: "Wicked Good Unarchiver",
						URL:  "https://chrome.google.com/webstore/detail/wicked-good-unarchiver/mljpablpddhocfbnokacjggdbmafjnon",
					},
				},
				Fixture: "chromeLoggedInWithGaia",
			},
			{
				Name: "cws_tablet",
				Val: &appInfo{
					name:         "Wicked Good Unarchiver",
					id:           "mljpablpddhocfbnokacjggdbmafjnon",
					source:       fromCWS,
					isTabletMode: true,
					cwsApp: cws.App{
						Name: "Wicked Good Unarchiver",
						URL:  "https://chrome.google.com/webstore/detail/wicked-good-unarchiver/mljpablpddhocfbnokacjggdbmafjnon",
					},
				},
				Fixture: "chromeLoggedInWithGaia",
			},
			{
				Name: "arc",
				Val: &appInfo{
					name:          "VLC",
					id:            "faabdppbcbmkemcigbhofgomflchgocl",
					source:        fromPlayStore,
					isTabletMode:  false,
					arcAppPkgName: "org.videolan.vlc",
				},
				Fixture:           "arcBootedWithPlayStore",
				ExtraSoftwareDeps: []string{"arc"},
			},
			{
				Name: "arc_tablet",
				Val: &appInfo{
					name:          "VLC",
					id:            "faabdppbcbmkemcigbhofgomflchgocl",
					source:        fromPlayStore,
					isTabletMode:  true,
					arcAppPkgName: "org.videolan.vlc",
				},
				Fixture:           "arcBootedWithPlayStore",
				ExtraSoftwareDeps: []string{"arc"},
			},
		},
	})
}

// UninstallApp verifies app can be deleted from the list of app in launcher.
func UninstallApp(ctx context.Context, s *testing.State) {
	app := s.Param().(*appInfo)

	var cr *chrome.Chrome
	var a *arc.ARC

	switch app.source {
	case fromCWS:
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
	case fromPlayStore:
		cr = s.FixtValue().(*arc.PreData).Chrome
		a = s.FixtValue().(*arc.PreData).ARC
	default:
		s.Fatal("Unexpected app source: ", app.source)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := installApp(ctx, cr, tconn, a, app); err != nil {
		s.Fatalf("Failed to install app %s: %v", app.name, err)
	}
	// Ensure cleanup the app if test case fail before uninstalling it.
	defer uninstall(cleanupCtx, cr, tconn, app)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "uninstall_app_from_launcher")

	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, app.isTabletMode, true /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "uninstall_app_from_launcher_after_test_setup")

	ui := uiauto.New(tconn)

	allAppsRoot := nodewith.Ancestor(nodewith.Role(role.Group).NameContaining("All Apps"))
	appItem := allAppsRoot.Name(app.name).HasClass(launcher.ExpandedItemsClass)

	if err := launcher.UninstallsAppUsingContextMenu(ctx, tconn, appItem); err != nil {
		s.Fatal("Failed to delete app in launcher: ", err)
	}

	if err := ui.EnsureGoneFor(nodewith.Name(app.name).HasClass(launcher.ExpandedItemsClass), 10*time.Second)(ctx); err != nil {
		s.Fatalf("The app %q still remains in launcher: %v", app.name, err)
	}
}

// installApp installs the app and returns an error indicating if the app has been successfully installed.
func installApp(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, arcSession *arc.ARC, app *appInfo) error {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	testing.ContextLog(ctx, "Installing app")

	switch app.source {
	case fromCWS:
		// The cws has already sets the installation timeout, use the ctx directly.
		if err := cws.InstallApp(ctx, cr, tconn, app.cwsApp); err != nil {
			return errors.Wrapf(err, "failed to install %s", app.name)
		}
	case fromPlayStore:
		if arcSession == nil {
			return errors.New("arc session not provided")
		}
		device, err := arcSession.NewUIDevice(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create new ARC UI device")
		}
		defer device.Close(cleanupCtx)

		if err := playstore.InstallOrUpdateAppAndClose(ctx, tconn, arcSession, device, app.arcAppPkgName, &playstore.Options{TryLimit: -1, InstallationTimeout: installationTimeout}); err != nil {
			return errors.Wrapf(err, "failed to install %s", app.name)
		}
	default:
		return errors.Errorf("unexpected app source %v", app.source)
	}

	return nil
}

// uninstall uninstalls the app via ossettings.
func uninstall(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, app *appInfo) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, tconn, app.id)
	if err != nil {
		return errors.Wrap(err, "failed to check app's existance")
	}

	if !isInstalled {
		return nil
	}

	defer func(ctx context.Context) {
		settings := ossettings.New(tconn)
		settings.Close(ctx)
	}(ctx)
	testing.ContextLogf(ctx, "Uninstall app: %q", app.name)
	return ossettings.UninstallApp(ctx, tconn, cr, app.name, app.id)
}
