// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apputil implements the libraries used to control ARC apps
package apputil

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// App holds resources of ARC app.
type App struct {
	KB     *input.KeyboardEventWriter
	Tconn  *chrome.TestConn
	ARC    *arc.ARC
	Device *ui.Device

	AppName string
	PkgName string

	launched bool
}

// NewApp creates and returns an instance of App which represents and ARC App.
func NewApp(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC, appName, pkgName string) (*App, error) {
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new ARC UI device")
	}

	return &App{
		KB:       kb,
		Tconn:    tconn,
		ARC:      a,
		Device:   d,
		AppName:  appName,
		PkgName:  pkgName,
		launched: false,
	}, nil
}

// InstallationTimeout defines the maximum time duration to install an app from the play store.
const InstallationTimeout = 10 * time.Minute

// Install installs the ARC app with the package name.
func (app *App) Install(ctx context.Context) error {
	deadLine, ok := ctx.Deadline()
	if ok && deadLine.Sub(time.Now()) < InstallationTimeout {
		return errors.Errorf("there are no time to install ARC app %q", app.AppName)
	}

	if err := playstore.InstallOrUpdateAppAndClose(ctx, app.Tconn, app.ARC, app.Device, app.PkgName, &playstore.Options{TryLimit: -1, InstallationTimeout: InstallationTimeout}); err != nil {
		return errors.Wrapf(err, "failed to install %s", app.PkgName)
	}
	return nil
}

// Launch launches the ARC app and returns the time spent for the app to be visible.
// The app has to be installed before calling this function,
// i.e. `Install(context.Context)` should be called first.
func (app *App) Launch(ctx context.Context) (time.Duration, error) {
	if w, err := ash.GetARCAppWindowInfo(ctx, app.Tconn, app.PkgName); err == nil {
		// If the package is already visible,
		// needs to close it and launch again to collect app start time.
		if err := w.CloseWindow(ctx, app.Tconn); err != nil {
			return -1, errors.Wrapf(err, "failed to close %s app", app.AppName)
		}
	}

	var startTime time.Time
	// Sometimes the Spotify App will fail to open, so add retry here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := launcher.SearchAndLaunch(app.Tconn, app.KB, app.AppName)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s app", app.AppName)
		}
		startTime = time.Now()
		return ash.WaitForVisible(ctx, app.Tconn, app.PkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return -1, errors.Wrapf(err, "failed to wait for the new window of %s", app.PkgName)
	}

	app.launched = true
	return time.Since(startTime), nil
}

// GetVersion gets the version of the ARC app.
func (app *App) GetVersion(ctx context.Context) (version string, err error) {
	out, err := app.ARC.Command(ctx, "dumpsys", "package", app.PkgName).Output()
	if err != nil {
		return "", err
	}
	versionNamePrefix := "versionName="
	output := string(out)
	splitOutput := strings.Split(output, "\n")
	for splitLine := range splitOutput {
		if strings.Contains(splitOutput[splitLine], versionNamePrefix) {
			version := strings.Split(splitOutput[splitLine], "=")[1]
			testing.ContextLogf(ctx, "Version of app %q is: %s", app.AppName, version)
			return version, nil
		}
	}
	return "", errors.New("failed to find versionName")
}

// Close cleans up the ARC app resources,
// it dumps the ARC UI if hasError returns true, and then closes ARC app.
// If hasError returns true, screenshot will be taken and UI hierarchy will be dumped to the given dumpDir.
func (app *App) Close(ctx context.Context, cr *chrome.Chrome, hasError func() bool, outDir string) error {
	if err := app.Device.Close(ctx); err != nil {
		// Just log the error.
		testing.ContextLog(ctx, "Failed to close ARC UI device: ", err)
	}

	faillog.SaveScreenshotOnError(ctx, cr, outDir, hasError)
	if err := app.ARC.DumpUIHierarchyOnError(ctx, outDir, hasError); err != nil {
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

// DismissMobilePrompt dismisses the prompt of "This app is designed for mobile".
func DismissMobilePrompt(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	prompt := nodewith.Name("This app is designed for mobile").Role(role.Window)
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(prompt)(ctx); err == nil {
		testing.ContextLog(ctx, "Dismiss the app prompt")
		gotIt := nodewith.Name("Got it").Role(role.Button).Ancestor(prompt)
		if err := ui.LeftClickUntil(gotIt, ui.WithTimeout(time.Second).WaitUntilGone(gotIt))(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Got it' button")
		}
	}
	return nil
}
