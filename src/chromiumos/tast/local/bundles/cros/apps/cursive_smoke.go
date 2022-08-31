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
var cursiveEnabledModels = []string{
	"nautilus",
	"phaser360",
	"krane",
	"bobba360",
	"kohaku",
	"kevin",
	"robo360",
	"vayne",
	"scarlet",
	"eve",
	"nocturne",
	"betty",
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
		HardwareDeps: hwdep.D(hwdep.Model(cursiveEnabledModels...)),
		Vars:         []string{"cursiveServerURL"},
		Params: []testing.Param{{
			Fixture: fixture.LoggedIn,
		}, {
			Name:              "lacros",
			Fixture:           fixture.LacrosLoggedIn,
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func CursiveSmoke(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(fixture.FixtData).Chrome
	tconn := s.FixtValue().(fixture.FixtData).TestAPIConn
	browserType := s.FixtValue().(fixture.FixtData).BrowserType

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree")

	ui := uiauto.New(tconn).WithInterval(2 * time.Second)
	installIcon := nodewith.HasClass("PwaInstallView").Role(role.Button)
	installButton := nodewith.Name("Install").Role(role.Button)

	appURL := cursive.AppURL
	if serverURL, ok := s.Var("cursiveServerURL"); ok {
		appURL = serverURL
	}

	conn, br, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, browserType, appURL)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}

	shouldClose := true
	defer func(ctx context.Context) {
		if shouldClose {
			closeBrowser(cleanupCtx)
			conn.Close()
		}
	}(cleanupCtx)

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
		s.Fatal("Failed to wait for Cursive to be installable: ", err)
	}

	if err := uiauto.Combine("",
		ui.LeftClick(installIcon),
		ui.LeftClick(installButton))(ctx); err != nil {
		s.Fatal("Failed to click install button: ", err)
	}

	cursiveAppID, err := ash.WaitForChromeAppByNameInstalled(ctx, tconn, apps.Cursive.Name, 2*time.Minute)
	if err != nil {
		s.Fatal("Failed to wait for installed app: ", err)
	}

	// The web page is the PWA to be installed. Keeping the page opened
	// will make it in A11y which can make the ui finder a bit harder.
	// Besides, have the web page & PWA app launched at the same time,
	// can potentially cause conflicts? (I guess it is possible in theory),
	// and nice to test the PWA launch in a clean environment align with real user experience.
	if browserType == browser.TypeLacros {
		if err := closeBrowser(ctx); err != nil {
			s.Fatal("Failed to close Lacros: ", err)
		}
		// b/244390727 Lacros needs to be opened to make PWA launchable.
		_, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeLacros)
		if err != nil {
			s.Fatal("Failed to set up Lacros: ", err)
		}
		defer closeBrowser(cleanupCtx)
	}
	shouldClose = false

	// Close all ash apps and pages to make a clean desk.
	targetsToClose := func(t *chrome.Target) bool {
		return t.Type == "page" || t.Type == "app"
	}
	if err := cr.CloseTargets(ctx, targetsToClose); err != nil {
		s.Fatal("Failed to close targets: ", err)
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
