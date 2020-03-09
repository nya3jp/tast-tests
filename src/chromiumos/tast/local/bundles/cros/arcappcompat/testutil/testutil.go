// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// Package testutil contains functionality shared by tast tests for android apps on Chromebooks.
package testutil

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"context"
	"time"
)

// Variables used by other tast tests
const (
	AndroidButtonClassName = "android.widget.Button"
	DefaultUITimeout       = 20 * time.Second
	LongUITimeout          = 5 * time.Minute
)

// TestFunc represents the "test" function.
type TestFunc func(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string)

// TestSuite represents the  name of test, and the function to call.
type TestSuite struct {
	Name string
	Fn   TestFunc
}

// TestParams represents the collection of tests to run in tablet mode or clamshell mode.
type TestParams struct {
	TabletMode bool
	Tests      []TestSuite
}

// FindTheError finds the error.
func FindTheError(s *testing.State, err error) {
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
}

// LaunchHelper helps to launch the installed app.
func LaunchHelper(ctx context.Context, s *testing.State, d *ui.Device) {
	const (
		openButtonRegex = "Open|OPEN"
	)
	// Click on open button.
	openButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextMatches(openButtonRegex))
	FindTheError(s, openButton.WaitForExists(ctx, LongUITimeout))
	// Open button exist and click.
	FindTheError(s, openButton.Click(ctx))
}

// ClamshellFullscreenApp Test launches the app in full screen window and verifies launch successfully without crash or ANR.
func ClamshellFullscreenApp(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		restartButtonResourceID = "android:id/button1"
	)
	s.Log("Set the window to fullscreen")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
		s.Error(" Failed to set the window to fullscreen: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
		s.Error("The window is not in fullscreen: ", err)
	}
	nApp := isNApp(ctx, s, tconn, a, d, appPkgName)
	if !nApp {
		s.Log("It's a pre N-app")
		// Click on restart button.
		restartButton := d.Object(ui.ResourceID(restartButtonResourceID))
		FindTheError(s, restartButton.WaitForExists(ctx, LongUITimeout))
		FindTheError(s, restartButton.Click(ctx))
		if _, err := d.WaitForWindowUpdate(ctx, appPkgName, LongUITimeout); err != nil {
			s.Fatal("Failed to wait for the window to update: ", err)
		}
	} else {
		s.Log("It's an N-app")
	}
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	currentAppPackage(ctx, s, d)
}

// MinimizeRestoreApp Test "minimize and relaunch the app" and verifies app relaunch successfully without crash or ANR.
func MinimizeRestoreApp(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	s.Log("Minimize the window")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMinimize); err != nil {
		s.Error("Failed to minimize the window: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMinimized); err != nil {
		s.Error("The window is not minimized: ", err)
	}
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	// Create a gmail app activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	} else {
		s.Log("Created new app activity")
	}
	defer act.Close()
	// Launch the activity.
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start app: ", err)
	} else {
		s.Log("App relaunched successfully")
	}
	currentAppPackage(ctx, s, d)
}

// ClamshellResizeWindow Test "resize and restore back to original state of the app" and verifies app launch successfully without crash or ANR.
func ClamshellResizeWindow(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	const (
		restartButtonResourceID = "android:id/button1"
	)
	nApp := isNApp(ctx, s, tconn, a, d, appPkgName)
	if !nApp {
		s.Log("It's a pre N-app")
		s.Log("Maximize the window")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMaximize); err != nil {
			s.Log("Failed to maximize the window: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMaximized); err != nil {
			s.Log("The window is not maximized: ", err)
		}
		// Click on restart button.
		restartButton := d.Object(ui.ResourceID(restartButtonResourceID))
		FindTheError(s, restartButton.WaitForExists(ctx, LongUITimeout))
		FindTheError(s, restartButton.Click(ctx))
		if _, err := d.WaitForWindowUpdate(ctx, appPkgName, LongUITimeout); err != nil {
			s.Fatal("Failed to wait window updated: ", err)
		}
		DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
		currentAppPackage(ctx, s, d)
	} else {
		s.Log("It's an N-app")
		s.Log("Get to Normal size of the window")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
			s.Error("Normal size of the window is failed: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
			s.Error("The window is not normalized: ", err)
		}
		DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
		currentAppPackage(ctx, s, d)
		s.Log("Get back to maximized window state")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMaximize); err != nil {
			s.Log("Maximize the window is failed: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMaximized); err != nil {
			s.Log("The window is not maximized: ", err)
		}
	}
}

// isNApp func to check if it is an N or pre-N app
func isNApp(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string) bool {
	var nApp bool
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app sdk version: ", err)
	} else {
		s.Logf("App sdk version %+v", info.SDKInt)
	}
	if info.SDKInt >= 24 {
		nApp = true
		s.Log("It's an N-app")
	} else {
		s.Log("It's a pre N-app")
	}
	return nApp
}

// ReOpenWindow Test "close and relaunch the app" and verifies app launch successfully without crash or ANR.
func ReOpenWindow(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) {
	// Create a gmail app activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	} else {
		s.Log("Created new app activity")
	}
	defer act.Close()
	// Launch the activity.
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start app: ", err)
	} else {
		s.Log("App launched successfully")
	}
	s.Log("Close the app")
	if err := act.Stop(ctx); err != nil {
		s.Fatal("Failed to close the app: ", err)
	}
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	// ReLaunch the activity.
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start app: ", err)
	} else {
		s.Log("App relaunched successfully")
	}
	currentAppPackage(ctx, s, d)
}

// currentAppPackage func to get info on current package name
func currentAppPackage(ctx context.Context, s *testing.State, d *ui.Device) {
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app package name: ", err)
	} else {
		s.Logf("Current app package name %+v", info.CurrentPackagename)
	}
}

// DetectAndCloseCrashOrAppNotResponding func to handle Crash or ANR.
func DetectAndCloseCrashOrAppNotResponding(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string) {
	const (
		alertTitleCanNotDownloadText = "Can't download app"
		alertTitleHasStoppedText     = "has stopped"
		alertTitleKeepsStoppingText  = "keeps stopping"
		alertTitleNotRespondingText  = "isn't responding"
		alertTitleOpenAppAgainText   = "Open app again"
		shortUITimeout               = 2 * time.Second
	)
	// Check for isn't responding alert title
	alertTitleCanNotDownload := d.Object(ui.TextContains(alertTitleCanNotDownloadText))
	alertTitleHasStopped := d.Object(ui.TextContains(alertTitleHasStoppedText))
	alertTitleKeepsStopping := d.Object(ui.TextContains(alertTitleKeepsStoppingText))
	alertTitleNotResponding := d.Object(ui.TextContains(alertTitleNotRespondingText))
	alertTitleOpenAppAgain := d.Object(ui.TextContains(alertTitleOpenAppAgainText))
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := alertTitleNotResponding.WaitForExists(ctx, shortUITimeout); err != nil {
		} else {
			return testing.PollBreak(errors.Wrap(err, "NotResponding does exist"))
		}
		if err := alertTitleHasStopped.WaitForExists(ctx, shortUITimeout); err != nil {
		} else {
			return testing.PollBreak(errors.Wrap(err, "HasStopped does exist"))
		}
		if err := alertTitleKeepsStopping.WaitForExists(ctx, shortUITimeout); err != nil {
		} else {
			return testing.PollBreak(errors.Wrap(err, "KeepsStopping does exist"))
		}
		if err := alertTitleOpenAppAgain.WaitForExists(ctx, shortUITimeout); err != nil {
		} else {
			return testing.PollBreak(errors.Wrap(err, "OpenAppAgain does exist"))
		}
		if err := alertTitleCanNotDownload.WaitForExists(ctx, shortUITimeout); err != nil {
		} else {
			return testing.PollBreak(errors.Wrap(err, "OpenAppAgain does exist"))
		}
		return nil
	}, &testing.PollOptions{Timeout: shortUITimeout}); err != nil {
		s.Log("Alert Title does exist: ", err)
		handleCrashOrANRDialog(ctx, s, d)
	}
}

// handleCrashOrANRDialog func will handle the crash or ANR dialog box
func handleCrashOrANRDialog(ctx context.Context, s *testing.State, d *ui.Device) {
	const (
		closeAppText     = "Close"
		okText           = "ok"
		OpenAppAgainText = "Open app again"
	)
	// Click on Open App Again
	openAppAgainButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextContains(OpenAppAgainText))
	if err := openAppAgainButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Error("OpenAppAgainButton doesn't exist: ", err)
	} else {
		FindTheError(s, openAppAgainButton.Click(ctx))
	}
	// Click on close app
	closeButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextContains(closeAppText))
	if err := closeButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Error("CloseButton doesn't exist: ", err)
	} else {
		FindTheError(s, closeButton.Click(ctx))
	}
	// Click on ok button
	okButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextContains(okText))
	if err := okButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Error("OkButton doesn't exist: ", err)
	} else {
		FindTheError(s, okButton.Click(ctx))
	}
}
