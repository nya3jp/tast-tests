// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/settings"
	"chromiumos/tast/local/coords"
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

	// Landscape and Portrait constaints come from:
	// http://cs/android/vendor/google_arc/packages/development/ArcWMTestApp/src/org/chromium/arc/testapp/windowmanager/BaseActivity.java?l=411
	wmLandscape = "landscape"
	wmPortrait  = "portrait"
)

// wmTestStateFunc represents a function that tests if the window is in a certain state.
type wmTestStateFunc func(context.Context, *chrome.Conn, *arc.Activity, *ui.Device) error

// uiClickFunc represents a function that "clicks" on a certain widget using UI Automator.
type uiClickFunc func(context.Context, *arc.Activity, *ui.Device) error

// wmCUJTestFunc represents the "test" function.
type wmCUJTestFunc func(context.Context, *chrome.Conn, *arc.ARC, *ui.Device) error

// wmCUJTestParams represents the name of test, and the function to call.
type wmCUJTestParams struct {
	name string
	fn   wmCUJTestFunc
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowManagerCUJ,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-p",
		Contacts:     []string{"ricardoq@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcWMTestApp_23.apk", "ArcWMTestApp_24.apk", "ArcPipSimpleTastTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
		Params: []testing.Param{{
			// Only stable tests should be placed here. The ones that are ready for CQ.
			Val: []wmCUJTestParams{
				{"Default Launch Clamshell N", wmDefaultLaunchClamshell24},
				{"Default Launch Clamshell Pre-N", wmDefaultLaunchClamshell23},
				{"Maximize / Restore Clamshell N", wmMaximizeRestoreClamshell24},
				{"Maximize / Restore Clamshell Pre-N", wmMaximizeRestoreClamshell23},
				{"Lights out / Lights in N", wmLightsOutIn},
				{"Lights out ignored", wmLightsOutIgnored},
				{"Picture in Picture", wmPIP},
			},
		}, {
			// New and flaky tests should be placed here. These tests should be fixed,
			// and moved them to "stable" ASAP.
			Name: "flaky",
			Val: []wmCUJTestParams{
				{"Follow Root Activity N / Pre-N", wmFollowRoot},
				{"Springboard N / Pre-N", wmSpringboard},
				{"Freeform Resize", wmFreeformResize},
				{"Snapping to half screen", wmSnapping},
				{"Display resolution", wmDisplayResolution},
				{"Page Zoom", wmPageZoom},
			},
		}},
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

	for _, apk := range []string{"ArcWMTestApp_23.apk", "ArcWMTestApp_24.apk", "ArcPipSimpleTastTest.apk"} {
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
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}
		// TODO(ricardoq): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	for idx, test := range s.Param().([]wmCUJTestParams) {
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
		{"Landscape + Resize enabled", wmResizeableLandscapeActivity, wm.CheckMaximizeResizeable},
		// Window #B.
		{"Landscape + Resize disabled", wmNonResizeableLandscapeActivity, wm.CheckMaximizeNonResizeable},
		// Window #A.
		{"Unspecified + Resize enabled", wmResizeableUnspecifiedActivity, wm.CheckMaximizeResizeable},
		// Window #B.
		{"Unspecified + Resize disabled", wmNonResizeableUnspecifiedActivity, wm.CheckMaximizeNonResizeable},
		// Window #C.
		{"Portrait + Resized enabled", wmResizeablePortraitActivity, wm.CheckRestoreResizeable},
		// Window #D.
		{"Portrait + Resize disabled", wmNonResizeablePortraitActivity, wm.CheckPillarboxNonResizeable},
	} {
		if err := func() error {
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
			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			return test.wantedState(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// wmDefaultLaunchClamshell23 launches in clamshell mode three SDK-pre-N activities with different orientations.
// And it verifies that their default launch state is the expected one, as defined in:
// go/arc-wm-p "Clamshell: default launch behavior - Android MNC and below" (slide #7).
func wmDefaultLaunchClamshell23(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name        string
		act         string
		wantedState wmTestStateFunc
	}{
		// The are two possible default states (windows #A to #B) from three possible different activities.
		// Window #A.
		{"Unspecified", wmUnspecifiedActivity, wm.CheckRestoreResizeable},
		// Window #A.
		{"Portrait", wmPortraitActivity, wm.CheckRestoreResizeable},
		// Window #B.
		{"Landscape", wmLandscapeActivity, wm.CheckRestoreResizeable},
	} {
		if err := func() error {
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
			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			return test.wantedState(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
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
		stateA       ash.WMEventType
		wantedStateA wmTestStateFunc
		stateB       ash.WMEventType
		wantedStateB wmTestStateFunc
	}{
		{"Unspecified", wmResizeableUnspecifiedActivity,
			ash.WMEventMaximize, wm.CheckMaximizeResizeable, ash.WMEventNormal, wm.CheckRestoreResizeable},
		{"Portrait", wmResizeablePortraitActivity,
			ash.WMEventNormal, wm.CheckRestoreResizeable, ash.WMEventMaximize, wm.CheckPillarboxResizeable},
		{"Landscape", wmResizeableLandscapeActivity,
			ash.WMEventMaximize, wm.CheckMaximizeResizeable, ash.WMEventNormal, wm.CheckRestoreResizeable},
	} {
		if err := func() error {
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

			if err := test.wantedStateA(ctx, tconn, act, d); err != nil {
				return err
			}

			if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), test.stateB); err != nil {
				return err
			}

			if err := test.wantedStateB(ctx, tconn, act, d); err != nil {
				return err
			}

			if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), test.stateA); err != nil {
				return err
			}

			return test.wantedStateA(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
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
		{"Landscape", wmLandscapeActivity, wm.CheckMaximizeResizeable},
		{"Unspecified", wmUnspecifiedActivity, wm.CheckMaximizeResizeable},
		{"Portrait", wmPortraitActivity, wm.CheckPillarboxResizeable},
	} {
		if err := func() error {
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
			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			if err := wm.CheckRestoreResizeable(ctx, tconn, act, d); err != nil {
				return err
			}

			// Calling ash.SetARCAppWindowState() doesn't work in this subtest, using act.SetWindowState instead.
			// Pre-N applications trigger a pop-up dialog  asking for confirmation. ash.SetARCAppWindowState() will
			// "wait forever" for a window event to occur, but this event won't occur due to the pop-up dialog.
			if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
				return err
			}

			// Wait for the "Application needs to restart to resize" dialog that appears on all Pre-N apks.
			if err := wm.UIWaitForRestartDialogAndRestart(ctx, act, d); err != nil {
				return err
			}

			// Could be either maximized or pillarbox states.
			if err := test.maximizedState(ctx, tconn, act, d); err != nil {
				return err
			}

			if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
				return err
			}

			// Wait for the "Application needs to restart to resize" dialog that appears on all Pre-N apks.
			if err := wm.UIWaitForRestartDialogAndRestart(ctx, act, d); err != nil {
				return err
			}

			return wm.CheckRestoreResizeable(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
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
		// Root activities.
		{"Unspecified (N)", wmPkg24, wmResizeableUnspecifiedActivity},
		{"Portrait (N)", wmPkg24, wmResizeablePortraitActivity},
		{"Landscape (N)", wmPkg24, wmResizeableLandscapeActivity},

		{"Unspecified (Pre-N)", wmPkg23, wmUnspecifiedActivity},
		{"Portrait (Pre-N)", wmPkg23, wmPortraitActivity},
		{"Landscape (Pre-N)", wmPkg23, wmLandscapeActivity},
	} {
		for _, orientation := range []struct {
			name string
			fn   uiClickFunc
		}{
			// Orientations for the child activity.
			{"Unspecified", wm.UIClickUnspecified},
			{"Landscape", wm.UIClickLandscape},
			{"Portrait", wm.UIClickPortrait},
		} {
			if err := func() error {
				testing.ContextLogf(ctx, "Running subtest: \"Root activity=%s -> child=%s\"", test.name, orientation.name)

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
				if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
					return err
				}

				if ws, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
					return err
				} else if ws != ash.WindowStateNormal {
					return errors.Errorf("failed to set window state: got %s, want %s", ws, ash.WMEventNormal)
				}

				origOrientation, err := wm.UIOrientation(ctx, act, d)
				if err != nil {
					return err
				}

				if err := orientation.fn(ctx, act, d); err != nil {
					return err
				}
				if err := wm.UIClickLaunchActivity(ctx, act, d); err != nil {
					return err
				}

				// Window state and orientation should not change, and there should be two activities in the stack.

				if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
					return err
				}

				if newOrientation, err := wm.UIOrientation(ctx, act, d); err != nil {
					return err
				} else if newOrientation != origOrientation {
					return errors.Errorf("invalid orientation: got %q; want %q", newOrientation, origOrientation)
				}

				if nrActivities, err := wm.UINumberActivities(ctx, act, d); err != nil {
					return err
				} else if nrActivities != 2 {
					return errors.Errorf("invalid number of activities: got %d; want 2", nrActivities)
				}

				return nil
			}(); err != nil {
				return errors.Wrapf(err, "\"Root activity=%s -> child=%s\" subtest failed", test.name, orientation.name)
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
			fn     uiClickFunc
			wanted string
		}{
			{"Landscape", wm.UIClickLandscape, wmLandscape},
			{"Portrait", wm.UIClickPortrait, wmPortrait},
		} {
			if err := func() error {
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
				if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
					return err
				}

				if s, err := ash.GetARCAppWindowState(ctx, tconn, act.PackageName()); err != nil {
					return err
				} else if s != ash.WindowStateNormal {
					if ws, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
						return err
					} else if ws != ash.WindowStateNormal {
						return errors.Errorf("failed to set window state: got %s, want %s", ws, ash.WindowStateNormal)
					}
				}

				if err := orientation.fn(ctx, act, d); err != nil {
					return err
				}

				if err := wm.UIClickRootActivity(ctx, act, d); err != nil {
					return err
				}

				if err := wm.UIClickLaunchActivity(ctx, act, d); err != nil {
					return err
				}

				if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
					return err
				}

				// Orientation should change, and there should be only one activity in the stack.

				if newOrientation, err := wm.UIOrientation(ctx, act, d); err != nil {
					return err
				} else if newOrientation != orientation.wanted {
					return errors.Errorf("invalid orientation: got %q; want %q", newOrientation, orientation.wanted)
				}

				if nrActivities, err := wm.UINumberActivities(ctx, act, d); err != nil {
					return err
				} else if nrActivities != 1 {
					return errors.Errorf("invalid number of activities: got %d; want 1", nrActivities)
				}
				return nil
			}(); err != nil {
				return errors.Wrapf(err, "%q subtest failed", test.name)
			}
		}
	}
	return nil
}

// wmLightsOutIn verifies that the activity can go from maximized to fullscreen mode, an vice-versa as defined in go/arc-wm-p:
// "Clamshell: lights out or fullscreen" (slide #19) and "Clamshell: exit lights out or fullscreen" (slide #20).
func wmLightsOutIn(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// Slides #19 and #20 describe this scenario with "Landscape" activities. But using "unspecified" since
	// a tablet in portrait mode (like Dru) + keyboard means that we have a clamshell device in portrait mode.
	act, err := arc.NewActivity(a, wmPkg24, wmResizeableUnspecifiedActivity)
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
			func() error { return wm.ToggleFullscreen(ctx, tconn) },
			func() error { return wm.ToggleFullscreen(ctx, tconn) }},
		{"Using Android API",
			func() error { return wm.UIClickImmersive(ctx, act, d) },
			func() error { return wm.UIClickNormal(ctx, act, d) }},
	} {
		if err := func() error {
			testing.ContextLogf(ctx, "Running subtest %q", test.name)
			if err := act.Start(ctx); err != nil {
				return err
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx)

			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			// Initial state: maximized with visible caption.
			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
				return err
			}

			if err := wm.WaitUntilFrameMatchesCondition(ctx, tconn, act.PackageName(), true, ash.FrameModeNormal); err != nil {
				return err
			}

			// Invoke fullscreen method.
			if err := test.lightsOutFn(); err != nil {
				return err
			}

			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateFullscreen); err != nil {
				return err
			}

			if err := wm.WaitUntilFrameMatchesCondition(ctx, tconn, act.PackageName(), false, ash.FrameModeImmersive); err != nil {
				return err
			}

			// Invoke maximized method.
			if err := test.lightsInFn(); err != nil {
				return err
			}

			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
				return err
			}

			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// wmLightsOutIgnored verifies that an N activity cannot go from restored to fullscreen mode as defined in:
// go/arc-wm-p "Clamshell: lights out and fullscreen ignored" (slides #22-#23)
func wmLightsOutIgnored(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name     string
		pkg      string
		activity string
	}{
		// Slide #22
		{"Landscape + Resize enabled + N", wmPkg24, wmResizeableLandscapeActivity},
		// Slide #23
		{"Portrait + Resize enabled + N", wmPkg24, wmResizeablePortraitActivity},
		// Slide #22
		{"Landscape + PreN", wmPkg23, wmLandscapeActivity},
		// Slide #23
		{"Portrait + PreN", wmPkg23, wmPortraitActivity},
	} {
		if err := func() error {
			testing.ContextLogf(ctx, "Running subtest %q", test.name)
			act, err := arc.NewActivity(a, test.pkg, test.activity)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				return err
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx)
			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			if ws, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
				return err
			} else if ws != ash.WindowStateNormal {
				return errors.Errorf("failed to set window state: got %s, want %s", ws, ash.WindowStateNormal)
			}

			// Clicking on "Immersive" button should not change the state of the restored window.
			if err := wm.UIClickImmersive(ctx, act, d); err != nil {
				return err
			}

			// TODO(crbug.com/1010469): This tries to verify that nothing changes, which is very hard.
			if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			return ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// wmPIP verifies that the activity enters into Picture In Picture mode when occluded as defined in:
// go/arc-wm-p "Clamshell: Picture in Picture" (slide #24)
func wmPIP(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// 1) Launch a PIP-ready activity in non-PIP mode.
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
	defer actPIP.Stop(ctx)
	if err := ash.WaitForVisible(ctx, tconn, actPIP.PackageName()); err != nil {
		return err
	}

	// TODO(crbug.com/1010469): This tries to verify that nothing changes, which is very hard.
	// PIP activity must not be in PIP mode yet.
	if ws, err := ash.GetARCAppWindowState(ctx, tconn, actPIP.PackageName()); err != nil {
		return err
	} else if ws == ash.WindowStatePIP {
		return errors.New("invalid window state: window started in PIP mode unexpectedly")
	}

	// 2) Launch a maximized application to make sure that it occludes the previous activity.
	testing.ContextLog(ctx, "Launching maximized activity")
	actOther, err := arc.NewActivity(a, wmPkg24, wmResizeableLandscapeActivity)
	if err != nil {
		return err
	}
	defer actOther.Close()
	if err := actOther.Start(ctx); err != nil {
		return err
	}
	defer actOther.Stop(ctx)
	if err := ash.WaitForVisible(ctx, tconn, actOther.PackageName()); err != nil {
		return err
	}

	// 3) Verify that the occluded activity is in PIP mode.
	return ash.WaitForARCAppWindowState(ctx, tconn, actPIP.PackageName(), ash.WindowStatePIP)
}

// wmFreeformResize verifies that a window can be resized as defined in:
// go/arc-wm-p "Clamshell: freeform resize" (slide #26)
func wmFreeformResize(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	act, err := arc.NewActivity(a, wmPkg24, wmResizeableLandscapeActivity)
	if err != nil {
		return err
	}
	defer act.Close()
	if err := act.Start(ctx); err != nil {
		return err
	}
	defer act.Stop(ctx)
	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	// N apps are launched as maximized. We grab the bounds from the maximized app, and we use those
	// bounds to resize the app when it is in restored mode.
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}
	maxBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}

	if ws, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
		return err
	} else if ws != ash.WindowStateNormal {
		return errors.Errorf("failed to set window state: got %s, want %s", ws, ash.WindowStateNormal)
	}

	// Now we grab the bounds from the restored app, and we try to resize it to its previous right margin.
	origBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}

	// The -1 is needed to prevent injecting a touch event outside bounds.
	right := maxBounds.Left + maxBounds.Width - 1
	testing.ContextLog(ctx, "Resizing app to right margin = ", right)
	to := coords.NewPoint(right, origBounds.Top+origBounds.Height/2)
	if err := act.ResizeWindow(ctx, arc.BorderRight, to, 500*time.Millisecond); err != nil {
		return err
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	// ResizeWindow() does not guarantee pixel-perfect resizing.
	// For this particular test, we are good as long as the window has been resized at least one pixel.
	if bounds.Width <= origBounds.Width {
		testing.ContextLogf(ctx, "Original bounds: %+v; resized bounds: %+v", origBounds, bounds)
		return errors.Errorf("invalid window width: got %d; want %d > %d", bounds.Width, bounds.Width, origBounds.Width)
	}
	return nil
}

// wmSnapping verifies that a window can be snapped as defined in:
// go/arc-wm-p "Clamshell: Snapping to half screen" (slide #27).
func wmSnapping(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer kb.Close()

	act, err := arc.NewActivity(a, wmPkg24, wmResizeableLandscapeActivity)
	if err != nil {
		return err
	}
	defer act.Close()
	if err := act.Start(ctx); err != nil {
		return err
	}
	defer act.Stop(ctx)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	maximizedBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	halfWidth := maximizedBounds.Width / 2

	// Snap to left edge.
	if err := kb.Accel(ctx, "Alt+["); err != nil {
		return err
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		snappedBounds, err := act.WindowBounds(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		// 1-pixel margin error in case of using an odd screen width.
		const errorMargin = 1
		if math.Abs(float64(snappedBounds.Width-halfWidth)) > errorMargin {
			return errors.Errorf("invalid window width: got %d, want Abs(%d - %d) <= 1", snappedBounds.Width, snappedBounds.Width, halfWidth)
		}
		if snappedBounds.Left != 0 {
			return errors.Errorf("invalid window origin: got %d, want 0", snappedBounds.Left)
		}
		state, err := ash.GetARCAppWindowState(ctx, tconn, wmPkg24)
		if err != nil {
			return testing.PollBreak(err)
		}
		if state != ash.WindowStateLeftSnapped {
			return errors.Errorf("invalid window state: got %s, want WindowStateNormal", state)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// wmDisplayResolution verifies that the Android resolution gets updated as defined in:
// go/arc-wm-p "Clamshell: display resolution change" (slides #28-#29).
func wmDisplayResolution(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	act, err := arc.NewActivity(a, wmPkg24, wmResizeableLandscapeActivity)
	if err != nil {
		return err
	}
	defer act.Close()
	if err := act.Start(ctx); err != nil {
		return err
	}
	defer act.Stop(ctx)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	disp, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return err
	}

	oldBounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Window bounds before changing display resolution: %+v", oldBounds)

	button := d.Object(ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.ID("org.chromium.arc.testapp.windowmanager:id/button_show"))
	if err := button.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}
	buttonBoundsOld, err := button.GetBounds(ctx)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Button bounds before changing display resolution: %+v", buttonBoundsOld)
	array := disp.AvailableDisplayZoomFactors
	testing.ContextLog(ctx, "Available zoom factors: ", array)
	newZoom := 0.
	// We are intersted in the first Zoom Factor different than 1.
	for _, z := range array {
		if z != 1 {
			newZoom = z
			break
		}
	}
	if newZoom == 0 {
		return errors.Errorf("invalid AvailableDisplayZoomFactors: got %v; want array with at least one value different than '1'", array)
	}

	testing.ContextLog(ctx, "Using display zoom factor = ", newZoom)
	if err := wm.ChangeDisplayZoomFactor(ctx, tconn, disp.ID, newZoom); err != nil {
		return err
	}
	// Restore original zoom factor on exit.
	defer wm.ChangeDisplayZoomFactor(ctx, tconn, disp.ID, disp.DisplayZoomFactor)

	// Polling until we get the expected value. But we don't PollBreak if we get an "unexpected" value, since
	// we might correctly get "unexpected" values from Android during the scale factor switch.
	return testing.Poll(ctx, func(ctx context.Context) error {
		// New bounds should be: old bounds / newZoom.
		// Since the zoom factor could use numbers like 1.100000023841858, we take rounding-error into account.
		// But error shouldn't be more than 1 pixel.
		const errorMargin = 1
		newBounds, err := act.WindowBounds(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		testing.ContextLogf(ctx, "Window bounds after changing display resolution: %+v", newBounds)

		expectedW := float64(oldBounds.Width) / newZoom
		if math.Abs(float64(newBounds.Width)-expectedW) > errorMargin {
			return errors.Errorf("invalid width: got %d, want: %v +/- %d", newBounds.Width, expectedW, errorMargin)
		}
		expectedH := float64(oldBounds.Height) / newZoom
		if math.Abs(float64(newBounds.Height)-expectedH) > errorMargin {
			return errors.Errorf("invalid height: got %d, want: %v +/- %d", newBounds.Height, expectedH, errorMargin)
		}

		buttonBoundsNew, err := button.GetBounds(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		testing.ContextLogf(ctx, "Button bounds after changing display resolution: %+v", buttonBoundsNew)

		// It might be possible that the layout changed, changing the buttons' position.
		// But the buttons' size in DPs, and not pixels, should be the same.
		if buttonBoundsOld.Width != buttonBoundsNew.Width {
			return errors.Errorf("invalid button width: got %d, want %d", buttonBoundsNew.Width, buttonBoundsOld.Width)
		}
		if buttonBoundsOld.Height != buttonBoundsNew.Height {
			return errors.Errorf("invalid button height: got %d, want %d", buttonBoundsNew.Height, buttonBoundsOld.Height)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// wmPageZoom verifies that the Android zoom level gets updated as defined in:
// go/arc-wm-p "Clamshell: Page/content zoom" (slides #30-#31).
func wmPageZoom(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	act, err := arc.NewActivity(a, wmPkg24, wmResizeableLandscapeActivity)
	if err != nil {
		return err
	}
	defer act.Close()
	if err := act.Start(ctx); err != nil {
		return err
	}
	defer act.Stop(ctx)

	if err := wm.WaitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
		return err
	}

	zoom, err := settings.DefaultZoom(ctx, tconn)
	if err != nil {
		return err
	}
	testing.ContextLog(ctx, "Default zoom is: ", zoom)

	boundsOld, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Window bounds before changing page zoom: %+v", boundsOld)

	button := d.Object(ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.ID("org.chromium.arc.testapp.windowmanager:id/button_show"))
	if err := button.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}
	buttonBoundsOld, err := button.GetBounds(ctx)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Button bounds before changing page zoom: %+v", buttonBoundsOld)

	// Chrome has zoom factors that go from 0.25x to 5x. But Android only supports zoom factors
	// from 0.8x to 1.5x. Android clamps values that are outside the valid range. E.g: a 2.0 zoom factor
	// will be "transformed" to 1.5x in Android.
	const zoomFactor = 1.5
	testing.ContextLog(ctx, "Setting page zoom to ", zoomFactor)
	if err := settings.SetDefaultZoom(ctx, tconn, zoomFactor); err != nil {
		return err
	}
	// Be nice and restore default zoom on exit.
	defer settings.SetDefaultZoom(ctx, tconn, zoom)

	if zoom, err := settings.DefaultZoom(ctx, tconn); err != nil {
		return err
	} else if zoom != zoomFactor {
		// Checking that zoomFactor got applied correctly and was not converted to any another value.
		return errors.Errorf("unexpected zoom value: got %f, want %f", zoom, zoomFactor)
	}

	// After changing the zoom factor, the Android window size should remain the same.
	// But the content should be adjusted to the new zoom factor.
	return testing.Poll(ctx, func(ctx context.Context) error {
		boundsNew, err := act.WindowBounds(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}
		if boundsNew != boundsOld {
			return errors.Errorf("invalid window bounds: got %v, want %v", boundsNew, boundsOld)
		}

		buttonBoundsNew, err := button.GetBounds(ctx)
		if err != nil {
			return testing.PollBreak(err)
		}

		// Widget size should be bigger by zoomFactor +/- some error margin to deal with rounding issues.
		const errorMargin = zoomFactor * 0.1
		wIncrease := float64(buttonBoundsNew.Width) / float64(buttonBoundsOld.Width)
		hIncrease := float64(buttonBoundsNew.Height) / float64(buttonBoundsOld.Height)

		if math.Abs(wIncrease-zoomFactor) > errorMargin {
			return errors.Errorf("invalid button width increase: got %v; want value in [%v, %v] range",
				wIncrease, zoomFactor-errorMargin, zoomFactor+errorMargin)
		}

		if math.Abs(hIncrease-zoomFactor) > errorMargin {
			return errors.Errorf("invalid button height increase: got %v; want value in [%v, %v] range",
				hIncrease, zoomFactor-errorMargin, zoomFactor+errorMargin)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
