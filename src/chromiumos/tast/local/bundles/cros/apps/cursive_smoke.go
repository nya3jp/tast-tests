// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apps contains functionality and test cases for ChromeOS essential Apps.
package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/cursive"
	"chromiumos/tast/local/bundles/cros/apps/fixture"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// The models that support Cursive.
var cursiveManualInstallModels = []string{
	"nautilus",
	"phaser360",
	"bobba360",
	"kohaku",
	"robo360",
	"vayne",
	"scarlet",
	"betty",
}

var cursiveAutoInstallModels = []string{
	"eve",
	"kevin",
	"krane",
	"hatch",
	"nocturne",
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CursiveSmoke,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Cursive smoke test app launching and basic function",
		Contacts: []string{
			"shengjun@chromium.org",
			"gabpalado@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Vars:         []string{"cursiveServerURL"},
		Params: []testing.Param{
			{
				Name:              "manual_install",
				Fixture:           fixture.LoggedInDisableInstall,
				ExtraHardwareDeps: hwdep.D(hwdep.Model(cursiveManualInstallModels...)),
				Val:               false,
			},
			{
				Name:              "auto_install",
				Fixture:           fixture.LoggedIn,
				ExtraHardwareDeps: hwdep.D(hwdep.Model(cursiveAutoInstallModels...)),
				Val:               true,
			},
			{
				Name:              "manual_install_lacros",
				Fixture:           fixture.LacrosLoggedInDisableInstall,
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(cursiveManualInstallModels...)),
				Val:               false,
			},
			// TODO(b/245224264): Re-enable auto install testing on lacros.
			// {
			// 	Name:              "auto_install_lacros",
			// 	Fixture:           fixture.LacrosLoggedIn,
			// 	ExtraSoftwareDeps: []string{"lacros"},
			// 	ExtraHardwareDeps: hwdep.D(hwdep.Model(cursiveAutoInstallModels...)),
			// 	Val:               true,
			// },
		},
	})
}

func CursiveSmoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	browserType := s.FixtValue().(fixture.FixtData).BrowserType

	isAutoInstall := s.Param().(bool)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	appURL := cursive.AppURL
	if serverURL, ok := s.Var("cursiveServerURL"); ok {
		appURL = serverURL
	}

	var cursiveAppID string
	var err error
	if isAutoInstall {
		testing.ContextLog(ctx, "Waiting for Cursive to be auto installed")
		cursiveAppID, err = ash.WaitForChromeAppByNameInstalled(ctx, tconn, apps.Cursive.Name, 3*time.Minute)
		if err != nil {
			s.Fatal("Failed to wait for Cursive auto installed: ", err)
		}
	} else {
		cursiveAppID, err = manualInstallCursive(ctx, tconn, cr, browserType, appURL)
		if err != nil {
			s.Fatal("Failed to manually install Cursive: ", err)
		}
	}

	// Validate Cursive can be launched from shelf.
	if err := apps.Launch(ctx, tconn, cursiveAppID); err != nil {
		s.Fatal("Failed to launch Cursive: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, cursiveAppID, time.Minute); err != nil {
		s.Fatalf("Fail to wait for %s by app id %s: %v", apps.Cursive.Name, cursiveAppID, err)
	}

	if err := cursive.WaitForAppRendered(tconn)(ctx); err != nil {
		s.Fatal("Failed to render Cursive: ", err)
	}
}

func manualInstallCursive(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, browserType browser.Type, appURL string) (string, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	conn, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, browserType, appURL)
	if err != nil {
		return "", errors.Wrap(err, "failed to set up browser")
	}

	defer func(ctx context.Context) {
		closeBrowser(ctx)
		conn.Close()
	}(cleanupCtx)

	ui := uiauto.New(tconn).WithInterval(2 * time.Second)
	installIcon := nodewith.HasClass("PwaInstallView").Role(role.Button)
	installButton := nodewith.Name("Install").Role(role.Button)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Wait for longer time after second launch, since it can be delayed on low-end devices.
		if err := ui.WithTimeout(30 * time.Second).WaitUntilExists(installIcon)(ctx); err != nil {
			testing.ContextLog(ctx, "Install button is not shown initially. See b/230413572")
			testing.ContextLog(ctx, "Refresh page to enable installation")
			if reloadErr := br.ReloadActiveTab(ctx); reloadErr != nil {
				return testing.PollBreak(errors.Wrap(reloadErr, "failed to reload page"))
			}
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
		return "", errors.Wrap(err, "failed to wait for Cursive to be installable")
	}

	if err := uiauto.Combine("",
		ui.LeftClick(installIcon),
		ui.LeftClick(installButton))(ctx); err != nil {
		return "", err
	}

	cursiveAppID, err := ash.WaitForChromeAppByNameInstalled(ctx, tconn, apps.Cursive.Name, 1*time.Minute)
	if err != nil {
		return "", errors.Wrap(err, "failed to wait for installed app")
	}

	// Close all ash apps and pages to make a clean desk.
	targetsToClose := func(t *chrome.Target) bool {
		return t.Type == "page" || t.Type == "app"
	}
	if err := cr.CloseTargets(ctx, targetsToClose); err != nil {
		return "", errors.Wrap(err, "failed to close targets")
	}
	return cursiveAppID, nil
}
