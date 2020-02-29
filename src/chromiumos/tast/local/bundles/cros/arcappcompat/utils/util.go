// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains functionality shared by tast tests for android apps on Chromebooks.
package utils

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

const (
	androidButtonClassName = "android.widget.Button"

	defaultUITimeout = 20 * time.Second
	longUITimeout    = 5 * time.Minute
)

// ClamshellFullscreenApp Test launches the app in full screen window and verifies launch successfully without crash or ANR.
func ClamshellFullscreenApp(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) error {
	const (
		restartButtonResourceID = "android:id/button1"
	)
	isNApp := FindSDKVersion(ctx, s, tconn, a, d, appPkgName)

	testing.ContextLog(ctx, "Fullscreen the window")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
		s.Log("Full screen the window is failed: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
		s.Error("The window is not in fullscreen: ", err)
	}
	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}
	if isNApp {
		s.Log("It's a N app")
		// Click on restart button.
		restartButton := d.Object(ui.ResourceID(restartButtonResourceID))
		must(restartButton.WaitForExists(ctx, longUITimeout))
		must(restartButton.Click(ctx))

		if _, err := d.WaitForWindowUpdate(ctx, appPkgName, longUITimeout); err != nil {
			s.Fatal("Failed to wait window updated: ", err)
		}
	} else {
		s.Log("It's not a N app")
	}
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app package name: ", err)
	} else {
		s.Logf("App package name %+v", info.CurrentPackagename)
	}
	return nil
}

// MinimizeRestoreApp Test "minimize and relaunch the app" and verifies app relaunch successfully without crash or ANR.
func MinimizeRestoreApp(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) error {
	testing.ContextLog(ctx, "Minimize the window")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMinimize); err != nil {
		s.Error("Minimize the window is failed: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMinimized); err != nil {
		s.Error("The window is not minimized: ", err)
	}
	// Create a gmail app activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	} else {
		testing.ContextLog(ctx, "App is relaunched successfully")
	}
	defer act.Close()
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	// Launch the activity.
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Gmail app: ", err)
	}
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app package name: ", err)
	} else {
		s.Logf("App package name %+v", info.CurrentPackagename)
	}
	return nil
}

// ClamshellResizeWindow Test "resize and restore back to original state of the app" and verifies app launch successfully without crash or ANR.
func ClamshellResizeWindow(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) error {
	const (
		restartButtonResourceID = "android:id/button1"
	)
	isNApp := FindSDKVersion(ctx, s, tconn, a, d, appPkgName)

	if isNApp {
		s.Log("It's a N app")
		testing.ContextLog(ctx, "Maximize the window")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMaximize); err != nil {
			s.Log("Maximize the window is failed: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMaximized); err != nil {
			s.Log("The window is not maximized: ", err)
		}
		must := func(err error) {
			if err != nil {
				s.Fatal(err) // NOLINT: arc/ui returns loggable errors
			}
		}
		// Click on restart button.
		restartButton := d.Object(ui.ResourceID(restartButtonResourceID))
		must(restartButton.WaitForExists(ctx, longUITimeout))
		must(restartButton.Click(ctx))

		if _, err := d.WaitForWindowUpdate(ctx, appPkgName, longUITimeout); err != nil {
			s.Fatal("Failed to wait window updated: ", err)
		}
		DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

		info, err := d.GetInfo(ctx)
		if err != nil {
			s.Log("Failed to get app package name: ", err)
		} else {
			s.Logf("Current app package name %+v", info.CurrentPackagename)
		}

	} else {
		s.Log("It's not a N app")
		s.Log("Get to Normal size of the window")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
			s.Error("Normal size of the window is failed: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
			s.Error("The window is not normalized: ", err)
		}
		DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

		info, err := d.GetInfo(ctx)
		if err != nil {
			s.Log("Failed to get app package name: ", err)
		} else {
			s.Logf("Current app package name %+v", info.CurrentPackagename)
		}
		testing.ContextLog(ctx, "Get back to maximized window state")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMaximize); err != nil {
			s.Log("Maximize the window is failed: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMaximized); err != nil {
			s.Log("The window is not maximized: ", err)
		}
	}
	return nil
}

// FindSDKVersion func to check if it is N or pre-N app
func FindSDKVersion(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string) bool {
	var nApp bool
	nApp = false
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app sdk version: ", err)
	} else {
		s.Logf("App sdk version %+v", info.SDKInt)
	}
	if info.SDKInt >= 24 {
		testing.ContextLog(ctx, "It's not a N app")
	} else {
		testing.ContextLog(ctx, "It's a N app")
		nApp = true
	}
	return nApp
}

// ReOpenWindow Test "close and relaunch the app" and verifies app launch successfully without crash or ANR.
func ReOpenWindow(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string, appActivity string) error {

	// Create a gmail app activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	} else {
		testing.ContextLog(ctx, "App is launched successfully")
	}
	defer act.Close()

	testing.ContextLog(ctx, "Close the app")
	defer act.Stop(ctx)

	// Relaunch the app.
	act, err = arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	} else {
		testing.ContextLog(ctx, "App is relaunched successfully")
	}
	defer act.Close()
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app package name: ", err)
	} else {
		s.Logf("Current app package name %+v", info.CurrentPackagename)
	}
	return nil
}

// DetectAndCloseCrashOrAppNotResponding func to handle Crash or ANR.
func DetectAndCloseCrashOrAppNotResponding(ctx context.Context, s *testing.State, tconn *chrome.Conn, a *arc.ARC, d *ui.Device, appPkgName string) {
	const (
		alertTitleCanNotDownloadText = "Can't download app"
		alertTitleHasStoppedText     = "has stopped"
		alertTitleKeepsStoppingText  = "keeps stopping"
		alertTitleNotRespondingText  = "isn't responding"
		alertTitleOpenAppAgainText   = "Open app again"
		closeAppText                 = "Close"
		okText                       = "ok"
		OpenAppAgainText             = "Open app again"
	)

	// Check for isn't responding alert title
	alertTitleCanNotDownload := d.Object(ui.TextContains(alertTitleCanNotDownloadText))
	alertTitleHasStopped := d.Object(ui.TextContains(alertTitleHasStoppedText))
	alertTitleKeepsStopping := d.Object(ui.TextContains(alertTitleKeepsStoppingText))
	alertTitleNotResponding := d.Object(ui.TextContains(alertTitleNotRespondingText))
	alertTitleOpenAppAgain := d.Object(ui.TextContains(alertTitleOpenAppAgainText))

	if err := alertTitleNotResponding.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Alert TitleNotResponding doesn't exist: ", err)
	} else if err := alertTitleHasStopped.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Alert TitleNotResponding doesn't exist: ", err)
	} else if err := alertTitleKeepsStopping.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Alert TitleKeepsStopping doesn't exist: ", err)
	} else if err := alertTitleOpenAppAgain.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Alert TitleOpenAppAgain doesn't exist: ", err)
	} else if err := alertTitleCanNotDownload.WaitForExists(ctx, defaultUITimeout); err != nil {
		s.Log("Alert TitleCanNotDownload doesn't exist: ", err)
	} else {
		s.Log("Alert Title does exist")
		// Click on Open App Again
		openAppAgainButton := d.Object(ui.ClassName(androidButtonClassName), ui.TextContains(OpenAppAgainText))
		if err := openAppAgainButton.WaitForExists(ctx, defaultUITimeout); err != nil {
			s.Error("OpenAppAgainButton doesn't exist: ", err)
		} else {
			openAppAgainButton.Click(ctx)
		}
		// Click on close app
		closeButton := d.Object(ui.ClassName(androidButtonClassName), ui.TextContains(closeAppText))
		if err := closeButton.WaitForExists(ctx, defaultUITimeout); err != nil {
			s.Error("CloseButton doesn't exist: ", err)
		} else {
			closeButton.Click(ctx)
		}
		// Click on ok button
		okButton := d.Object(ui.ClassName(androidButtonClassName), ui.TextContains(okText))
		if err := okButton.WaitForExists(ctx, defaultUITimeout); err != nil {
			s.Error("OkButton doesn't exist: ", err)
		} else {
			okButton.Click(ctx)
		}

	}
}
