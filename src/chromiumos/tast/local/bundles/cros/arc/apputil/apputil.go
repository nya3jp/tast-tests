// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apputil implements the libraries used to control ARC apps
package apputil

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// App holds resources of ARC app.
type App struct {
	Kb    *input.KeyboardEventWriter
	Tconn *chrome.TestConn
	A     *arc.ARC
	D     *ui.Device

	AppName string
	PkgName string

	launched bool
}

// NewApp creates and returns ArcApp.
func NewApp(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC, appName, pkgName string) (*App, error) {
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new ARC UI device")
	}

	return &App{
		Kb:       kb,
		Tconn:    tconn,
		A:        a,
		D:        d,
		AppName:  appName,
		PkgName:  pkgName,
		launched: false,
	}, nil
}

// Install installs ARC app.
func (app *App) Install(ctx context.Context) error {
	installed, err := app.A.PackageInstalled(ctx, app.PkgName)
	if err != nil {
		return errors.Wrapf(err, "failed to get the installed state of %q", app.PkgName)
	}
	if installed {
		return nil
	}

	// Limit the installation time with a new context.
	installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	if err := playstore.InstallOrUpdateApp(installCtx, app.A, app.D, app.PkgName, -1); err != nil {
		return errors.Wrapf(err, "failed to install %s", app.PkgName)
	}
	if err := optin.ClosePlayStore(ctx, app.Tconn); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}
	return nil
}

// Launch launches ARC app.
func (app *App) Launch(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := launcher.SearchAndLaunch(app.Tconn, app.Kb, app.AppName)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s app", app.AppName)
		}
		return ash.WaitForVisible(ctx, app.Tconn, app.PkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrapf(err, "failed to wait for the new window of %s", app.PkgName)
	}

	app.launched = true
	return nil
}

// Close cleans up the ARC app resources,
// it dumps the ARC UI if hasError returns true, and then closes ARC app.
// If hasError returns true, screenshot will be taken and UI hierarchy will be dumped to the given dumpDir.
func (app *App) Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error {
	if err := app.D.Close(ctx); err != nil {
		// Just log the error.
		testing.ContextLog(ctx, "Failed to close ARC UI device: ", err)
	}

	faillog.SaveScreenshotOnError(ctx, cr, outDir, hasError)
	if err := app.A.DumpUIHierarchyOnError(ctx, outDir, hasError); err != nil {
		// Just log the error.
		testing.ContextLog(ctx, "Failed to dump ARC UI hierarchy: ", err)
	}

	if !app.launched {
		return nil
	}
	w, err := ash.GetARCAppWindowInfo(ctx, app.Tconn, app.PkgName)
	if err != nil {
		return errors.Wrap(err, "failed to get ARC UI window info")
	}
	return w.CloseWindow(ctx, app.Tconn)
}
