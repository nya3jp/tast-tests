// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil contains functionality shared by tast tests for android apps on Chromebooks.
package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Variables used by other tast tests
const (
	AndroidButtonClassName = "android.widget.Button"

	defaultTestCaseTimeout = 2 * time.Minute
	DefaultUITimeout       = 20 * time.Second
	ShortUITimeout         = 30 * time.Second
	LongUITimeout          = 90 * time.Second
)

// TestFunc represents the "test" function.
type TestFunc func(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string)

// TestCase represents the  name of test, and the function to call.
type TestCase struct {
	Name    string
	Fn      TestFunc
	Timeout time.Duration
}

// RunTestCases setups the device and runs all app compat test cases.
func RunTestCases(ctx context.Context, s *testing.State, appPkgName, appActivity string, testCases []TestCase) {
	// Step up chrome on Chromebook.
	cr, tconn, a := SetUpDevice(ctx, s, appPkgName, appActivity)

	// Ensure app launches before test cases.
	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()
	// TODO(b/166637700): Remove this if a proper solution is found that doesn't require the display to be on.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Log("Failed to ensure the display is on: ", err)
	}
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app before test cases: ", err)
	}
	if window, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName); err != nil {
		s.Fatal("Failed to get window info: ", err)
	} else if err := window.CloseWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to close app window before test cases: ", err)
	}
	if err := act.Stop(ctx, tconn); err != nil {
		s.Fatal("Failed to stop app before test cases: ", err)
	}
	s.Log("Successfully tested launching and closing the app")

	// Run the different test cases.
	for idx, test := range testCases {
		// If a timeout is not specified, limited individual test cases to the default.
		// This makes sure that one test case doesn't use all of the time when it fails.
		timeout := defaultTestCaseTimeout
		if test.Timeout != 0 {
			timeout = test.Timeout
		}
		testCaseCtx, cancel := ctxutil.Shorten(ctx, timeout)
		defer cancel()
		s.Run(testCaseCtx, test.Name, func(cleanupCtx context.Context, s *testing.State) {
			// Save time for cleanup and screenshot.
			ctx, cancel := ctxutil.Shorten(cleanupCtx, 20*time.Second)
			defer cancel()
			// TODO(b/166637700): Remove this if a proper solution is found that doesn't require the display to be on.
			if err := power.TurnOnDisplay(ctx); err != nil {
				s.Log("Failed to ensure the display is on: ", err)
			}
			// Launch the app.
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			s.Log("App launched successfully")

			// Close the app between iterations.
			defer func(ctx context.Context) {
				if window, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName); err != nil {
					s.Fatal("Failed to get window info: ", err)
				} else if err := window.CloseWindow(ctx, tconn); err != nil {
					s.Fatal("Failed to close app window: ", err)
				}
				if err := act.Stop(ctx, tconn); err != nil {
					s.Fatal("Failed to stop app: ", err)
				}
			}(cleanupCtx)

			// Take screenshot and dump ui info on failure.
			defer func(ctx context.Context) {
				if s.HasError() {
					filename := fmt.Sprintf("screenshot-arcappcompat-failed-test-%d.png", idx)
					path := filepath.Join(s.OutDir(), filename)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						s.Log("Failed to capture screenshot: ", err)
					}
					if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
						s.Log("Failed to dump UIAutomator: ", err)
						return
					}
					filename = fmt.Sprintf("ui-dump-arcappcompat-failed-test-%d.xml", idx)
					path = filepath.Join(s.OutDir(), filename)
					if err := a.PullFile(ctx, "/sdcard/window_dump.xml", path); err != nil {
						s.Log("Failed to pull UIAutomator dump: ", err)
					}
				}
			}(cleanupCtx)

			d, err := a.NewUIDevice(ctx)
			if err != nil {
				s.Fatal("Failed initializing UI Automator: ", err)
			}
			defer d.Close(ctx)

			DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

			// It is ok if the package is currently equal the installer package.
			// It is also ok if the package is currently equal the play service package.
			// It is also ok if the package is currently equal the android permission controller package
			// This happens when you need to accept permissions.
			if currentAppPkg, err := CurrentAppPackage(ctx, d); err != nil {
				s.Fatal("Failed to get current app package: ", err)
			} else if currentAppPkg != appPkgName && currentAppPkg != "com.google.android.packageinstaller" && currentAppPkg != "com.google.android.gms" && currentAppPkg != "com.google.android.permissioncontroller" {
				s.Fatalf("Failed to launch app: incorrect package(expected: %s, actual: %s)", appPkgName, currentAppPkg)
			}
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
		cancel()
	}
}

// SetUpDevice func setup Chrome on Chromebook.
func SetUpDevice(ctx context.Context, s *testing.State, appPkgName, appActivity string) (*chrome.Chrome, *chrome.TestConn, *arc.ARC) {
	// Setup Chrome.
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)
	s.Log("Enable showing ANRs")
	if err := a.Command(ctx, "settings", "put", "secure", "anr_show_background", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable showing ANRs: ", err)
	}
	s.Log("Enable crash dialog")
	if err := a.Command(ctx, "settings", "put", "secure", "show_first_crash_dialog_dev_option", "1").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to enable crash dialog: ", err)
	}

	s.Log("Installing app")
	// TODO(b/166637700): Remove this if a proper solution is found that doesn't require the display to be on.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Log("Failed to ensure the display is on: ", err)
	}
	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName, 3); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	// To get app version name.
	out, err := exec.Command("adb", "shell", "dumpsys", "package", appPkgName, "\t|\t", "grep", "\t", "versionName").Output()
	if err != nil {
		errors.Wrap(err, "could not get version name")
	} else {
		output := string(out)
		versionNameAfterSplit := strings.Split(output, "=")[1]
		s.Log("Version name of ", appPkgName, " is: ", versionNameAfterSplit)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}
	return cr, tconn, a
}

// ClamshellFullscreenApp Test launches the app in full screen window and verifies launch successfully without crash or ANR.
func ClamshellFullscreenApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	s.Log("Setting the window to fullscreen")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
		s.Fatal(" Failed to set the window to fullscreen: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
		s.Fatal("The window is not in fullscreen: ", err)
	}

	if !isNApp(ctx, s, tconn, a, d, appPkgName) {
		restartApp(ctx, s, tconn, a, d, appPkgName)
	}

	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
}

// MinimizeRestoreApp Test "minimize and relaunch the app" and verifies app relaunch successfully without crash or ANR.
func MinimizeRestoreApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	s.Log("Minimizing the window")
	defaultState, err := ash.GetARCAppWindowState(ctx, tconn, appPkgName)
	if err != nil {
		s.Error("Failed to get the default window state: ", err)
	}
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventMinimize); err != nil {
		s.Error("Failed to minimize the window: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateMinimized); err != nil {
		s.Error("The window is not minimized: ", err)
	}

	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)

	s.Log("Restoring the window")
	var restoreEvent ash.WMEventType
	switch defaultState {
	case ash.WindowStateFullscreen:
		restoreEvent = ash.WMEventFullscreen
	case ash.WindowStateMaximized:
		restoreEvent = ash.WMEventMaximize
	default:
		restoreEvent = ash.WMEventNormal
	}
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, restoreEvent); err != nil {
		s.Error("Failed to restore the window: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, defaultState); err != nil {
		s.Error("The window is not restored: ", err)
	}
}

// ClamshellResizeWindow Test "resize and restore back to original state of the app" and verifies app launch successfully without crash or ANR.
func ClamshellResizeWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	info, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName)
	if err != nil {
		s.Error("Failed to get window info: ", err)
	}
	s.Logf("App Resize info, info.CanResize %+v", info.CanResize)
	if !info.CanResize {
		s.Log("This app is not resizable. Skipping test")
		return
	}
	goalState := ash.WindowStateMaximized
	if info.State == ash.WindowStateFullscreen {
		goalState = ash.WindowStateFullscreen
	}

	if isNApp(ctx, s, tconn, a, d, appPkgName) {
		s.Log("N-apps start maximized. Reseting window to normal size")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
			s.Error("Failed to reset window to normal size: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
			s.Error("The window is not normalized: ", err)
		}

		DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
	}

	s.Log("Maximizing the window")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventTypeForState(goalState)); err != nil {
		s.Log("Failed to maximize the window: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, goalState); err != nil {
		s.Log("The window is not maximized: ", err)
	}

	if !isNApp(ctx, s, tconn, a, d, appPkgName) {
		restartApp(ctx, s, tconn, a, d, appPkgName)
	}

	DetectAndCloseCrashOrAppNotResponding(ctx, s, tconn, a, d, appPkgName)
}

func restartApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName string) {
	const restartButtonResourceID = "android:id/button1"

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
		s.Fatal("Failed to wait for window updated: ", err)
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
	if window, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName); err != nil {
		s.Fatal("Failed to get window info: ", err)
	} else if err := window.CloseWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to close app window: ", err)
	}
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

	// Wait for app to launch.
	d.WaitForIdle(ctx, ShortUITimeout)
	info, err := d.GetInfo(ctx)
	if err != nil {
		return "", err
	}
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
