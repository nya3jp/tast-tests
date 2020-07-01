// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil contains functionality shared by tast tests for android apps on Chromebooks.
package testutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/screenshot"
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

// TestCase represents the  name of test, and the function to call.
type TestCase struct {
	Name string
	Fn   TestFunc
}

// RunTestCases setups the device and runs all app compat test cases.
func RunTestCases(ctx context.Context, s *testing.State, appPkgName, appActivity string, testCases []TestCase) {
	// Step up chrome on Chromebook.
	cr, tconn, a, d := SetUpDevice(ctx, s, appPkgName, appActivity)
	defer d.Close()

	// Run the different test cases.
	for idx, test := range testCases {
		// Run subtests.
		s.Run(ctx, test.Name, func(ctx context.Context, s *testing.State) {
			// Launch the app.
			act, err := arc.NewActivity(a, appPkgName, appActivity)
			if err != nil {
				s.Fatal("Failed to create new app activity: ", err)
			}
			s.Log("Created new app activity")

			defer act.Close()
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed start app: ", err)
			}
			s.Log("App launched successfully")

			defer act.Stop(ctx, tconn)

			// Take screenshot on failure.
			defer func() {
				if s.HasError() {
					path := fmt.Sprintf("%s/screenshot-arcappcompat-failed-test-%d.png", s.OutDir(), idx)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
				}
			}()

			DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

			if currentAppPkg, err := CurrentAppPackage(ctx, d); err != nil {
				s.Fatal("Failed to get current app package: ", err)
			} else if currentAppPkg != appPkgName {
				s.Fatalf("Failed to launch app: incorrect package(expected: %s, actual: %s)", appPkgName, currentAppPkg)
			}
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
	}
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
	if err := playstore.InstallApp(ctx, a, d, appPkgName); err != nil {
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

	s.Log("Setting the window to fullscreen")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
		s.Fatal(" Failed to set the window to fullscreen: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
		s.Fatal("The window is not in fullscreen: ", err)
	}

	if !isNApp(ctx, s, tconn, a, d, appPkgName) {
		// Click on restart button.
		s.Log("It's a pre N-app; Attempting restart")
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
	}

	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
}

// MinimizeRestoreApp Test "minimize and relaunch the app" and verifies app relaunch successfully without crash or ANR.
func MinimizeRestoreApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	s.Log("Minimizing the window")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMinimize); err != nil {
		s.Error("Failed to minimize the window: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMinimized); err != nil {
		s.Error("The window is not minimized: ", err)
	}

	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

	// Create an activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()

	// Launch the activity.
	s.Log("Relaunching the app to restore from minimize")
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to restart app: ", err)
	}
}

// ClamshellResizeWindow Test "resize and restore back to original state of the app" and verifies app launch successfully without crash or ANR.
func ClamshellResizeWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const restartButtonResourceID = "android:id/button1"

	if !isNApp(ctx, s, tconn, a, d, appPkgName) {
		s.Log("Maximizing the window")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMaximize); err != nil {
			s.Log("Failed to maximize the window: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMaximized); err != nil {
			s.Log("The window is not maximized: ", err)
		}

		// Click on restart button.
		s.Log("It's a pre N-app; Attempting restart")
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
	} else {
		act, err := arc.NewActivity(a, appPkgName, appActivity)
		if err != nil {
			s.Fatal("Failed to create new app activity: ", err)
		}
		defer act.Close()

		s.Log("It's an N-app; Checking if Resizable")
		resizable, err := act.PackageResizable(ctx)
		if err != nil {
			s.Fatal("Failed get the resizable info: ", err)
		}
		if resizable {
			s.Log("App is resizable")
			s.Log("Reseting window to normal size")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
				s.Error("Failed to reset window to normal size: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
				s.Error("The window is not normalized: ", err)
			}

			DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

			s.Log("Setting window back to maximzied")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMaximize); err != nil {
				s.Log("Failed to set window to maximized: ", err)
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
	// Create an activity handle.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()

	// Close the app.
	s.Log("Closing the app")
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app: ", err)
	}
	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

	// Relaunch the app.
	s.Log("Relaunching the app")
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to restart app: ", err)
	}
}

// CurrentAppPackage func to get info on current package name
func CurrentAppPackage(ctx context.Context, d *ui.Device) (string, error) {
	info, err := d.GetInfo(ctx)
	if err != nil {
		return "", err
	}
	// s.Logf("Current app package name %+v", info.CurrentPackagename)
	return info.CurrentPackagename, nil
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
	// Click on open app again
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
