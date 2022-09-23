// Copyright 2021 The ChromiumOS Authors
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

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// Variables used by other tast tests
const (
	AndroidButtonClassName           = "android.widget.Button"
	notNowText                       = "Not now"
	asphaltPkgName                   = "com.gameloft.android.ANMP.GloftA9HM"
	homescapesPkgName                = "com.playrix.homescapes"
	skypePkgName                     = "com.skype.raider"
	toontasticPkgName                = "com.google.toontastic"
	playButtonText                   = "Play"
	clamshellLaunchTestForToontastic = "Launch app in Clamshell"
	tabletLaunchTestForToontastic    = "Launch app in Touchview"

	defaultTestCaseTimeout = 2 * time.Minute
	LaunchTestCaseTimeout  = 5 * time.Minute
	SignoutTestCaseTimeout = 3 * time.Minute
	windowTestCaseTimeout  = 3 * time.Minute
	DefaultUITimeout       = 20 * time.Second
	ShortUITimeout         = 30 * time.Second
	LongUITimeout          = 90 * time.Second
)

// TestFunc represents the "test" function.
type TestFunc func(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string)

// TestCase represents the name of test, the function to call.
type TestCase struct {
	Name    string
	Fn      TestFunc
	Timeout time.Duration
}

// TestParams represents the collection of tests to run in tablet mode or clamshell mode.
type TestParams struct {
	LaunchTests      []TestCase
	CommonTests      []TestCase
	ReleaseTests     []TestCase
	TopAppTests      []TestCase
	AppSpecificTests []TestCase
}

// ClamshellCommonTests is a list of all tests common to all apps in clamshell mode.
var ClamshellCommonTests = []TestCase{
	// TODO(b/166637700): Remove the commented testcases if the proper solution is found for the issue.
	//{Name: "Clamshell: Orientation", Fn: OrientationSize},
	{Name: "Clamshell: Touchscreen Scroll", Fn: TouchScreenScroll},
	//{Name: "Clamshell: Mouse click", Fn: MouseClick},
	//{Name: "Clamshell: Mouse Scroll", Fn: MouseScrollAction},
	//{Name: "Clamshell: Physical Keyboard", Fn: TouchAndTextInputs},
	//{Name: "Clamshell: Keyboard Critical Path", Fn: KeyboardNavigations},
	//{Name: "Clamshell: Special keys: ESC key", Fn: EscKey},
	// Commented Clamshell: Largescreen Layout testcase since it is handled by Resize lock feature in ARC-VM.
	//{Name: "Clamshell: Largescreen Layout", Fn: Largescreenlayout},
	{Name: "Clamshell: Fullscreen app", Fn: ClamshellFullscreenApp},
	{Name: "Clamshell: Minimise and Restore", Fn: MinimizeRestoreApp},
	//{Name: "Clamshell: Resize window", Fn: ClamshellResizeWindow},
	{Name: "Clamshell: Reopen app", Fn: ReOpenWindow},
	{Name: "Clamshell: Resize Lock", Fn: ResizeLock},
}

// TouchviewCommonTests is a list of all tests common to all apps in touchview mode.
var TouchviewCommonTests = []TestCase{
	// TODO(b/166637700): Remove the commented testcases if the proper solution is found for the issue.
	{Name: "Touchview: Rotate", Fn: TouchviewRotate},
	//{Name: "Touchview: Splitscreen", Fn: SplitScreen},
	//{Name: "Touchview: Touchscreen Scroll", Fn: TouchScreenScroll},
	//{Name: "Touchview: Virtual Keyboard", Fn: TouchAndTextInputs},
	//{Name: "Touchview: Largescreen Layout", Fn: Largescreenlayout},
	{Name: "Touchview: Minimise and Restore", Fn: MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: ReOpenWindow},
}

// ClamshellReleaseTests is a list of clamshell tests common to apps in appcompat_release suite.
var ClamshellReleaseTests = []TestCase{
	{Name: "Clamshell: Fullscreen app", Fn: ClamshellFullscreenApp, Timeout: windowTestCaseTimeout},
	{Name: "Clamshell: Minimise and Restore", Fn: MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: ClamshellResizeWindow, Timeout: windowTestCaseTimeout},
	{Name: "Clamshell: Reopen app", Fn: ReOpenWindow},
}

// TouchviewReleaseTests is a list of touchview tests common to apps in appcompat_release suite.
var TouchviewReleaseTests = []TestCase{
	{Name: "Touchview: Minimise and Restore", Fn: MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: ReOpenWindow},
}

// ClamshellTopAppTests is a list of clamshell tests common to apps in appcompat_top_apps suite.
var ClamshellTopAppTests = []TestCase{
	{Name: "Clamshell: Fullscreen app", Fn: ClamshellFullscreenApp, Timeout: windowTestCaseTimeout},
	{Name: "Clamshell: Minimise and Restore", Fn: MinimizeRestoreApp},
	{Name: "Clamshell: Resize window", Fn: ClamshellResizeWindow, Timeout: windowTestCaseTimeout},
	{Name: "Clamshell: Reopen app", Fn: ReOpenWindow},
}

// TouchviewTopAppTests is a list of touchview tests common to apps in appcompat_top_apps suite.
var TouchviewTopAppTests = []TestCase{
	{Name: "Touchview: Rotate", Fn: TouchviewRotate},
	{Name: "Touchview: Minimise and Restore", Fn: MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: ReOpenWindow},
}

// ClamshellSmokeTests is a list of clamshell tests common to apps in appcompat_smoke suite.
var ClamshellSmokeTests = []TestCase{
	{Name: "Clamshell: Touchscreen Scroll", Fn: TouchScreenScroll},
	{Name: "Clamshell: Physical Keyboard", Fn: TouchAndTextInputs},
	{Name: "Clamshell: Mouse click", Fn: MouseClick},
	{Name: "Clamshell: Resize window", Fn: ClamshellResizeWindow, Timeout: windowTestCaseTimeout},
}

// TouchviewSmokeTests is a list of touchview tests common to apps in appcompat_smoke suite.
var TouchviewSmokeTests = []TestCase{
	{Name: "Touchview: Minimise and Restore", Fn: MinimizeRestoreApp},
	{Name: "Touchview: Reopen app", Fn: ReOpenWindow},
}

// RunTestCases setups the device and runs all app compat test cases.
func RunTestCases(ctx context.Context, s *testing.State, appPkgName, appActivity string, testCases TestParams) {
	// Step up chrome on Chromebook.
	cr, tconn, a, appVer := setUpDevice(ctx, s, appPkgName, appActivity)

	updatedAppActivity := appActivity
	if appPkgName == skypePkgName {
		if strings.Compare(appVer, "8.80.0.137") >= 0 {
			updatedAppActivity = "com.skype4life.MainActivity"
		}
	}
	s.Log("Updated app activity: ", updatedAppActivity)
	// Ensure app launches before test cases.
	act, err := arc.NewActivity(a, appPkgName, updatedAppActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()

	// Keep the device awake throughout all tests.
	resetKeepAwake, err := power.RequestKeepAwake(ctx, tconn, power.Display)
	if err != nil {
		s.Fatal("Failed to request the device to keep awake: ", err)
	}
	defer resetKeepAwake(ctx, tconn)
	if appPkgName != toontasticPkgName { // Skip StartWithDefaultOptions for Toontastic app only.
		if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
			s.Fatal("Failed to start app before test cases: ", err)
		}

		if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to stop app before test cases: ", err)
		}
		s.Log("Successfully tested launching and closing the app")
	}

	// AllTests will have LaunchTests, CommonTests, ReleaseTests, TopAppTests and AppSpecificTests.
	var AllTests = []TestCase{}
	for _, curTest := range testCases.LaunchTests {
		AllTests = append(AllTests, curTest)
	}
	for _, curTest := range testCases.CommonTests {
		AllTests = append(AllTests, curTest)
	}
	for _, curTest := range testCases.ReleaseTests {
		AllTests = append(AllTests, curTest)
	}
	for _, curTest := range testCases.TopAppTests {
		AllTests = append(AllTests, curTest)
	}
	for _, curTest := range testCases.AppSpecificTests {
		AllTests = append(AllTests, curTest)
	}

	// Run the different test cases.
	for idx, test := range AllTests {
		// Limit the launch test case, signout test case to a specified timeout.
		// This makes sure that one test case doesn't use all of the time when it fails.
		timeout := test.Timeout
		// Use the default if the timeout wasn't set.
		if timeout == time.Duration(0) {
			timeout = defaultTestCaseTimeout
		}
		s.Logf("Timeout for %s test case: %s", test.Name, timeout)

		testCaseCtx, cancel := ctxutil.Shorten(ctx, timeout)
		defer cancel()

		s.Run(testCaseCtx, test.Name, func(cleanupCtx context.Context, s *testing.State) {

			// Save time for cleanup and screenshot.
			ctx, cancel := ctxutil.Shorten(cleanupCtx, 20*time.Second)
			defer cancel()

			if appPkgName == toontasticPkgName { // Skip StartWithDefaultOptions if app is Toontastic and if test cases are clamshell / tablet launch test cases.
				if test.Name != clamshellLaunchTestForToontastic && test.Name != tabletLaunchTestForToontastic {
					if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
						s.Fatal("Failed to start app: ", err)
					}
				}
			} else if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			s.Log("App launched successfully")

			d, err := a.NewUIDevice(ctx)
			if err != nil {
				s.Fatal("Failed initializing UI Automator: ", err)
			}
			defer d.Close(ctx)

			// Close the app between iterations.
			defer func(ctx context.Context) {
				if appPkgName == asphaltPkgName {
					HandleDialogBoxes(ctx, s, d, appPkgName)
				}
				if appPkgName == toontasticPkgName {
					if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
						s.Log("Failed to close Play Store: ", err)
					}
				}
				if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
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

			DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

			// Click on not now button to skip the display profile visible in public.
			notNowButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextMatches("(?i)"+notNowText))
			if err := notNowButton.WaitForExists(ctx, DefaultUITimeout); err == nil {
				if err := notNowButton.Click(ctx); err != nil {
					s.Fatal("Failed to click on notNow Button: ", err)
				}
			}

			// It is ok if the package is currently equal the installer package.
			// It is also ok if the package is currently equal the play service package.
			// It is also ok if the package is currently equal the android permission controller package
			// It is also ok if the package is currently equal the settings package.
			// This happens when you need to accept permissions.
			var allowedAppPackage bool
			currentAppPkg, err := CurrentAppPackage(ctx, d, s)
			if err != nil {
				s.Fatal("Failed to get current app package: ", err)
			}
			if currentAppPkg == appPkgName {
				allowedAppPackage = true
			} else if currentAppPkg != appPkgName {
				for _, perAppPkg := range []struct {
					permissionAppPkgNames string
				}{
					{"com.google.android.packageinstaller"}, {"com.google.android.gms"},
					{"com.google.android.permissioncontroller"}, {"com.android.settings"},
				} {
					if currentAppPkg == perAppPkg.permissionAppPkgNames {
						allowedAppPackage = true
						break
					}
				}
			}
			if !allowedAppPackage {
				s.Fatalf("Failed to launch app: incorrect package(expected: %s, actual: %s)", appPkgName, currentAppPkg)
			}
			test.Fn(ctx, s, tconn, a, d, appPkgName, updatedAppActivity)
		})
		cancel()
	}
}

// setUpDevice func setup Chrome on Chromebook.
func setUpDevice(ctx context.Context, s *testing.State, appPkgName, appActivity string) (*chrome.Chrome, *chrome.TestConn, *arc.ARC, string) {

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

	// Keep the device awake while installing from the play store.
	resetKeepAwake, err := power.RequestKeepAwake(ctx, tconn, power.Display)
	if err != nil {
		s.Fatal("Failed to request the device to keep awake: ", err)
	}
	defer resetKeepAwake(ctx, tconn)

	if err := apps.Launch(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}
	if err := playstore.InstallApp(ctx, a, d, appPkgName, &playstore.Options{}); err != nil {
		s.Fatal("Failed to install app: ", err)
	}
	versionName, err := GetAppVersion(ctx, s, a, d, appPkgName)
	if appPkgName == toontasticPkgName {
		// Click on play button to launch Toontastic app
		playButton := d.Object(ui.ClassName(AndroidButtonClassName), ui.TextMatches("(?i)"+playButtonText))
		if err := playButton.WaitForExists(ctx, ShortUITimeout); err != nil {
			s.Fatal("playbutton doesn't exist: ", err)
		} else if err := playButton.Click(ctx); err != nil {
			s.Fatal("Failed to click on play button: ", err)
		}
	} else if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}
	return cr, tconn, a, versionName
}

// ClamshellFullscreenApp Test launches the app in full screen window and verifies launch successfully without crash or ANR on ARC-P devices
func ClamshellFullscreenApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	info, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName)
	if err != nil {
		s.Error("Failed to get window info: ", err)
	}
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}
	// If ARC-P.
	if t == arc.Container {
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
	}
	if t == arc.VM {
		// If app is launched in maximized state or
		// If app doesn't have resize lock feature in normal window mode then set the window to full screen.
		uia := uiauto.New(tconn)
		button := nodewith.HasClass(centerButtonClassName)

		err := uia.WithTimeout(10 * time.Second).WaitUntilExists(button)(ctx)
		if info.State == ash.WindowStateMaximized || err != nil {
			s.Log("app is in maximized mode or it can be an O4C app. Setting the window to fullscreen")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
				s.Error("Failed to set the window to fullscreen: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
				s.Error("The window is not in fullscreen: ", err)
			}
			return
		}

		// If app has resize lock feature and it is in phone or tablet size.
		nodes, err := uia.WithTimeout(10*time.Second).NodesInfo(ctx, button)
		if err != nil {
			s.Error("Failed to get button node info: ", err)
		}
		if len(nodes) >= 1 {
			info := nodes[0]
			defaultState := info.Name
			s.Logf("Default state of app is in: %+v", defaultState)
			if info.Name == phoneButtonName || info.Name == tabletButtonName {
				// CloseSplash to handle got it button.
				if err := closeSplash(ctx, tconn); err == nil {
					s.Log("CloseSplash does exist")
				}

				// Check if the compat-mode button of a fully-locked app is disabled
				if err := uia.LeftClick(button)(ctx); err != nil {
					s.Fatal(err, "failed to click on the compat-mode button: ", err)
				}
				// Need some sleep here as we verify that nothing changes.
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
				}
				// Check if compat-mode button is disabled or enabled.
				if err := checkVisibility(ctx, tconn, bubbleDialogClassName, false); err == nil {
					// If compat-mode button is disabled.
					s.Log("The app is non-resizable. Skipping test")
					return
				}
			}
			// If compat-mode button is enabled.
			if err := selectResizeLockMode(ctx, tconn, appPkgName); err != nil {
				s.Fatal("Failed to click on the compat-mode dialog: ", err)
			}
			if err := handleConfirmDialog(ctx, tconn); err == nil {
				s.Log("confirmDialog does exist")
			}
			s.Log("Setting the window to fullscreen")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
				s.Error("Failed to set the window to fullscreen: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
				s.Error("The window is not in fullscreen: ", err)
			}
			s.Log("Reseting window to normal size")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
				s.Error("Failed to reset window to normal size: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
				s.Error("The window is not normalized: ", err)
			}
			// Restore the window to the default window state of an app.
			selectDefaultWindowState(ctx, s, tconn, appPkgName, defaultState)
		}
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// MinimizeRestoreApp Test "minimize and relaunch the app" and verifies app relaunch successfully without crash or ANR.
func MinimizeRestoreApp(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	s.Log("Minimizing the window")
	defaultState, err := ash.GetARCAppWindowState(ctx, tconn, appPkgName)
	if err != nil {
		s.Fatal("Failed to get the default window state: ", err)
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

// ClamshellResizeWindow Test "resize and restore back to original state of the app" and verifies app launch successfully without crash or ANR on ARC-P devices.
func ClamshellResizeWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	info, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}
	goalState := ash.WindowStateMaximized
	if info.State == ash.WindowStateFullscreen {
		goalState = ash.WindowStateFullscreen
	}
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		s.Log("Device is in tablet mode. Skipping test")
		return
	}
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}
	// If ARC-P.
	if t == arc.Container {
		info, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName)
		if err != nil {
			s.Fatal("Failed to get window info: ", err)
		}
		s.Logf("App Resize info, info.CanResize %+v", info.CanResize)
		if !info.CanResize {
			s.Log("This app is not resizable. Skipping test")
			return
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
	}
	// If ARC-VM.
	// Handle resize lock feature.
	if t == arc.VM {
		// If app is launched in maximized or in fullscreen state.
		if info.State == ash.WindowStateMaximized || info.State == ash.WindowStateFullscreen {
			// Check if app is resizable or not.
			s.Logf("App Resize info, info.CanResize %+v", info.CanResize)
			if !info.CanResize {
				s.Log("This app is not resizable. Skipping test")
				return
			}
			s.Log("Reseting window to normal size")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
				s.Error("Failed to reset window to normal size: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
				s.Error("The window is not normalized: ", err)
			}
		}

		// If app doesn't have resize lock feature in normal window mode then maximize the app.
		uia := uiauto.New(tconn)
		centerButton := nodewith.HasClass(centerButtonClassName)
		if err := uia.WithTimeout(10 * time.Second).Exists(centerButton)(ctx); err != nil {
			s.Log("It can be an O4C app. App window is normal. Maximize the window")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventTypeForState(goalState)); err != nil {
				s.Error("Failed to maximize the window: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, goalState); err != nil {
				s.Error("The window is not maximized: ", err)
			}
			return
		}
		// If app has resize lock feature and it is in phone or tablet size.
		nodes, err := uia.NodesInfo(ctx, centerButton)
		if err != nil {
			s.Error("Failed to get node info for center button: ", err)
			return
		}
		if len(nodes) >= 1 {
			buttonInfo := nodes[0]
			defaultState := buttonInfo.Name
			s.Logf("Default state of app is in: %+v", defaultState)
			if buttonInfo.Name == phoneButtonName || buttonInfo.Name == tabletButtonName {
				s.Log("App is in: ", buttonInfo.Name)
				// CloseSplash to handle got it button.
				if err := closeSplash(ctx, tconn); err == nil {
					s.Log("CloseSplash does exist")
				}

				// Check if the compat-mode button of a fully-locked app is disabled.
				if err := uia.LeftClick(centerButton)(ctx); err != nil {
					s.Fatal("Failed to click on the compat-mode button: ", err)
				}
				// Need some sleep here as we verify that nothing changes.
				if err := testing.Sleep(ctx, time.Second); err != nil {
					s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
				}
				// Check if compat-mode button is disabled or enabled.
				if err := checkVisibility(ctx, tconn, bubbleDialogClassName, false); err == nil {
					// If compat-mode button is disabled.
					s.Log("The app is non-resizable. Skipping test")
					return
				}
			}
			s.Logf("State of app is in: %+v", buttonInfo.Name)
			if buttonInfo.Name != resizableButtonName {
				// If compat-mode button is enabled.
				if err := selectResizeLockMode(ctx, tconn, appPkgName); err != nil {
					s.Fatal("Failed to click on the compat-mode dialog: ", err)
				}
				if err := handleConfirmDialog(ctx, tconn); err != nil {
					s.Log("confirmDialog doesn't exist: ", err)
				}
			}
			s.Log("Maximizing the window")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
				s.Error("Failed to maximize the window: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
				s.Error("The window is not maximized: ", err)
			}
			s.Log("Reseting window to normal size")
			if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
				s.Error("Failed to reset window to normal size: ", err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
				s.Error("The window is not normalized: ", err)
			}
			// Restore the window to the default window state of an app.
			selectDefaultWindowState(ctx, s, tconn, appPkgName, defaultState)
		}
	}

	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// Represents the high-level state of the app from the resize-lock feature's perspective.
type resizeLockMode int

const (
	phoneResizeLockMode resizeLockMode = iota
	tabletResizeLockMode
	resizableResizeLockMode
)

func (mode resizeLockMode) String() string {
	switch mode {
	case phoneResizeLockMode:
		return phoneButtonName
	case tabletResizeLockMode:
		return tabletButtonName
	case resizableResizeLockMode:
		return resizableButtonName
	default:
		return ""
	}
}

const (
	// Used to (i) find the resize lock mode buttons on the compat-mode menu and (ii) check the state of the compat-mode button.
	phoneButtonName     = "Phone"
	tabletButtonName    = "Tablet"
	resizableButtonName = "Resizable"

	// Currently the automation API doesn't support unique ID, so use the classnames to find the elements of interest.
	centerButtonClassName  = "FrameCenterButton"
	checkBoxClassName      = "Checkbox"
	bubbleDialogClassName  = "BubbleDialogDelegateView"
	overlayDialogClassName = "OverlayDialog"
	menuItemViewClassName  = "MenuItemView"

	// A11y names are available for some UI elements.
	splashCloseButtonName = "Got it"
	confirmButtonName     = "Allow"
	cancelButtonName      = "Cancel"
	closeMenuItemViewName = "Close"
)

// ResizeLock Test verifies if app has resize lock feature available or not and verifies if app launch successfully without crash or ANR.
func ResizeLock(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}
	// If ARC-VM.
	if t == arc.VM {
		if err := checkCompatModeButton(ctx, s, tconn, a, d, appPkgName, appActivity); err != nil {
			s.Fatal("Failed to verify the window state and compat mode button availability: ", err)
		}
		DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
	}
}

// checkCompatModeButton verifies the window state of app and also verifies compat mode button is available or not for the given app.
func checkCompatModeButton(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) error {
	info, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}
	// Check if app is launched in maximized or in fullscreen state.
	if info.State == ash.WindowStateMaximized || info.State == ash.WindowStateFullscreen {
		s.Log("App is in maximized or full screen state")
		// Check if app is resizable or not.
		s.Logf("App Resize info, info.CanResize %+v", info.CanResize)
		if !info.CanResize {
			s.Log("This app is not resizable. Skipping test")
			return nil
		}
		// If resizable, on clicking resize button, check app shows up resize lock feature or not.
		s.Log("N-apps start maximized. Reseting window to normal size")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventNormal); err != nil {
			s.Error("Failed to reset window to normal size: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateNormal); err != nil {
			s.Error("The window is not normalized: ", err)
		}
	}
	// Check for compat-mode button.
	uia := uiauto.New(tconn)
	nodes, err := uia.WithTimeout(10*time.Second).NodesInfo(ctx, nodewith.HasClass(centerButtonClassName))
	if err != nil {
		return errors.Wrap(err, "failed to get node info for center button")
	}
	if len(nodes) == 0 {
		s.Log("App isn't resize locked app. It can be an O4C app")
	} else {
		button := nodes[0]
		if button.Name == phoneButtonName || button.Name == tabletButtonName || button.Name == resizableButtonName {
			s.Log("It is resize locked app")
			s.Logf("button.Name:%s", button.Name)
		} else {
			return errors.Wrap(err, "failed to find the compat mode options")
		}
	}
	return nil
}

// checkVisibility checks whether the node specified by the given class name exists or not.
func checkVisibility(ctx context.Context, tconn *chrome.TestConn, className string, visible bool) error {
	uia := uiauto.New(tconn)
	finder := nodewith.HasClass(className).First()
	if visible {
		return uia.WithTimeout(10 * time.Second).WaitUntilExists(finder)(ctx)
	}
	return uia.WithTimeout(10 * time.Second).WaitUntilGone(finder)(ctx)
}

// selectResizeLockMode clicks on the resizable lock mode button and clicks on the confirm button.
func selectResizeLockMode(ctx context.Context, tconn *chrome.TestConn, appPkgName string) error {
	uia := uiauto.New(tconn)
	compatModeMenuDialog := nodewith.Role(role.Window).HasClass(bubbleDialogClassName)
	resizeLockModeButton := nodewith.Role(role.MenuItem).Name(resizableButtonName).Ancestor(compatModeMenuDialog)
	if err := uia.WithTimeout(10 * time.Second).WaitUntilExists(compatModeMenuDialog)(ctx); err != nil {
		return errors.Wrapf(err, "failed to find the compat-mode menu dialog of %s", appPkgName)
	}
	if err := uia.WithTimeout(10 * time.Second).WaitUntilExists(resizeLockModeButton)(ctx); err != nil {
		return errors.Wrapf(err, "failed to find the %s button on the compat mode menu", resizableButtonName)
	}

	return uia.LeftClick(resizeLockModeButton)(ctx)
}

// selectDefaultWindowState clicks on the default window type which can be phone or tablet button and clicks on the confirm button.
func selectDefaultWindowState(ctx context.Context, s *testing.State, tconn *chrome.TestConn, appPkgName, defaultState string) error {
	uia := uiauto.New(tconn)
	button := nodewith.HasClass(centerButtonClassName)
	nodes, err := uia.WithTimeout(10*time.Second).NodesInfo(ctx, button)
	if err == nil && len(nodes) >= 1 {
		info := nodes[0]
		// If app is in resizable mode.
		if info.Name == resizableButtonName {
			s.Log("App is in: ", info.Name)
			// CloseSplash to handle got it button.
			if err := closeSplash(ctx, tconn); err == nil {
				s.Log("CloseSplash does exist")
			}

			// Check if the compat-mode button of a fully-locked app is disabled
			if err := uia.LeftClick(button)(ctx); err != nil {
				s.Fatal(err, "failed to click on the compat-mode button: ", err)
			}
			// Need some sleep here as we verify that nothing changes.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep after clicking on the compat-mode button: ", err)
			}
			// Check if compat-mode button is disabled or enabled.
			if err := checkVisibility(ctx, tconn, bubbleDialogClassName, false); err == nil {
				// If compat-mode button is disabled.
				s.Log("The app is non-resizable. Skipping test")
				return nil
			}
		}
	}
	compatModeMenuDialog := nodewith.Role(role.Window).HasClass(bubbleDialogClassName)
	if err := uia.WithTimeout(10 * time.Second).WaitUntilExists(compatModeMenuDialog)(ctx); err != nil {
		s.Fatal(err, "failed to find the compat-mode menu dialog of %s", appPkgName)
	}
	// To get back to the default window state of an app.
	defaultWindow := resizableButtonName
	if defaultState == "Phone" {
		s.Logf("Get back to default window state of an app: %+v", defaultState)
		defaultWindow = phoneButtonName
	}
	if defaultState == "Tablet" {
		s.Logf("Get back to default window state of an app: %+v", defaultState)
		defaultWindow = tabletButtonName
	}
	resizeLockModeButton := nodewith.Role(role.MenuItem).Name(defaultWindow).Ancestor(compatModeMenuDialog)
	if err := uia.WithTimeout(10 * time.Second).WaitUntilExists(resizeLockModeButton)(ctx); err != nil {
		s.Fatal(err, "failed to find the %s button on the compat mode menu", defaultWindow)
	}
	return uia.LeftClick(resizeLockModeButton)(ctx)
}

// handleConfirmDialog clicks on allow button for the confirmation dialog.
func handleConfirmDialog(ctx context.Context, tconn *chrome.TestConn) error {
	uia := uiauto.New(tconn)
	confirmationDialog := nodewith.HasClass(overlayDialogClassName)
	if err := uia.WithTimeout(10 * time.Second).WaitUntilExists(confirmationDialog)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the resizability confirmation dialog")
	}
	confirmButton := nodewith.Role(role.Button).Name(confirmButtonName).Ancestor(confirmationDialog)
	if err := uia.WithTimeout(10 * time.Second).WaitUntilExists(confirmButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the confirm button on the compat mode menu")
	}
	return uia.LeftClick(confirmButton)(ctx)
}

// closeSplash clicks on the close button and closes the splash screen.
func closeSplash(ctx context.Context, tconn *chrome.TestConn) error {
	splash := nodewith.HasClass(bubbleDialogClassName)
	button := nodewith.Ancestor(splash).HasClass(splashCloseButtonName)
	return uiauto.New(tconn).WithTimeout(10*time.Second).LeftClickUntil(button, func(ctx context.Context) error {
		return checkVisibility(ctx, tconn, bubbleDialogClassName, false /* visible */)
	})(ctx)
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
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--pct-touch", "30", "--pct-nav", "10", "--pct-touch", "40", "--pct-nav", "10", "--pct-anyevent", "2", "--throttle", "100", "-v", "50").Output(testexec.DumpLogOnError)
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
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--pct-nav", "20", "--pct-majornav", "20", "--pct-nav", "20", "--pct-majornav", "20", "--throttle", "100", "-v", "50").Output(testexec.DumpLogOnError)
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
	// Press enter key twice.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Log("Failed to enter KEYCODE_ENTER: ", err)
	}
	// To perform touch and play video.
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--pct-touch", "60", "--throttle", "100", "-v", "50").Output(testexec.DumpLogOnError)
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
	checkForScrollLayout := d.Object(ui.PackageName(appPkgName), ui.Scrollable(true), ui.Focusable(true), ui.Enabled(true))
	if err := checkForScrollLayout.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("ScrollLayout doesn't exist. Page is not scrollable and skipping the test: ", err)
		return
	}
	// To perform mouse scroll actions.
	out, err := a.Command(ctx, "monkey", "--pct-syskeys", "0", "-p", appPkgName, "--pct-trackball", "100", "--throttle", "100", "-v", "50").Output(testexec.DumpLogOnError)
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
	checkForScrollLayout := d.Object(ui.PackageName(appPkgName), ui.Scrollable(true), ui.Focusable(true), ui.Enabled(true))
	if err := checkForScrollLayout.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("ScrollLayout doesn't exist. Page is not scrollable and skipping the test: ", err)
		return
	}

	scrollForwdInfo, err := checkForScrollLayout.ScrollForward(ctx, 50)
	if err != nil {
		s.Fatal("Failed to scroll forward: ", err)
	}
	if !scrollForwdInfo {
		s.Log("App page can not be scrolled forward anymore")
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
	checkUIElement := d.Object(ui.PackageName(appPkgName), ui.Clickable(true), ui.Focusable(true), ui.Enabled(true))
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
		blackBars     = "Black bars observed on both sides of an app"
		maximizedSize = "Maximized"
		phoneSize     = "Phone"
		tabletSize    = "Tablet"
	)
	t, ok := arc.Type()
	if !ok {
		s.Fatal("Unable to determine arc type")
	}
	// If ARC-VM.
	if t == arc.VM {
		// TODO(b/188816051): Remove ash.TabletModeEnabled(ctx, tconn) if a solution is found for identifying the device in clamshell/ laptop mode using hardware/software dependency.
		tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get tablet mode: ", err)
		}
		if tabletModeEnabled {
			s.Log("Device is in tablet mode. Skipping test")
			return
		}

		appWidth, appHeight, err := getAppCoordinates(ctx, s, a, d, appPkgName)
		if err != nil {
			s.Fatal("Failed to get app coordinates: ", err)
		}

		info, err := d.GetInfo(ctx)
		if err != nil {
			s.Fatal("Failed to get device display: ", err)
		}
		deviceDisplayWidth := info.DisplayWidth
		deviceDisplayHeight := info.DisplayHeight

		windowInfo, err := getAppWindowInfo(ctx, s, a, d, appPkgName)

		switch windowInfo {
		case "freeform":
			if appWidth == deviceDisplayWidth {
				testing.ContextLogf(ctx, "Orientation size of an app is %+v and its appWidth %+v appHeight %+v deviceDisplayWidth %+v deviceDisplayHeight %+v ", maximizedSize, appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
			} else if appWidth > deviceDisplayWidth/2 && appWidth <= deviceDisplayWidth*3/4 && appWidth != deviceDisplayWidth {
				testing.ContextLogf(ctx, "Orientation size of an app is %+v and its appWidth %+v appHeight %+v deviceDisplayWidth %+v deviceDisplayHeight %+v ", tabletSize, appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
			} else if appWidth >= deviceDisplayWidth*3/4 && appWidth != deviceDisplayWidth {
				testing.ContextLogf(ctx, "Orientation size of an app: %v and its appWidth %+v appHeight %+v deviceDisplayWidth %+v deviceDisplayHeight %+v", tabletSize, appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
			} else {
				testing.ContextLogf(ctx, "Orientation size of an app is %+v and its appWidth %+v appHeight %+v deviceDisplayWidth %+v deviceDisplayHeight %+v ", phoneSize, appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
			}
		case "fullscreen":
			if appWidth == deviceDisplayWidth {
				testing.ContextLogf(ctx, "Orientation size of an app: %v and its appWidth %+v appHeight %+v deviceDisplayWidth %+v deviceDisplayHeight %+v", maximizedSize, appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
			} else if appWidth < deviceDisplayWidth && appWidth != deviceDisplayWidth {
				testing.ContextLogf(ctx, "appWidth %+v appHeight %+v deviceDisplayWidth %+v deviceDisplayHeight %+v", appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
				s.Fatal("Failed to utilize the screen and app is in ", maximizedSize+" size with "+blackBars)
			}
		}
		DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
	}
}

// Largescreenlayout Test verifies if app utilizes large screen after maximizing the app and without crash or ANR.
func Largescreenlayout(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if !tabletModeEnabled {
		s.Log("Setting the window to fullscreen")
		if _, err := ash.SetARCAppWindowState(ctx, tconn, appPkgName, ash.WMEventFullscreen); err != nil {
			s.Fatal("Failed to set the window to fullscreen: ", err)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateFullscreen); err != nil {
			s.Fatal("The window is not in fullscreen: ", err)
		}
	}

	appWidth, appHeight, err := getAppCoordinates(ctx, s, a, d, appPkgName)
	if err != nil {
		s.Fatal("Failed to get app coordinates: ", err)
	}

	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get device display: ", err)
	}
	deviceDisplayWidth := info.DisplayWidth
	deviceDisplayHeight := info.DisplayHeight

	if appWidth >= deviceDisplayWidth {
		testing.ContextLogf(ctx, "App utilizes the device display screen and its appWidth %+v appHeight %+v  deviceDisplayWidth %+v deviceDisplayHeight %+v", appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
	} else {
		testing.ContextLogf(ctx, "appWidth %+v appHeight %+v  deviceDisplayWidth %+v deviceDisplayHeight %+v", appWidth, appHeight, deviceDisplayWidth, deviceDisplayHeight)
		s.Fatal("Failed to utilize the device display screen and black bars observed on both side of an app")
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
	if err := a.Command(ctx, "am", "force-stop", appPkgName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to stop app before test cases: ", err)
	}

	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)

	// Relaunch the app.
	s.Log("Relaunching the app")
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to restart app: ", err)
	}
}

// EscKey Test verifies if app doesn't quit on pressing esc key and without crash or ANR.
func EscKey(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ESCAPE, 0); err != nil {
		s.Fatal("Failed to press KEYCODE_ESCAPE: ", err)
	}

	if currentAppPkg, err := CurrentAppPackage(ctx, d, s); err != nil {
		s.Fatal("Failed to get current app package: ", err)
	} else if currentAppPkg != appPkgName && currentAppPkg != "com.google.android.packageinstaller" && currentAppPkg != "com.google.android.gms" && currentAppPkg != "com.google.android.permissioncontroller" {
		s.Fatalf("App quits on pressing esc key: package(expected: %s, actual: %s)", appPkgName, currentAppPkg)
	}
}

// SplitScreen Test verifies if app supports split screen and check if app performs split screen without crash or ANR.
func SplitScreen(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	displayInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}
	s.Logf("displayInfo:%+v", displayInfo.HasTouchSupport)
	if !displayInfo.HasTouchSupport {
		s.Log("Device doesn't support touchscreen. Skipping test")
		return
	}

	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, appPkgName)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}
	s.Logf("App Resize info, info.CanResize %+v", windowInfo.CanResize)
	if !windowInfo.CanResize {
		s.Log("App doesn't support split screen. Skipping test")
		return
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	s.Logf("Orientation of primary window, orientation.Type %+v", orientation.Type)
	//TODO(b/178401320): Remove this if a proper solution is found to perform split screen on portrait oriented apps.
	if orientation.Type == display.OrientationPortraitPrimary || orientation.Type == display.OrientationPortraitSecondary {
		s.Log("App is in portrait orientation. Skipping test")
		return
	}

	tew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to access to the touch screen: ", err)
	}
	defer tew.Close()

	// Ensure device display is in landscape orientation so that app window snap on
	// the left and right.
	orientation, err = display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	s.Logf("orientation.Angle %+v", orientation.Angle)

	rotation := -orientation.Angle
	tew.SetRotation(rotation)

	stw, err := tew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create a single touch writer: ", err)
	}
	defer stw.Close()

	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enter overview: ", err)
	}
	const target = 0
	if err := dragToSnapFirstOverviewWindow(ctx, s, tconn, tew, stw, target); err != nil {
		s.Fatal("Failed to drag window from overview and snap left: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, appPkgName, ash.WindowStateLeftSnapped); err != nil {
		s.Fatal("Failed to wait until window state change to left: ", err)
	}
	DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
}

// StylusClick func verifies if stylus click works properly in the app without crash or ANR.
func StylusClick(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string) {
	var (
		xCoordinate int
		yCoordinate int
	)
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}
	if !info.HasTouchSupport {
		s.Log("Device does not have touch support. Skipping the test")
		return
	}
	checkUIElement := d.Object(ui.Clickable(true), ui.Focusable(true), ui.Enabled(true))
	if err := checkUIElement.WaitForExists(ctx, DefaultUITimeout); err != nil {
		s.Log("checkUIElement doesn't exist and skipped stylus click: ", err)
		return
	}
	s.Log("checkUIElement does exists")
	if uiElementBounds, err := checkUIElement.GetBounds(ctx); err != nil {
		s.Log("Failed to get uiElementBounds and skipped stylus click : ", err)
	} else {
		s.Log("uiElementBounds: ", uiElementBounds)
		xCoordinate = uiElementBounds.Left
		s.Log("Xcoordinate: ", xCoordinate)
		yCoordinate = uiElementBounds.Top
		s.Log("Ycoordinate: ", yCoordinate)

		// TODO (b/188840879): Remove a.Command(ctx, "input", "stylus", "tap", XCoordinate, YCoordinate) if proper solution is found for it.
		// To perform stylus click.
		out, err := a.Command(ctx, "input", "stylus", "tap", strconv.Itoa(xCoordinate), strconv.Itoa(yCoordinate)).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to perform stylus click: ", err)
		} else {
			s.Log("Performed stylus click: ", string(out))
		}
		DetectAndHandleCloseCrashOrAppNotResponding(ctx, s, d)
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

// CurrentAppPackage func to get info on current package name.
func CurrentAppPackage(ctx context.Context, d *ui.Device, s *testing.State) (string, error) {
	// Wait for app to launch.
	d.WaitForIdle(ctx, ShortUITimeout)
	// Check if UiAutomator server is up.
	isALive := d.Alive(ctx)
	if !isALive {
		s.Fatal("UiAutomator server isn't responding: ", isALive)
	}
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
		cancelText                  = "Cancel"
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
	cancelButton := d.Object(ui.TextMatches("(?i)" + cancelText))

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
		if err := cancelButton.Exists(ctx); err == nil {
			s.Log("Click on cancelButton")
			cancelButton.Click(ctx)
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
					if err != nil {
						s.Fatal("Failed to get x1CoordinateValue: ", err)
					}
					y1coordinateWithOutTrim := strings.Split(splitCoordinate[CoordinateInfo], ",")[1]
					y1Coordinate := strings.Split(y1coordinateWithOutTrim, "]")[0]
					y1CoordinateValue, err := strconv.Atoi(y1Coordinate)
					if err != nil {
						s.Fatal("Failed to get y1CoordinateValue: ", err)
					}
					x2coordinateWithOutTrim := strings.Split(splitCoordinate[CoordinateInfo], ",")[1]
					x2Coordinate := strings.Split(x2coordinateWithOutTrim, "[")[1]
					x2CoordinateValue, err := strconv.Atoi(x2Coordinate)
					if err != nil {
						s.Fatal("Failed to get x2CoordinateValue: ", err)
					}
					y2coordinateWithOutTrim := strings.Split(splitCoordinate[CoordinateInfo], ",")[2]
					y2Coordinate := strings.Split(y2coordinateWithOutTrim, "]")[0]
					y2CoordinateValue, err := strconv.Atoi(y2Coordinate)
					if err != nil {
						s.Fatal("Failed to get y2CoordinateValue: ", err)
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

// getAppWindowInfo func provides coordinates of the app.
func getAppWindowInfo(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName string) (string, error) {

	var windowInfo string

	// To get app activities.
	out, err := a.Command(ctx, "dumpsys", "activity", "activities").Output()
	if err != nil {
		s.Fatal("Failed to get dumpsys activity activities: ", err)
	}
	output := string(out)
	TaskRecordPrefix := "* Task"
	windowStatePrefixForARCP := "WindowState"
	windowStatePrefixForARCR := "mode="
	var arcpFound bool
	var arcrFound bool

	splitOutput := strings.Split(output, "\n")
	for splitLine := range splitOutput {
		if strings.Contains(splitOutput[splitLine], appPkgName) && strings.Contains(splitOutput[splitLine], TaskRecordPrefix) {
			splitWindowInfo := strings.Split(splitOutput[splitLine], " ")
			for appWindowInfo := range splitWindowInfo {
				if !arcpFound && strings.Contains(splitWindowInfo[appWindowInfo], windowStatePrefixForARCP) {
					s.Log("windowInfo raw message ARCP: ", splitWindowInfo[appWindowInfo])
					windowInfoForARCP := strings.Split(splitWindowInfo[appWindowInfo], "{")[1]
					s.Log("windowInfoARCP: ", windowInfoForARCP)
					windowInfo = windowInfoForARCP
					arcpFound = true
					break
				}
				if !arcrFound && strings.Contains(splitWindowInfo[appWindowInfo], windowStatePrefixForARCR) {
					s.Log("windowInfo raw message ARCR: ", splitWindowInfo[appWindowInfo])
					windowInfoForARCR := strings.Split(splitWindowInfo[appWindowInfo], "=")[1]
					s.Log("windowInfoARCR: ", windowInfoForARCR)
					windowInfo = windowInfoForARCR
					arcrFound = true
					break
				}
			}
		}
	}
	return windowInfo, err
}

// dragToSnapFirstOverviewWindow finds the first window in overview, and drags
// to snap it. This function assumes that overview is already active.
func dragToSnapFirstOverviewWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, tew *input.TouchscreenEventWriter, stw *input.SingleTouchEventWriter, targetX input.TouchCoord) error {
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the internal display info")
	}
	tcc := tew.NewTouchCoordConverter(info.Bounds.Size())

	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find window in overview grid")
	}

	centerX, centerY := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
	if err := stw.LongPressAt(ctx, centerX, centerY); err != nil {
		return errors.Wrap(err, "failed to long-press to start dragging landscape window")
	}
	s.Logf("Long pressed at (%v, %v)", centerX, centerY)

	if err := stw.Swipe(ctx, centerX, centerY, targetX, tew.Height()/2, time.Second); err != nil {
		return errors.Wrap(err, "failed to swipe for snapping window to left")
	}
	s.Logf("Swiped from: (%v, %v) to ( %v, %v, %v)", centerX, centerY, targetX, tew.Height()/2, time.Second)

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to end swipe")
	}
	return nil
}

// GetAppVersion provides info on app version.
func GetAppVersion(ctx context.Context, s *testing.State, a *arc.ARC, d *ui.Device, appPkgName string) (string, error) {
	var versionNameAfterSplit string
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
				versionNameAfterSplit = strings.Split(splitOutput[splitLine], "=")[1]
				s.Log("Version name of ", appPkgName, " is: ", versionNameAfterSplit)
				break
			}
		}
	}
	return versionNameAfterSplit, err
}

// TabletOnlyModels is a list of tablet only models to be skipped from clamshell mode runs.
var TabletOnlyModels = []string{
	"dru",
	"krane",
}

// ClamshellOnlyModels is a list of clamshell only models to be skipped from tablet mode runs.
var ClamshellOnlyModels = []string{
	"sarien",
	"elemi",
	"berknip",
	"dratini",

	// grunt:
	"careena",
	"kasumi",
	"treeya",
	"grunt",
	"barla",
	"aleena",
	"liara",
	"nuwani",

	// octopus:
	"bluebird",
	"apel",
	"blooglet",
	"blorb",
	"bobba",
	"casta",
	"dorp",
	"droid",
	"fleex",
	"foob",
	"garfour",
	"garg",
	"laser14",
	"lick",
	"mimrock",
	"nospike",
	"orbatrix",
	"phaser",
	"sparky",
}
