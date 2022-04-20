// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type appSource int

const (
	fromCWS appSource = iota
	fromPlayStore
)

type appInfo struct {
	name   string
	source appSource

	cwsApp   cws.App
	cwsAppID string

	arcAppPkgName string
}

const installationTimeout = cws.InstallationTimeout

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
					name:   "Wicked Good Unarchiver",
					source: fromCWS,
					cwsApp: cws.App{
						Name: "Wicked Good Unarchiver",
						URL:  "https://chrome.google.com/webstore/detail/wicked-good-unarchiver/mljpablpddhocfbnokacjggdbmafjnon",
					},
					cwsAppID: "mljpablpddhocfbnokacjggdbmafjnon",
				},
				Fixture: "chromeLoggedWithGaia",
			}, {
				Name: "arc",
				Val: &appInfo{
					name:          "Calendar",
					source:        fromPlayStore,
					arcAppPkgName: "com.google.android.calendar",
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

	cleanup, err := installApp(ctx, cr, tconn, a, app)
	if err != nil {
		s.Fatalf("Failed to install app %s: %v", app.name, err)
	}
	// Ensure cleanup the app if uninstallAppInLauncher failed or test case fail before uninstalling it.
	defer cleanup(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "uninstall_app_from_launcher")

	if err := launcher.Open(tconn)(ctx); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	ui := uiauto.New(tconn)
	appItem := nodewith.Name(app.name).HasClass(launcher.ExpandedItemsClass)

	if err := uninstallAppInLauncher(ctx, tconn, ui, appItem); err != nil {
		s.Fatal("Failed to delete app in launcher: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	// Reopen launcher, otherwise app list item will always exist.
	if err := reopenLauncher(ctx, tconn, kb, ui); err != nil {
		s.Fatal("Failed to reopen launcher: ", err)
	}

	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Failed to wait the launcher state stable with full screen search: ", err)
	}

	if err := ui.EnsureGoneFor(appItem, 5*time.Second)(ctx); err != nil {
		s.Fatalf("The app %s is still remain in launcher: %v", app.name, err)
	}
}

// installApp installs the app.
// returns a cleanup function to uninstall the app, and an error indicates if the app has been successfully installed.
func installApp(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, a *arc.ARC, app *appInfo) (func(context.Context), error) {
	cleanup := func(ctx context.Context) {}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	switch app.source {
	case fromCWS:
		// The cws has already sets the installation timeout, use the ctx directly.
		if err := cws.InstallApp(ctx, cr, tconn, app.cwsApp); err != nil {
			return cleanup, errors.Wrapf(err, "failed to install %s", app.name)
		}
		return func(ctx context.Context) { ossettings.UninstallApp(ctx, tconn, cr, app.name, app.cwsAppID) }, nil
	case fromPlayStore:
		if a == nil {
			return cleanup, errors.New("arc session not provided")
		}
		d, err := a.NewUIDevice(ctx)
		if err != nil {
			return cleanup, errors.Wrap(err, "failed to create new ARC UI device")
		}
		defer d.Close(cleanupCtx)

		if err := playstore.InstallApp(ctx, a, d, app.arcAppPkgName, &playstore.Options{TryLimit: -1, InstallationTimeout: installationTimeout}); err != nil {
			return cleanup, errors.Wrapf(err, "failed to install %s", app.name)
		}
		return func(ctx context.Context) { a.Command(ctx, "pm", "uninstall", app.arcAppPkgName).Run() }, nil
	}

	return cleanup, errors.Errorf("unexpected app source %v", app.source)
}

// uninstallAppInLauncher uninstalls an app from launcher.
func uninstallAppInLauncher(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context, appItem *nodewith.Finder) error {
	uninstallBtn := nodewith.Name("Uninstall").HasClass("MenuItemView")
	confirmBtn := nodewith.Name("Uninstall").HasClass("MdTextButton")

	return uiauto.Combine("uninstall app",
		ui.RightClick(appItem),
		ui.LeftClick(uninstallBtn),
		// Clicking the confirm uninstall button too frequently will not be effectively, add 1 sec interval between clicks to avoid.
		ui.WithInterval(time.Second).LeftClickUntil(confirmBtn, ui.Gone(confirmBtn)),
	)(ctx)
}

// reopenLauncher closes launcher and open it.
func reopenLauncher(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, ui *uiauto.Context) error {
	if err := closeLauncher(ctx, tconn, kb, ui); err != nil {
		return errors.Wrap(err, "failed to close launcher")
	}
	return launcher.Open(tconn)(ctx)
}

// closeLauncher ensures the launcher is closed.
func closeLauncher(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, ui *uiauto.Context) error {
	return ui.RetryUntil(
		kb.AccelAction("esc"),
		func(ctx context.Context) error { return ash.WaitForLauncherState(ctx, tconn, ash.Closed) },
	)(ctx)
}
