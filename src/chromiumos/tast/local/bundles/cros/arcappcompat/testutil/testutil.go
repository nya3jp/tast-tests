// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil contains functionality shared by tast tests for android apps on Chromebooks.
package testutil

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Variables used by other tast tests
const (
	AndroidButtonClassName = "android.widget.Button"

	DefaultUITimeout = 20 * time.Second
	LongUITimeout    = 5 * time.Minute
)

// TestFunc represents the "test" function.
type TestFunc func(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string)

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

// SetUpDevice func setup Chrome on Chromebook.
func SetUpDevice(ctx context.Context, s *testing.State, appPkgName, appActivity string) (*chrome.Chrome, *chrome.TestConn, *arc.ARC, *ui.Device) {

	// Setup Chrome.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer func() {
		if s.HasError() {
			d.Close()
		}
	}()
	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable crash dialog: ", err)
	}

	s.Log("Installing app")
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName, 3); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}
	return cr, tconn, a, d
}

// ClamshellFullscreenApp Test launches the app in full screen window and verifies launch successfully without crash or ANR.
func ClamshellFullscreenApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const restartButtonResourceID = "android:id/button1"

	if currentAppPkg := CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("entered ClamshellFullscreenApp and failed to launch the app: ", currentAppPkg)
	}
	s.Log("App launched successfully and entered ClamshellFullscreenApp")

	s.Log("Set the window to fullscreen")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
		s.Error(" Failed to set the window to fullscreen: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
		s.Error("The window is not in fullscreen: ", err)
	}

	if !isNApp(ctx, s, tconn, a, d, appPkgName) {
		s.Log("It's a pre N-app")
		// Click on restart button.
		restartButton := d.Object(ui.ResourceID(restartButtonResourceID))
		if err := restartButton.WaitForExists(ctx, LongUITimeout); err != nil {
			s.Fatal("Restart button does not exist: ", err)
		}
		if err := restartButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on restart button: ", err)
		}

		if _, err := d.WaitForWindowUpdate(ctx, appPkgName, LongUITimeout); err != nil {
			s.Fatal("Failed to wait for the window to update: ", err)
		}
	} else {
		s.Log("It's an N-app")
	}
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	CurrentAppPackage(ctx, s, d)
}

// MinimizeRestoreApp Test "minimize and relaunch the app" and verifies app relaunch successfully without crash or ANR.
func MinimizeRestoreApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	if currentAppPkg := CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Entered MinimizeRestoreApp and failed to launch the app: ", currentAppPkg)
	}
	s.Log("App launched successfully and entered MinimizeRestoreApp")

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
	}
	s.Log("Created new app activity")
	defer act.Close()
	// Launch the activity.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start app: ", err)
	}
	s.Log("App relaunched successfully")

	CurrentAppPackage(ctx, s, d)
}

// ClamshellResizeWindow Test "resize and restore back to original state of the app" and verifies app launch successfully without crash or ANR.
func ClamshellResizeWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const restartButtonResourceID = "android:id/button1"

	if currentAppPkg := CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Entered ClamshellResizeWindow and failed to launch the app: ", currentAppPkg)
	}
	s.Log("App launched successfully and entered ClamshellResizeWindow")

	if !isNApp(ctx, s, tconn, a, d, appPkgName) {
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
		if err := restartButton.WaitForExists(ctx, LongUITimeout); err != nil {
			s.Fatal("Restart button does not exist: ", err)
		}
		if err := restartButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on restart button: ", err)
		}

		if _, err := d.WaitForWindowUpdate(ctx, appPkgName, LongUITimeout); err != nil {
			s.Fatal("Failed to wait window updated: ", err)
		}
		DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
		CurrentAppPackage(ctx, s, d)
	} else {
		s.Log("It's an N-app")
		// Launch the app.
		act, err := arc.NewActivity(a, appPkgName, appActivity)
		if err != nil {
			s.Fatal("Failed to create new app activity: ", err)
		}
		defer act.Close()
		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed start app: ", err)
		}
		defer act.Stop(ctx, tconn)

		checkForResizable, err := act.PackageResizable(ctx)
		if err != nil {
			s.Fatal("Failed get the resizable info: ", err)
		}
		s.Log("checkForResizable:", checkForResizable)
		if checkForResizable {
			s.Log("App is resizable")
			s.Log("Get to Normal size of the window")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
				s.Error("Normal size of the window is failed: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
				s.Error("The window is not normalized: ", err)
			}
			DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
			CurrentAppPackage(ctx, s, d)
			s.Log("Get back to maximized window state")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMaximize); err != nil {
				s.Log("Maximize the window is failed: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMaximized); err != nil {
				s.Log("The window is not maximized: ", err)
			}
		} else {
			s.Log("App is not resizable")
		}

	}

}

// isNApp func to check if it is an N or pre-N app
func isNApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName string) bool {
	var nApp bool

	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app sdk version: ", err)
		return false
	}
	s.Logf("App sdk version %+v", info.SDKInt)
	if info.SDKInt >= 24 {
		nApp = true
	}
	return nApp
}

// ReOpenWindow Test "close and relaunch the app" and verifies app launch successfully without crash or ANR.
func ReOpenWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	if currentAppPkg := CurrentAppPackage(ctx, s, d); currentAppPkg != appPkgName {
		s.Fatal("Entered ReOpenWindow and failed to launch the app: ", currentAppPkg)
	}
	s.Log("App launched successfully and entered ReOpenWindow")

	s.Log("Close the app")
	if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to close the app: ", err)
	}
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	// Create a gmail app activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	s.Log("Created new app activity")

	defer act.Close()
	// ReLaunch the activity.
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start app: ", err)
	}
	s.Log("App relaunched successfully")

	CurrentAppPackage(ctx, s, d)
}

// CurrentAppPackage func to get info on current package name
func CurrentAppPackage(ctx context.Context, s *testing.State, d *ui.Device) string {
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Log("Failed to get app package name: ", err)
		return ""
	}
	s.Logf("Current app package name %+v", info.CurrentPackagename)
	return info.CurrentPackagename
}

// DetectAndCloseCrashOrAppNotResponding func to handle Crash or ANR.
func DetectAndCloseCrashOrAppNotResponding(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName string) {
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

		if err := alertTitleNotResponding.Exists(ctx); err == nil {
			return testing.PollBreak(errors.Wrap(err, "NotResponding does exist"))
		}
		if err := alertTitleHasStopped.Exists(ctx); err == nil {
			return testing.PollBreak(errors.Wrap(err, "HasStopped does exist"))
		}
		if err := alertTitleKeepsStopping.Exists(ctx); err == nil {
			return testing.PollBreak(errors.Wrap(err, "KeepsStopping does exist"))
		}
		if err := alertTitleOpenAppAgain.Exists(ctx); err == nil {
			return testing.PollBreak(errors.Wrap(err, "OpenAppAgain does exist"))
		}
		if err := alertTitleCanNotDownload.Exists(ctx); err == nil {
			return testing.PollBreak(errors.Wrap(err, "CanNotDownload does exist"))
		}
		return errors.New("waiting for crash")
	}, &testing.PollOptions{Timeout: shortUITimeout}); err != nil && !strings.Contains(err.Error(), "waiting for crash") {
		s.Error("The application crashed: ", err)
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
	} else if err := openAppAgainButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on OpenAppAgainButton: ", err)
	}

	// Click on close app
	closeButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextContains(closeAppText))
	if err := closeButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Error("CloseButton doesn't exist: ", err)
	} else if err := closeButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on CloseButton: ", err)
	}

	// Click on ok button
	okButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextContains(okText))
	if err := okButton.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Error("OkButton doesn't exist: ", err)
	} else if err := okButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on OkButton: ", err)
	}
}
