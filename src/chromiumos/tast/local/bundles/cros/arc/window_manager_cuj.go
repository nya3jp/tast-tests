// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// Apk compiled against target SDK 23 (Pre-N)
	wmPkg23 = "org.chromium.arc.testapp.windowmanager23"
	// Apk compiled against target SDK 24 (N)
	wmPkg24 = "org.chromium.arc.testapp.windowmanager24"

	// Different activities used by the subtests.
	wmResizeableLandscapeActivity      = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"
	wmNonResizeableLandscapeActivity   = "org.chromium.arc.testapp.windowmanager.NonResizeableLandscapeActivity"
	wmResizeableUnspecifiedActivity    = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
	wmNonResizeableUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.NonResizeableUnspecifiedActivity"
	wmResizeablePortraitActivity       = "org.chromium.arc.testapp.windowmanager.ResizeablePortraitActivity"
	wmNonResizeablePortraitActivity    = "org.chromium.arc.testapp.windowmanager.NonResizeablePortraitActivity"
	wmLandscapeActivity                = "org.chromium.arc.testapp.windowmanager.LandscapeActivity"
	wmUnspecifiedActivity              = "org.chromium.arc.testapp.windowmanager.UnspecifiedActivity"
	wmPortraitActivity                 = "org.chromium.arc.testapp.windowmanager.PortraitActivity"

	// These values must match the strings from ArcWMTestApp defined in BaseActivity#parseCaptionButtons:
	// http://cs/android/vendor/google_arc/packages/development/ArcWMTestApp/src/org/chromium/arc/testapp/windowmanager/BaseActivity.java?l=448
	wmAutoHide  = "auto_hide"
	wmBack      = "back"
	wmClose     = "close"
	wmLandscape = "landscape"
	wmMaximize  = "maximize"
	wmMinimize  = "minimize"
	wmPortrait  = "portrait"
	wmRestore   = "restore"
	wmVisible   = "visible"
)

// wmTestStateFunc represents a function that tests if the window is in a certain state.
type wmTestStateFunc func(context.Context, *arc.Activity, *ui.Device) error

// uiFunc represents a function that presses a widget using UI Automator.
type uiFunc func(context.Context, *arc.Activity, *ui.Device) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowManagerCUJ,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-p",
		Contacts:     []string{"ricardoq@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcWMTestApp_23.apk", "ArcWMTestApp_24.apk", "ArcPipTastTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WindowManagerCUJ(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	for _, apk := range []string{"ArcWMTestApp_23.apk", "ArcWMTestApp_24.apk", "ArcPipTastTest.apk"} {
		if err := a.Install(ctx, s.DataPath(apk)); err != nil {
			s.Fatal("Failed installing app: ", err)
		}
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		// Be nice and restore tablet mode to its original state on exit.
		defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to  set tablet mode disabled: ", err)
		}
		// TODO(ricardoq): Wait for tablet mode animation is finished in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	type testFunc func(context.Context, *chrome.Conn, *arc.ARC, *ui.Device) error
	for idx, test := range []struct {
		name string
		fn   testFunc
	}{
		// Tests are grouped by tablet/clamshell mode to avoid unnecessary switches.
		// {"Default Launch Clamshell N", wmDefaultLaunchClamshell24},
		// {"Default Launch Clamshell Pre-N", wmDefaultLaunchClamshell23},
		// {"Maximize / Restore Clamshell N", wmMaximizeRestoreClamshell24},
		// {"Maximize / Restore Clamshell Pre-N", wmMaximizeRestoreClamshell23},
		// {"Follow Root Activity N / Pre-N", wmFollowRoot},
		// {"Springboard N / Pre-N", wmSpringboard},
		// {"Lights out / Lights in N", wmLightsOutIn},
		// {"Lights out ignored", wmLightsOutIgnored},
		{"Picture in Picutre", wmPIP},
	} {
		s.Logf("Running test %q", test.name)

		// Reset WM state to default values.
		if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
			s.Fatal("Failed to clear task states: ", err)
		}

		if err := test.fn(ctx, tconn, a, d); err != nil {
			path := fmt.Sprintf("%s/screenshot-cuj-failed-test-%d.png", s.OutDir(), idx)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("%q test failed: %v", test.name, err)
		}
	}
}

// wmDefaultLaunchClamshell24 launches in clamshell mode six SDK-N activities with different orientations
// and resize conditions. And it verifies that their default launch state is the expected one, as defined in:
// go/arc-wm-p "Clamshell: default launch behavior - Android NYC or above" (slide #6).
func wmDefaultLaunchClamshell24(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name        string
		act         string
		wantedState wmTestStateFunc
	}{
		// The are four possible default states (windows #A to #D) from six possible different activities.
		// Window #A.
		{"Landscape + Resize enabled", wmResizeableLandscapeActivity, checkMaximizeResizeable},
		// Window #B.
		{"Landscape + Resize disabled", wmNonResizeableLandscapeActivity, checkMaximizeNonResizeable},
		// Window #A.
		{"Unspecified + Resize enabled", wmResizeableUnspecifiedActivity, checkMaximizeResizeable},
		// Window #B.
		{"Unspecified + Resize disabled", wmNonResizeableUnspecifiedActivity, checkMaximizeNonResizeable},
		// Window #C.
		{"Portrait + Resized enabled", wmResizeablePortraitActivity, checkRestoreResizeable},
		// Window #D.
		{"Portrait + Resize disabled", wmNonResizeablePortraitActivity, checkPillarboxNonResizeable},
	} {
		testing.ContextLogf(ctx, "Running subtest %q", test.name)
		act, err := arc.NewActivity(a, wmPkg24, test.act)
		if err != nil {
			return err
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			return err
		}
		// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
		defer act.Stop(ctx)

		if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
			return err
		}

		if err := test.wantedState(ctx, act, d); err != nil {
			return err
		}

		// Stopping current activity in order to make it possible to launch a different from the same package.
		if err := act.Stop(ctx); err != nil {
			return errors.Wrapf(err, "could not stop activity %v", test.act)
		}
	}
	return nil
}

// wmDefaultLaunchClamshell23 launches in clamshell mode six SDK-pre-N activities with different orientations
// and resize conditions. And it verifies that their default launch state is the expected one, as defined in:
// go/arc-wm-p "Clamshell: default launch behavior - Android MNC and below" (slide #7).
func wmDefaultLaunchClamshell23(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name        string
		act         string
		wantedState wmTestStateFunc
	}{
		// The are two possible default states (windows #A to #B) from three possible different activities.
		// Window #A.
		{"Unspecified", wmUnspecifiedActivity, checkRestoreResizeable},
		// Window #A.
		{"Portrait", wmPortraitActivity, checkRestoreResizeable},
		// Window #B.
		{"Landscape", wmLandscapeActivity, checkRestoreResizeable},
	} {
		testing.ContextLogf(ctx, "Running subtest %q", test.name)
		act, err := arc.NewActivity(a, wmPkg23, test.act)
		if err != nil {
			return err
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			return err
		}
		// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
		defer act.Stop(ctx)

		if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
			return err
		}

		if err := test.wantedState(ctx, act, d); err != nil {
			return err
		}

		// Stopping current activity in order to make it possible to launch a different from the same package.
		if err := act.Stop(ctx); err != nil {
			return errors.Wrapf(err, "could not stop activity %v", test.act)
		}
	}
	return nil
}

// wmMaximizeRestoreClamshell24 verifies that switching to maximize state from restore state, and vice-versa, works as defined in:
// go/arc-wm-p "Clamshell: maximize/restore" (slides #8 - #10).
func wmMaximizeRestoreClamshell24(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name         string
		act          string
		stateA       arc.WindowState
		wantedStateA wmTestStateFunc
		stateB       arc.WindowState
		wantedStateB wmTestStateFunc
	}{
		{"Unspecified", wmResizeableUnspecifiedActivity,
			arc.WindowStateMaximized, checkMaximizeResizeable, arc.WindowStateNormal, checkRestoreResizeable},
		{"Portrait", wmResizeablePortraitActivity,
			arc.WindowStateNormal, checkRestoreResizeable, arc.WindowStateMaximized, checkPillarboxResizeable},
		{"Landscape", wmResizeableLandscapeActivity,
			arc.WindowStateMaximized, checkMaximizeResizeable, arc.WindowStateNormal, checkRestoreResizeable},
	} {
		testing.ContextLogf(ctx, "Running subtest %q", test.name)
		act, err := arc.NewActivity(a, wmPkg24, test.act)
		if err != nil {
			return err
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			return err
		}
		// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
		defer act.Stop(ctx)

		if err := test.wantedStateA(ctx, act, d); err != nil {
			return err
		}

		if err := act.SetWindowState(ctx, test.stateB); err != nil {
			return err
		}

		if err := test.wantedStateB(ctx, act, d); err != nil {
			return err
		}

		if err := act.SetWindowState(ctx, test.stateA); err != nil {
			return err
		}

		if err := test.wantedStateA(ctx, act, d); err != nil {
			return err
		}

		// Stop current activity in order to make it possible to launch a different from the same package.
		if err := act.Stop(ctx); err != nil {
			return errors.Wrapf(err, "could not stop activity %v", test.act)
		}
	}
	return nil
}

// wmMaximizeRestoreClamshell23 verifies that switching to maximize state from restore state, and vice-versa, works as defined in:
// go/arc-wm-p "Clamshell: maximize/restore" (slides #11 - #13).
func wmMaximizeRestoreClamshell23(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name           string
		act            string
		maximizedState wmTestStateFunc
	}{
		{"Landscape", wmLandscapeActivity, checkMaximizeResizeable},
		{"Unspecified", wmUnspecifiedActivity, checkMaximizeResizeable},
		{"Portrait", wmPortraitActivity, checkPillarboxResizeable},
	} {
		testing.ContextLogf(ctx, "Running subtest %q", test.name)
		act, err := arc.NewActivity(a, wmPkg23, test.act)
		if err != nil {
			return err
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			return err
		}
		// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
		defer act.Stop(ctx)

		if err := checkRestoreResizeable(ctx, act, d); err != nil {
			return err
		}

		if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
			return err
		}

		// Wait for the "Application needs to restart to resize" dialog that appears on all Pre-N apks.
		if err := uiWaitForRestartDialogAndRestart(ctx, act, d); err != nil {
			return err
		}

		// Could be either maximized or pillarbox states.
		if err := test.maximizedState(ctx, act, d); err != nil {
			return err
		}

		if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
			return err
		}

		// Wait for the "Application needs to restart to resize" dialog that appears on all Pre-N apks.
		if err := uiWaitForRestartDialogAndRestart(ctx, act, d); err != nil {
			return err
		}

		if err := checkRestoreResizeable(ctx, act, d); err != nil {
			return err
		}

		// Stop current activity in order to make it possible to launch a different from the same package.
		if err := act.Stop(ctx); err != nil {
			return errors.Wrapf(err, "could not stop activity %v", test.act)
		}
	}
	return nil
}

// wmFollowRoot verifies that child activities follow the root activity state as defined in:
// go/arc-wm-p "Clamshell: new activities follow root activity" (slides #15 - #17).
func wmFollowRoot(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name    string
		pkgName string
		act     string
	}{
		{"Unspecified (N)", wmPkg24, wmResizeableUnspecifiedActivity},
		{"Portrait (N)", wmPkg24, wmResizeablePortraitActivity},
		{"Landscape (N)", wmPkg24, wmResizeableLandscapeActivity},

		{"Unspecified (Pre-N)", wmPkg23, wmUnspecifiedActivity},
		{"Portrait (Pre-N)", wmPkg23, wmPortraitActivity},
		{"Landscape (Pre-N)", wmPkg23, wmLandscapeActivity},
	} {
		for _, orientation := range []struct {
			name string
			fn   uiFunc
		}{
			{"Unspecified", uiClickUnspecified},
			{"Landscape", uiClickLandscape},
			{"Portrait", uiClickPortrait},
		} {
			testing.ContextLogf(ctx, "Running subtest: parent = %q, child = %q", test.name, orientation.name)

			if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
				return errors.Wrap(err, "failed to clear WM state")
			}
			act, err := arc.NewActivity(a, test.pkgName, test.act)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				return err
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx)

			s, err := act.GetWindowState(ctx)
			if err != nil {
				return err
			}
			// Slides only cover when the initial activity is Restored. Probably an oversight.
			// TODO(ricardoq): Add tests for initial state Maximized.
			if s != arc.WindowStateNormal {
				if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
					return err
				}
			}

			origOrientation, err := uiOrientation(ctx, act, d)
			if err != nil {
				return err
			}

			if err := orientation.fn(ctx, act, d); err != nil {
				return err
			}
			if err := uiClickLaunchActivity(ctx, act, d); err != nil {
				return err
			}

			// Window state and orientation should not change, and there should be two activities in the stack.

			newState, err := act.GetWindowState(ctx)
			if err != nil {
				return err
			}
			if newState != arc.WindowStateNormal {
				return errors.Errorf("invalid window state %q; want %q", newState.String(), arc.WindowStateNormal.String())
			}

			newOrientation, err := uiOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if newOrientation != origOrientation {
				return errors.Errorf("invalid orientation %q; want %q", newOrientation, origOrientation)
			}

			nrActivities, err := uiNumberActivities(ctx, act, d)
			if err != nil {
				return err
			}
			if nrActivities != 2 {
				return errors.Errorf("invalid number of activities %d; want 2", nrActivities)
			}

			// Stop current activity in order to make it possible to launch a different from the same package.
			if err := act.Stop(ctx); err != nil {
				return errors.Wrapf(err, "could not stop activity %v", test.act)
			}
		}
	}
	return nil
}

// wmSpringboard verifies that child activities do not honor the root activity state as defined in:
// go/arc-wm-p "Clamshell: Springboard activities" (slide #18).
func wmSpringboard(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name    string
		pkgName string
		act     string
	}{
		{"Unspecified (N)", wmPkg24, wmResizeableUnspecifiedActivity},
		{"Portrait (N)", wmPkg24, wmResizeablePortraitActivity},
		{"Landscape (N)", wmPkg24, wmResizeableLandscapeActivity},

		{"Unspecified (Pre-N)", wmPkg23, wmUnspecifiedActivity},
		{"Portrait (Pre-N)", wmPkg23, wmPortraitActivity},
		{"Landscape (Pre-N)", wmPkg23, wmLandscapeActivity},
	} {
		for _, orientation := range []struct {
			name   string
			fn     uiFunc
			wanted string
		}{
			{"Landscape", uiClickLandscape, wmLandscape},
			{"Portrait", uiClickPortrait, wmPortrait},
		} {
			testing.ContextLogf(ctx, "Running subtest: parent = %q, child = %q", test.name, orientation.name)

			if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
				return errors.Wrap(err, "failed to clear WM state")
			}
			act, err := arc.NewActivity(a, test.pkgName, test.act)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				return err
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx)

			s, err := act.GetWindowState(ctx)
			if err != nil {
				return err
			}
			// Slides only cover when the initial activity is Restored. Probably an oversight.
			// TODO(ricardoq): Add tests for initial state Maximized.
			if s != arc.WindowStateNormal {
				if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
					return err
				}
			}

			if err := orientation.fn(ctx, act, d); err != nil {
				return err
			}

			if err := uiClickRootActivity(ctx, act, d); err != nil {
				return err
			}

			if err := uiClickLaunchActivity(ctx, act, d); err != nil {
				return err
			}

			// Orientation should change, and there should be only one activity in the stack.

			newOrientation, err := uiOrientation(ctx, act, d)
			if err != nil {
				return err
			}
			if newOrientation != orientation.wanted {
				return errors.Errorf("invalid orientation %q; want %q", newOrientation, orientation.wanted)
			}

			nrActivities, err := uiNumberActivities(ctx, act, d)
			if err != nil {
				return err
			}
			if nrActivities != 1 {
				return errors.Errorf("invalid number of activities %d; want 1", nrActivities)
			}

			// Stop current activity in order to make it possible to launch a different from the same package.
			if err := act.Stop(ctx); err != nil {
				return errors.Wrapf(err, "could not stop activity %v", test.act)
			}
		}
	}
	return nil
}

// wmLightsOutIn verifies that the activity can go from maximized to fullscreen mode, an vice-versa
// as defined in go/arc-wm-p:
// "Clamshell: lights out or fullscreen" (slide #19) and
// "Clamshell: exit lights out or fullscreen" (slide #20).
func wmLightsOutIn(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	act, err := arc.NewActivity(a, wmPkg24, wmResizeableLandscapeActivity)
	if err != nil {
		return err
	}
	defer act.Close()

	for _, test := range []struct {
		name        string
		lightsOutFn func() error
		lightsInFn  func() error
	}{
		{"Using Zoom Toggle key",
			func() error { return toggleFullscreen(ctx, tconn) },
			func() error { return toggleFullscreen(ctx, tconn) }},
		{"Using Android API",
			func() error { return uiClickImmersive(ctx, act, d) },
			func() error { return uiClickNormal(ctx, act, d) }},
	} {
		testing.ContextLogf(ctx, "Running %q", test.name)
		if err := act.Start(ctx); err != nil {
			return err
		}
		// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
		defer act.Stop(ctx)

		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}

		// Initial state: maximized with visible caption.
		ws, err := act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		if ws != arc.WindowStateMaximized {
			return errors.Errorf("invalid window state %q; want WindowStateMaximized", ws.String())
		}

		s, err := getUIState(ctx, act, d)
		if err != nil {
			return err
		}
		if s.CaptionVisibility != wmVisible {
			return errors.Errorf("invalid caption visibility %q; want 'auto_hide'", s.CaptionVisibility)
		}

		// Invoke fullscreen method.
		if err := test.lightsOutFn(); err != nil {
			return err
		}

		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}

		// Wanted state: fullscreen with auto-hide caption.
		ws, err = act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		if ws != arc.WindowStateFullscreen {
			return errors.Errorf("invalid window state %q; want 'WindowStateFullscreen'", ws.String())
		}

		s, err = getUIState(ctx, act, d)
		if err != nil {
			return err
		}
		if s.CaptionVisibility != wmAutoHide {
			return errors.Errorf("invalid caption visibility %q; want 'auto_hide'", s.CaptionVisibility)
		}

		// Invoke maximized method.
		if err := test.lightsInFn(); err != nil {
			return err
		}

		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}

		// Wanted state: Maximized
		ws, err = act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		if ws != arc.WindowStateMaximized {
			return errors.Errorf("invalid window state %q; want 'WindowStateFullscreen'", ws.String())
		}

		if err := act.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// wmLightsOutIgnored verifies that an N activity cannot go from restored to fullscreen mode as defined in:
// go/arc-wm-p "Clamshell: lights out and fullscreen ignored" (slides #21-#23)
func wmLightsOutIgnored(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name     string
		pkg      string
		activity string
	}{
		// Skipping test for Slide #21.
		// TODO(ricardoq): Confirm that slide #21 seems to be a duplicate of Slide #22.
		// Slide #22
		{"Landscape + Resize enabled + N", wmPkg24, wmResizeableLandscapeActivity},
		// Slide #23
		{"Portrait + Resize enabled + N", wmPkg24, wmResizeablePortraitActivity},
		// Slide #22
		{"Landscape + PreN", wmPkg23, wmLandscapeActivity},
		// Slide #23
		{"Portrait + PreN", wmPkg23, wmPortraitActivity},
	} {
		testing.ContextLogf(ctx, "Running %q", test.name)
		act, err := arc.NewActivity(a, test.pkg, test.activity)
		if err != nil {
			return err
		}
		defer act.Close()

		if err := act.Start(ctx); err != nil {
			return err
		}

		if err := act.Start(ctx); err != nil {
			return err
		}
		defer act.Stop(ctx)

		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}

		ws, err := act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		// PreN apps by default are in Normal state. N apps should be Maximized.
		// As required by slides #21-#23, set them to Normal.
		if ws != arc.WindowStateNormal {
			if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
				return err
			}
		}

		// Clicking on "Immersive" button should not change the state of the restored window.
		if err := uiClickImmersive(ctx, act, d); err != nil {
			return err
		}

		if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
			return err
		}

		ws, err = act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		wanted := arc.WindowStateNormal
		if ws != wanted {
			return errors.Errorf("invalid window state %q; want %q", ws.String(), wanted.String())
		}

		if err := act.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// wmPIP verifies that the activity enters into Picture In Picture mode when occluded as defined in:
// go/arc-wm-p "Clamshell: Picture in Picture" (slide #24)
func wmPIP(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// 1) Launch a PIP-ready activity in non-PIP mode
	testing.ContextLog(ctx, "Launching PIP activity")
	const pkgName = "org.chromium.arc.testapp.pictureinpicture"
	actPIP, err := arc.NewActivity(a, pkgName, ".MainActivity")
	if err != nil {
		return err
	}
	defer actPIP.Close()
	if err := actPIP.Start(ctx); err != nil {
		return err
	}
	if err := actPIP.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}
	ws, err := actPIP.GetWindowState(ctx)
	if err != nil {
		return err
	}
	notWanted := arc.WindowStatePIP
	if ws == notWanted {
		return errors.Errorf("invalid window state %q; want non-PIP state", notWanted.String())
	}

	// 2) Launch a maximized application to make sure that it occludes the previous activity
	testing.ContextLog(ctx, "Launching maximized activity")
	act, err := arc.NewActivity(a, wmPkg24, wmResizeableLandscapeActivity)
	if err != nil {
		return err
	}
	defer act.Close()
	if err := act.Start(ctx); err != nil {
		return err
	}
	if err := act.WaitForIdle(ctx, 5*time.Second); err != nil {
		return err
	}

	// 3) Verify that the occluded activity is in PIP mode.
	ws, err = actPIP.GetWindowState(ctx)
	if err != nil {
		return err
	}
	wanted := arc.WindowStatePIP
	if ws != wanted {
		return errors.Errorf("invalid window state %q; want %q", ws.String(), wanted.String())
	}

	return nil
}

// Helper functions

// checkMaximizeResizeable checks that the window is maximized and is resizeable.
func checkMaximizeResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateMaximized); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmRestore, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkMaximizeNonResizeable checks that the window is maximized and is non-resizeable.
func checkMaximizeNonResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateMaximized); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkRestoreResizeable checks that the window is in restore mode and is resizeable.
func checkRestoreResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmMaximize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkRestoreNonResizeable checks that the window is in restore mode and is non-resizeable.
func checkRestoreNonResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkPillarboxResizeable checks that the window is in pillar-box mode and is resizeable.
func checkPillarboxResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := checkPillarbox(ctx, act, d); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmRestore, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkPillarboxNonResizeable checks that the window is in pillar-box mode and is non-resizeable.
func checkPillarboxNonResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := checkPillarbox(ctx, act, d); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkPillarbox checks that the window is in pillar-box mode.
func checkPillarbox(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	const wanted = wmPortrait
	o, err := uiOrientation(ctx, act, d)
	if err != nil {
		return err
	}
	if o != wanted {
		return errors.Errorf("invalid orientation %v; want %v", o, wanted)
	}

	return compareWMState(ctx, act, arc.WindowStateMaximized)
}

// compareWMState compares the activity window state  with the wanted one.
// Returns nil only if they are equal.
func compareWMState(ctx context.Context, act *arc.Activity, wanted arc.WindowState) error {
	state, err := act.GetWindowState(ctx)
	if err != nil {
		return err
	}
	if state != wanted {
		return errors.Errorf("invalid window state %v, want %v", state, wanted)
	}
	return nil
}

// compareCaption compares the activity caption buttons with the wanted one.
// Returns nil only if they are equal.
func compareCaption(ctx context.Context, act *arc.Activity, d *ui.Device, wantedCaption []string) error {
	bn, err := uiCaptionButtons(ctx, act, d)
	if err != nil {
		return errors.Wrap(err, "could not get caption buttons state")
	}
	if !reflect.DeepEqual(bn, wantedCaption) {
		return errors.Errorf("invalid caption buttons %+v, want %+v", bn, wantedCaption)
	}
	return nil
}

// toggleFullscreen toggles fullscreen by injecting the Zoom Toggle keycode.
func toggleFullscreen(ctx context.Context, tconn *chrome.Conn) error {
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	l, err := input.KeyboardTopRowLayout(ctx, ew)
	if err != nil {
		return err
	}
	k := l.ZoomToggle
	return ew.Accel(ctx, k)
}

// Helper UI functions
// These functions use UI Automator to get / change the state of ArcWMTest activity.

// uiState represents the state of ArcWMTestApp activity.
type uiState struct {
	WindowState       string      `json:"windowState"`
	Orientation       string      `json:"orientation"`
	DeviceMode        string      `json:"deviceMode"`
	ActivityNr        int         `json:"activityNr"`
	CaptionVisibility string      `json:"captionVisibility"`
	Zoomed            bool        `json:"zoomed"`
	Rotation          int         `json:"rotation"`
	Buttons           []string    `json:"buttons"`
	Accel             interface{} `json:"accel"`
}

// getUIState returns the state from the ArcWMTest activity.
// The state is taken by parsing the activity's TextView which contains the state in JSON format.
func getUIState(ctx context.Context, act *arc.Activity, d *ui.Device) (*uiState, error) {
	obj := d.Object(
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.TextView"),
		ui.ResourceIDMatches(".+?(/caption_text_view)$"))
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return nil, err
	}
	s, err := obj.GetText(ctx)
	if err != nil {
		return nil, err
	}
	var state uiState
	if err := json.Unmarshal([]byte(s), &state); err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling state")
	}
	return &state, nil
}

// uiCaptionButtons returns the caption buttons that are present in the ArcWMTestApp window.
func uiCaptionButtons(ctx context.Context, act *arc.Activity, d *ui.Device) (buttons []string, err error) {
	s, err := getUIState(ctx, act, d)
	if err != nil {
		return nil, err
	}
	return s.Buttons, nil
}

// uiOrientation returns the current orientation of the ArcWMTestApp window.
func uiOrientation(ctx context.Context, act *arc.Activity, d *ui.Device) (string, error) {
	s, err := getUIState(ctx, act, d)
	if err != nil {
		return "", err
	}
	return s.Orientation, nil
}

// uiNumberActivities returns the number of activities present in the ArcWMTestApp stack.
func uiNumberActivities(ctx context.Context, act *arc.Activity, d *ui.Device) (int, error) {
	s, err := getUIState(ctx, act, d)
	if err != nil {
		return 0, err
	}
	return s.ActivityNr, nil
}

// uiClicks sends a "Click" message an UI Object.
// The UI Object is selected from opts, which are the selectors.
func uiClick(ctx context.Context, d *ui.Device, opts ...ui.SelectorOption) error {
	obj := d.Object(opts...)
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "could not click on widget")
	}
	return nil
}

// uiWaitForRestartDialogAndRestart waits for the "Application needs to restart to resize" dialog.
// This dialog appears on all Pre-N applications that tries to switch between maximized / restored window states.
// See: http://cs/pi-arc-dev/frameworks/base/core/java/com/android/internal/policy/DecorView.java
func uiWaitForRestartDialogAndRestart(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.ClassName("android.widget.Button"),
		ui.ID("android:id/button1"),
		ui.TextMatches("(?i)Restart")); err != nil {
		return errors.Wrap(err, "failed to click on Restart button")
	}
	return act.WaitForIdle(ctx, 10*time.Second)
}

// uiClickUnspecified clicks on the "Unspecified" radio button that is present in the ArcWMTest activity.
func uiClickUnspecified(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.RadioButton"),
		ui.TextMatches("(?i)Unspecified")); err != nil {
		return errors.Wrap(err, "failed to click on Unspecified radio button")
	}
	return nil
}

// uiClickLandscape clicks on the "Landscape" radio button that is present in the ArcWMTest activity.
func uiClickLandscape(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.RadioButton"),
		ui.TextMatches("(?i)Landscape")); err != nil {
		return errors.Wrap(err, "failed to click on Landscape radio button")
	}
	return nil
}

// uiClickPortrait clicks on the "Portrait" radio button that is present in the ArcWMTest activity.
func uiClickPortrait(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.RadioButton"),
		ui.TextMatches("(?i)Portrait")); err != nil {
		return errors.Wrap(err, "failed to click on Portrait radio button")
	}
	return nil
}

// uiClickRootActivity clicks on the "Root Activity" checkbox that is present on the ArcWMTest activity.
func uiClickRootActivity(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.CheckBox"),
		ui.TextMatches("(?i)Root Activity")); err != nil {
		return errors.Wrap(err, "failed to click on Root Activity checkbox")
	}
	return nil
}

// uiClickImmersive clicks on the "Immersive" button that is present on the ArcWMTest activity.
func uiClickImmersive(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)Immersive")); err != nil {
		return errors.Wrap(err, "failed to click on Immersive button")
	}
	return nil
}

// uiClickNormal clicks on the "Normal" button that is present on the ArcWMTest activity.
func uiClickNormal(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)Normal")); err != nil {
		return errors.Wrap(err, "failed to click on Normal button")
	}
	return nil
}

// uiClickLaunchActivity clicks on the "Launch Activity" button that is present in the ArcWMTest activity.
func uiClickLaunchActivity(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := uiClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)Launch Activity")); err != nil {
		return errors.Wrap(err, "failed to click on Launch Activity button")
	}
	return act.WaitForIdle(ctx, 10*time.Second)
}
