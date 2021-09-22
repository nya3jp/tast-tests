// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SearchInstalledApps,
		Desc: "Install apps from CWS and verify that it appears in the launcher",
		Contacts: []string{
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		Timeout:      3*time.Minute + cws.InstallationTimeout,
	})
}

// SearchInstalledApps tests that apps installed from CWS appear in launcher.
func SearchInstalledApps(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	ui := uiauto.New(tconn)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kw.Close()

	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		if err := ui.RetryUntil(
			kw.AccelAction("esc"),
			func(ctx context.Context) error { return ash.WaitForLauncherState(ctx, tconn, ash.Closed) },
		)(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to dismiss launcher: ", err)
		}
	}(cleanupCtx)

	cwsapp := newCwsAppGoogleDrawings(cr, tconn)

	if err := cwsapp.install(ctx); err != nil {
		s.Fatal("Failed to install App from Chrome Web Store: ", err)
	}
	defer cwsapp.uninstall(cleanupCtx)

	if err := uiauto.Combine("open launcher and verify app appears in search result",
		launcher.Open(tconn),
		launcher.Search(tconn, kw, cwsapp.name),
		ui.WaitUntilExists(launcher.AppSearchFinder(cwsapp.name)),
	)(ctx); err != nil {
		s.Fatal("Failed to complete test: ", err)
	}
}

// cwsAppGoogleDrawings defines the cws-app `Google Drawings`.
type cwsAppGoogleDrawings struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn

	id     string
	name   string
	cwsURL string

	app *cws.App
}

// newCwsAppGoogleDrawings returns the instance of cwsAppGoogleDrawings.
func newCwsAppGoogleDrawings(cr *chrome.Chrome, tconn *chrome.TestConn) *cwsAppGoogleDrawings {
	c := &cwsAppGoogleDrawings{
		cr:    cr,
		tconn: tconn,
	}

	c.id = "mkaakpdehdafacodkgkpghoibnmamcme"
	c.name = "Google Drawings"
	c.cwsURL = "https://chrome.google.com/webstore/detail/google-drawings/mkaakpdehdafacodkgkpghoibnmamcme"

	c.app = &cws.App{
		Name:         c.name,
		URL:          c.cwsURL,
		InstalledTxt: "Launch app",
		AddTxt:       "Add to Chrome",
		ConfirmTxt:   "Add app",
	}

	return c
}

// install installs the cws-app via Chrome web store.
func (c *cwsAppGoogleDrawings) install(ctx context.Context) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, c.tconn, c.id)
	if err != nil {
		return errors.Wrap(err, "failed to check CWS app's existance")
	}

	if isInstalled {
		return nil
	}

	testing.ContextLogf(ctx, "Try to install CWS app: %q", c.name)
	return cws.InstallApp(ctx, c.cr, c.tconn, *c.app)
}

// uninstall uninstalls the cws-app via ossettings.
func (c *cwsAppGoogleDrawings) uninstall(ctx context.Context) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, c.tconn, c.id)
	if err != nil {
		return errors.Wrap(err, "failed to check CWS app's existance")
	}

	if !isInstalled {
		return nil
	}

	defer func() {
		settings := ossettings.New(c.tconn)
		settings.Close(ctx)
	}()
	testing.ContextLogf(ctx, "Try to uninstall CWS app: %q", c.name)
	return ossettings.UninstallApp(ctx, c.tconn, c.cr, c.name, c.id)
}
