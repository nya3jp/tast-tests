// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil contains functionality shared by tast tests for android apps on Chromebooks.
package testutil

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
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
	"chromiumos/tast/local/chrome/display"
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
	cr, tconn, a := setUpDevice(ctx, s, appPkgName, appActivity)

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
					} else {
						filename = fmt.Sprintf("ui-dump-arcappcompat-failed-test-%d.xml", idx)
						path = filepath.Join(s.OutDir(), filename)
						if err := a.PullFile(ctx, "/sdcard/window_dump.xml", path); err != nil {
							s.Log("Failed to pull UIAutomator dump: ", err)
						}
					}
					filename = fmt.Sprintf("bugreport-arcappcompat-failed-test-%d.zip", idx)
					path = filepath.Join(s.OutDir(), filename)
					if err := a.BugReport(ctx, path); err != nil {
						s.Log("Failed to get bug report: ", err)
					}
				}
			}(cleanupCtx)

			d, err := a.NewUIDevice(ctx)
			if err != nil {
				s.Fatal("Failed initializing UI Automator: ", err)
			}
			defer d.Close(ctx)

			DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

			// It is ok if the package is currently equal the installer package.
			// It is also ok if the package is currently equal the play service package.
			// It is also ok if the package is currently equal the android permission controller package
			// This happens when you need to accept permissions.
			if currentAppPkg, err := currentAppPackage(ctx, d); err != nil {
				s.Fatal("Failed to get current app package: ", err)
			} else if currentAppPkg != appPkgName && currentAppPkg != "com.google.android.packageinstaller" && currentAppPkg != "com.google.android.gms" && currentAppPkg != "com.google.android.permissioncontroller" {
				s.Fatalf("Failed to launch app: incorrect package(expected: %s, actual: %s)", appPkgName, currentAppPkg)
			}
			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity)
		})
		cancel()
	}
}

// setUpDevice func setup Chrome on Chromebook.
func setUpDevice(ctx context.Context, s *testing.State, appPkgName, appActivity string) (*chrome.Chrome, *chrome.TestConn, *arc.ARC) {
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
	out, err := a.Command(ctx, "dumpsys", "package", appPkgName).Output()
	if err != nil {
		s.Log(err, "could not get dumpsys package")
	} else {
		versionNamePrefix := "versionName="
		output := string(out)
		splitOutput := strings.Split(output, "\n")
		for splitLine := range splitOutput {
			if strings.Contains(splitOutput[splitLine], versionNamePrefix) {
				versionNameAfterSplit := strings.Split(splitOutput[splitLine], "=")[1]
				s.Log("Version name of ", appPkgName, " is: ", versionNameAfterSplit)
				break
			}
		}
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

	if !isNApp(ctx, d) {
		if err := restartApp(ctx, d, appPkgName); err != nil {
			s.Fatal("Failed to restart app: ", err)
		}
	}

	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
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

	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

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
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		s.Log("Device is in tablet mode. Skipping test")
		return
	}
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

	if isNApp(ctx, d) {
		s.Log("N-apps start maximized. Reseting window to normal size")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
			s.Error("Failed to reset window to normal size: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
			s.Error("The window is not normalized: ", err)
		}

		DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
	}

	s.Log("Maximizing the window")
	if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventTypeForState(goalState)); err != nil {
		s.Log("Failed to maximize the window: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, goalState); err != nil {
		s.Log("The window is not maximized: ", err)
	}

	if !isNApp(ctx, d) {
		if err := restartApp(ctx, d, appPkgName); err != nil {
			s.Fatal("Failed to restart app: ", err)
		}
	}

	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// TouchAndTextInputs func verify touch and text inputs in the app are working properly without crash or ANR.
func TouchAndTextInputs(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	// Press enter key twice.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	// To perform touch and text inputs.
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--pct-touch", "30", "--pct-nav", "10", "--pct-touch", "40", "--pct-nav", "10", "--pct-anyevent", "10", "--throttle", "100", "-v", "2000").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Error("Failed to perform monkey test touch and text inputs: ", err)
	}
	if err := processMonkeyOutput(string(out)); err != nil {
		s.Error("Touch and text inputs are not working properly in the app: ", err)
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// KeyboardNavigations func verifies app perform keyboard navigations successfully without crash or ANR.
func KeyboardNavigations(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		s.Log("Device is in tablet mode. Skipping test")
		return
	}
	// Press enter key twice.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	// To perform keyboard navigations.
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--pct-touch", "20", "--pct-nav", "20", "--pct-majornav", "20", "--pct-nav", "20", "--pct-majornav", "20", "--throttle", "100", "-v", "2000").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Error("Failed to perform monkey test keyboard navigations: ", err)
	}
	if err := processMonkeyOutput(string(out)); err != nil {
		s.Error("Key board navigations such as up/down/left/right are not working properly in the app: ", err)
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// TouchAndPlayVideo func verifies app perform touch and play video successfully without crash or ANR.
func TouchAndPlayVideo(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		s.Log("Device is in tablet mode. Skipping test")
		return
	}
	// Press enter key twice.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	// To perform touch and play video.
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--pct-touch", "60", "--throttle", "100", "-v", "2000").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Error("Failed to perform monkey test touch and play video content: ", err)
	}
	if err := processMonkeyOutput(string(out)); err != nil {
		s.Error("Touch and play videos are not working properly in the app: ", err)
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// TouchviewRotate Test verifies if app performs rotation successfully without crash or ANR.
func TouchviewRotate(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {

	info, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}
	s.Logf("App Display ID, info.DisplayID %+v", info.DisplayID)

	// Set display orientation to natural state 90 degree.
	if err := display.SetDisplayRotationSync(ctx, tconn, info.DisplayID, "Rotate90"); err != nil {
		s.Fatal("Failed to set app to 90 rotation: ", err)
	} else {
		s.Log("Set app to 90 rotation was successful")
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

	// Set display orientation to natural state 180 degree.
	if err := display.SetDisplayRotationSync(ctx, tconn, info.DisplayID, "Rotate180"); err != nil {
		s.Fatal("Failed to set app to 180 rotation: ", err)
	} else {
		s.Log("Set app to 180 rotation was successful")
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

	// Set display orientation to natural state 270 degree.
	if err := display.SetDisplayRotationSync(ctx, tconn, info.DisplayID, "Rotate270"); err != nil {
		s.Fatal("Failed to set app to 270 rotation: ", err)
	} else {
		s.Log("Set app to 270 rotation was successful")
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

	// Set display orientation to natural state 0 degree.
	if err := display.SetDisplayRotationSync(ctx, tconn, info.DisplayID, "Rotate0"); err != nil {
		s.Fatal("Failed to set app to 0 rotation: ", err)
	} else {
		s.Log("Set app to 0 rotation was successful")
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// MouseScrollAction func verifies app perform mouse scroll actions successfully without crash or ANR.
func MouseScrollAction(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	// To perform mouse scroll actions.
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--throttle", "100", "--pct-touch", "30", "--pct-trackball", "50", "-v", "1000").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Error("Failed to perform monkey test mouse scroll: ", err)
	}
	if err := processMonkeyOutput(string(out)); err != nil {
		s.Error("Mouse scroll is not working properly in the app: ", err)
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// TouchScreenScroll Test verifies app perform scrollForward successfully without crash or ANR.
func TouchScreenScroll(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	checkForScrollLayout := d.Object(ui.Scrollable(true), ui.Focusable(true), ui.Enabled(true))
	if err := checkForScrollLayout.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("ScrollLayout doesn't exist. Page is not scrollable and skipping the test: ", err)
		return
	}
	if err := checkForScrollLayout.ScrollForward(ctx, 50); err != nil {
		s.Fatal("Failed to scrollForward: ", err)
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// MouseClick func verifies mouse click work successfully in the app without crash or ANR.
func MouseClick(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	var (
		xCoordinate int
		yCoordinate int
	)
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		s.Log("Device is in tablet mode. Skipping test")
		return
	}
	checkUIElement := d.Object(ui.Clickable(true), ui.Focusable(true), ui.Enabled(true))
	if err := checkUIElement.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("checkUIElement doesn't exist and skipped mouse click: ", err)
		return
	}
	s.Log("checkUIElement does exists")
	if uiElementBounds, err := checkUIElement.GetBounds(ctx); err != nil {
		s.Log("Failed to get uiElementBounds and skipped mouse click : ", err)
	} else {
		s.Log("uiElementBounds: ", uiElementBounds)
		xCoordinate = uiElementBounds.Left
		s.Log("Xcoordinate: ", xCoordinate)
		yCoordinate = uiElementBounds.Top
		s.Log("Ycoordinate: ", yCoordinate)

		// To perform mouse click.
		out, err := a.Command(ctx, "input", "mouse", "tap", strconv.Itoa(xCoordinate), strconv.Itoa(yCoordinate)).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to perform mouse click: ", err)
		} else {
			s.Log("Performed mouse click: ", string(out))
		}
		DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
	}
}

// OrientationSize Test verifies orientation size of the app after launch.
func OrientationSize(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	const (
		maximizedSize = "Maximized"
		phoneSize     = "Phone"
		tabletSize    = "Tablet"
		blackBars     = "Black bars on both sides of an app"
	)

	appWidth, appHeight, err := getAppCoordinates(ctx, s, a, d, appPkgName)
	if err != nil {
		s.Fatal("Failed to get app coordinates: ", err)
	}
	s.Log("appWidth", appWidth)
	s.Log("appHeight", appHeight)

	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get device display: ", err)
	}
	deviceDisplayWidth := info.DisplayWidth
	s.Log("deviceDisplayWidth", deviceDisplayWidth)
	deviceDisplayHeight := info.DisplayHeight
	s.Log("deviceDisplayHeight", deviceDisplayHeight)

	if appWidth == deviceDisplayWidth {
		s.Log("Orientation size of an app: ", maximizedSize)
	} else if appWidth < deviceDisplayWidth/3 && appWidth != deviceDisplayWidth {
		s.Fatal("Orientation size of an app: ", phoneSize)
	} else if appWidth > deviceDisplayWidth/2 && appWidth <= deviceDisplayWidth*3/4 && appWidth != deviceDisplayWidth {
		s.Log("Orientation size of an app: ", tabletSize)
	} else if appWidth >= deviceDisplayWidth*3/4 && appHeight >= deviceDisplayHeight*3/4 && appWidth != deviceDisplayWidth {
		s.Log("Orientation size of an app: ", tabletSize)
	} else if appWidth < deviceDisplayWidth/2 && appHeight < deviceDisplayHeight/2 && appWidth != deviceDisplayWidth {
		s.Fatal("Orientation size of an app: ", phoneSize)
	} else if appWidth < deviceDisplayWidth/2 && appHeight > deviceDisplayHeight*3/4 && appWidth != deviceDisplayWidth {
		s.Fatal("Orientation size of an app: ", maximizedSize+". "+blackBars)
	} else if appWidth < deviceDisplayWidth/2 && appHeight > deviceDisplayHeight/2 && appWidth != deviceDisplayWidth {
		s.Fatal("Orientation size of an app: ", phoneSize)
	}

	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
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

	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

	// Relaunch the app.
	s.Log("Relaunching the app")
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to restart app: ", err)
	}
}

// DetectAndHandleCloseCrashOrAppNotResponding func to handle Crash or ANR.
func DetectAndHandleCloseCrashOrAppNotResponding(ctx context.Context, s *testing.State, d *ui.Device) {
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
		path := filepath.Join(s.OutDir(), "app-crash-or-anr.png")
		if err := screenshot.Capture(ctx, path); err != nil {
			s.Log("Screenshot for app-crash-or-anr.png: ", err)
		}
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

func restartApp(ctx context.Context, d *ui.Device, appPkgName string) error {
	const restartButtonResourceID = "android:id/button1"

	// Click on restart button.
	testing.ContextLog(ctx, "Attempting restart")
	restartButton := d.Object(ui.ResourceID(restartButtonResourceID))
	if err := restartButton.WaitForExists(ctx, LongUITimeout); err != nil {
		return errors.Wrap(err, "restart button does not exist")
	}
	if err := restartButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on restart button")
	}
	if _, err := d.WaitForWindowUpdate(ctx, appPkgName, LongUITimeout); err != nil {
		return errors.Wrap(err, "failed to wait for window to update")
	}
	return nil
}

// isNApp func to check if it is an N or pre-N app
func isNApp(ctx context.Context, d *ui.Device) bool {
	info, err := d.GetInfo(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get app sdk version: ", err)
		return false
	}
	testing.ContextLogf(ctx, "App sdk version %+v", info.SDKInt)
	return info.SDKInt >= 24
}

// currentAppPackage func to get info on current package name
func currentAppPackage(ctx context.Context, d *ui.Device) (string, error) {
	// Wait for app to launch.
	d.WaitForIdle(ctx, ShortUITimeout)
	info, err := d.GetInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.CurrentPackagename, nil
}

// processMonkeyOutput func parse the output logs of monkey test.
func processMonkeyOutput(output string) error {
	applicationNotRespondingErrorMsg := "Application is not responding:"
	anrErrorMessage := "ANR"
	monkeyTestAbortedErrorMessage := "Monkey aborted due to error."
	monkeyTestAbortedErrorMsg := "monkey aborted."
	NotRespondingErrorMessage := "NOT RESPONDING:"

	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, monkeyTestAbortedErrorMessage) ||
			strings.Contains(line, monkeyTestAbortedErrorMsg) ||
			strings.Contains(line, applicationNotRespondingErrorMsg) ||
			strings.Contains(line, anrErrorMessage) ||
			strings.Contains(line, NotRespondingErrorMessage) {
			return errors.New("monkey test aborted: " + line)
		}
	}
	return nil
}

// HandleDialogBoxes func will handle the dialog box
func HandleDialogBoxes(ctx context.Context, s *testing.State, d *ui.Device, appPkgName string) {
	const (
		allowText                   = "ALLOW"
		agreeText                   = "Agree"
		continueText                = "Continue"
		gotItText                   = "Got it"
		notNowText                  = "NOT NOW"
		okText                      = "OK"
		okayText                    = "OKAY"
		skipText                    = "Skip"
		whileUsingThisAppButtonText = "WHILE USING THE APP"
	)

	allowButton := d.Object(ui.TextMatches("(?i)" + allowText))
	appverifer := d.Object(ui.PackageName(appPkgName))
	agreeButton := d.Object(ui.TextMatches("(?i)" + agreeText))
	continueButton := d.Object(ui.TextMatches("(?i)" + continueText))
	gotItButton := d.Object(ui.TextMatches("(?i)" + gotItText))
	notNowButton := d.Object(ui.TextMatches("(?i)" + notNowText))
	okButton := d.Object(ui.TextMatches("(?i)" + okText))
	okayButton := d.Object(ui.TextMatches("(?i)" + okayText))
	skipButton := d.Object(ui.TextMatches("(?i)" + skipText))
	whileUsingThisAppButton := d.Object(ui.TextMatches("(?i)" + whileUsingThisAppButtonText))

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := allowButton.Exists(ctx); err == nil {
			s.Log("Click on allowButton")
			allowButton.Click(ctx)
		}
		if err := whileUsingThisAppButton.Exists(ctx); err == nil {
			s.Log("Click on whileUsingThisApp")
			whileUsingThisAppButton.Click(ctx)
		}
		if err := notNowButton.Exists(ctx); err == nil {
			s.Log("Click on notNowButton")
			notNowButton.Click(ctx)
		}
		if err := agreeButton.Exists(ctx); err == nil {
			s.Log("Click on agreeButton")
			agreeButton.Click(ctx)
		}
		if err := okButton.Exists(ctx); err == nil {
			s.Log("Click on okButton")
			okButton.Click(ctx)
		}
		if err := okayButton.Exists(ctx); err == nil {
			s.Log("Click on okayButton")
			okayButton.Click(ctx)
		}
		if err := skipButton.Exists(ctx); err == nil {
			s.Log("Click on skipButton")
			skipButton.Click(ctx)
		}
		if err := continueButton.Exists(ctx); err == nil {
			s.Log("Click on continueButton")
			continueButton.Click(ctx)
		}
		if err := gotItButton.Exists(ctx); err == nil {
			s.Log("Click on gotItButton")
			gotItButton.Click(ctx)
		}
		return appverifer.Exists(ctx)
	}, &testing.PollOptions{Timeout: LongUITimeout}); err != nil {
		s.Error("appPkgName doesn't exist: ", err)
	}
}

// getAppCoordinates func provides coordinates of the app.
func getAppCoordinates(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName string) (int, int, error) {
	var (
		xCoordinate int
		yCoordinate int
	)
	// To get app activities.
	out, err := a.Command(ctx, "am", "stack", "list").Output()
	if err != nil {
		s.Fatal("Failed to get stack list: ", err)
	}
	output := string(out)
	coordinatePrefix := "bounds="
	splitOutput := strings.Split(output, "\n")
	for splitLine := range splitOutput {
		if strings.Contains(splitOutput[splitLine], appPkgName) {
			splitCoordinate := strings.Split(splitOutput[splitLine], " ")
			for CoordinateInfo := range splitCoordinate {
				if strings.Contains(splitCoordinate[CoordinateInfo], coordinatePrefix) {
					s.Log("Coordinates: ", splitCoordinate[CoordinateInfo])
					x1coordinateWithOutTrim := strings.Split(splitCoordinate[CoordinateInfo], ",")[0]
					x1Coordinate := strings.Split(x1coordinateWithOutTrim, "[")[1]
					x1CoordinateValue, err := strconv.Atoi(x1Coordinate)
					if err == nil {
						s.Logf("x1CoordinateValue=%d", x1CoordinateValue)
					}

					y1coordinateWithOutTrim := strings.Split(splitCoordinate[CoordinateInfo], ",")[1]
					y1Coordinate := strings.Split(y1coordinateWithOutTrim, "]")[0]
					y1CoordinateValue, err := strconv.Atoi(y1Coordinate)
					if err == nil {
						s.Logf("y1CoordinateValue=%d", y1CoordinateValue)
					}

					x2coordinateWithOutTrim := strings.Split(splitCoordinate[CoordinateInfo], ",")[1]
					x2Coordinate := strings.Split(x2coordinateWithOutTrim, "[")[1]
					x2CoordinateValue, err := strconv.Atoi(x2Coordinate)
					if err == nil {
						s.Logf("x2CoordinateValue=%d", x2CoordinateValue)
					}

					y2coordinateWithOutTrim := strings.Split(splitCoordinate[CoordinateInfo], ",")[2]
					y2Coordinate := strings.Split(y2coordinateWithOutTrim, "]")[0]
					y2CoordinateValue, err := strconv.Atoi(y2Coordinate)
					if err == nil {
						s.Logf("y2CoordinateValue=%d", y2CoordinateValue)
					}
					xCoordinate = x2CoordinateValue - x1CoordinateValue
					yCoordinate = y2CoordinateValue - y1CoordinateValue
					break
				}
			}
		}
	}
	return xCoordinate, yCoordinate, err
}
