// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
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
func (app *App) Launch(ctx context.Context) (time.Duration, error) {
	if w, err := ash.GetARCAppWindowInfo(ctx, app.Tconn, app.PkgName); err == nil {
		// If the package is already visible,
		// needs to close it and launch again to collect app start time.
		if err := w.CloseWindow(ctx, app.Tconn); err != nil {
			return -1, errors.Wrapf(err, "failed to close %s app", app.AppName)
		}
	}

	var startTime time.Time
	// Sometimes the app will fail to open, so add retry here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		startTime = time.Now()
		if err := launcher.SearchAndLaunch(app.Tconn, app.Kb, app.AppName)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s app", app.AppName)
		}
		return ash.WaitForVisible(ctx, app.Tconn, app.PkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return -1, errors.Wrapf(err, "failed to wait for the new window of %s", app.PkgName)
	}

	app.launched = true
	return time.Since(startTime), nil
}

// LaunchByActivity launches ARC app by launch its activity.
func (app *App) LaunchByActivity(ctx context.Context, actName string) (time.Duration, error) {
	act, err := arc.NewActivity(app.A, app.PkgName, actName)
	if err != nil {
		return -1, errors.Wrap(err, "failed to create the arc activity")
	}

	var startTime time.Time
	// Sometimes the app will fail to open, so add retry here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		startTime = time.Now()
		if err := act.Start(ctx, app.Tconn); err != nil {
			return errors.Wrapf(err, "failed to start the %s app", app.PkgName)
		}
		return ash.WaitForVisible(ctx, app.Tconn, app.PkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return -1, errors.Wrapf(err, "failed to wait for the new window of %s", app.PkgName)
	}

	app.launched = true
	return time.Since(startTime), nil
}

// LaunchAndAllowPermission launches ARC app and allow the permissions from permission controller window.
// If the permission controller window shows, the app window won't be visible until allowing the permissions.
func (app *App) LaunchAndAllowPermission(ctx context.Context) (time.Duration, error) {
	if w, err := ash.GetARCAppWindowInfo(ctx, app.Tconn, app.PkgName); err == nil {
		// If the package is already visible,
		// needs to close it and launch again to collect app start time.
		if err := w.CloseWindow(ctx, app.Tconn); err != nil {
			return -1, errors.Wrapf(err, "failed to close %s app", app.AppName)
		}
	}

	var startTime time.Time
	// Sometimes the app will fail to open, so add retry here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		startTime = time.Now()
		if err := launcher.SearchAndLaunch(app.Tconn, app.Kb, app.AppName)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s app", app.AppName)
		}

		// If the permission controller window shows, the app window won't be visivle until allowing the permissions.
		if err := ash.WaitForVisible(ctx, app.Tconn, "com.google.android.permissioncontroller"); err == nil {
			if err := FindAndClick(app.D.Object(ui.Text("CONTINUE")), 3*time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to click 'CONTUNUE'")
			}
		}

		return ash.WaitForVisible(ctx, app.Tconn, app.PkgName)
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return -1, errors.Wrapf(err, "failed to wait for the new window of %s", app.PkgName)
	}

	app.launched = true
	return time.Since(startTime), nil
}

// GetVersion gets the version of the ARC app.
func (app *App) GetVersion(ctx context.Context) (version string, err error) {
	out, err := app.A.Command(ctx, "dumpsys", "package", app.PkgName).Output()
	if err != nil {
		return "", err
	}
	versionNamePrefix := "versionName="
	output := string(out)
	splitOutput := strings.Split(output, "\n")
	for splitLine := range splitOutput {
		if strings.Contains(splitOutput[splitLine], versionNamePrefix) {
			version := strings.Split(splitOutput[splitLine], "=")[1]
			testing.ContextLog(ctx, "Version is: ", version)
			return version, nil
		}
	}
	return "", errors.New("failed to find versionName")
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

// FindAndClick returns an action function which finds and clicks Android ui object.
func FindAndClick(obj *ui.Object, timeout time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		if err := obj.WaitForExists(ctx, timeout); err != nil {
			return errors.Wrap(err, "failed to find the target object")
		}
		if err := obj.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the target object")
		}
		return nil
	}
}

// ClickIfExist returns an action function which clicks the UI object if it exists.
func ClickIfExist(obj *ui.Object, timeout time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		if err := obj.WaitForExists(ctx, timeout); err != nil {
			if ui.IsTimeout(err) {
				return nil
			}
			return errors.Wrap(err, "failed to wait for the target object")
		}
		return obj.Click(ctx)
	}
}

// WaitForExists returns an action function which wait for Android ui object.
func WaitForExists(obj *ui.Object, timeout time.Duration) uiauto.Action {
	return func(ctx context.Context) error {
		return obj.WaitForExists(ctx, timeout)
	}
}
