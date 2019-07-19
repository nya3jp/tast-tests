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
	wmBack     = "back"
	wmClose    = "close"
	wmMaximize = "maximize"
	wmMinimize = "minimize"
	wmPortrait = "portrait"
	wmRestore  = "restore"
)

// wmTestStateFunc represents a function that tests if the window is in a certain state.
type wmTestStateFunc func(context.Context, *arc.Activity, *ui.Device) error

// uiClickFunc represents a function that "clicks" on a certain widget using UI Automator.
type uiClickFunc func(context.Context, *arc.Activity, *ui.Device) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowManagerCUJ,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-p",
		Contacts:     []string{"ricardoq@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcWMTestApp_23.apk", "ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
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

	for _, apk := range []string{"ArcWMTestApp_23.apk", "ArcWMTestApp_24.apk"} {
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

	type testFunc func(context.Context, *chrome.Conn, *arc.ARC, *ui.Device) error
	for idx, test := range []struct {
		name string
		fn   testFunc
	}{
		{"Default Launch Clamshell N", wmDefaultLaunchClamshell24},
		{"Default Launch Clamshell Pre-N", wmDefaultLaunchClamshell23},
		{"Maximize / Restore Clamshell N", wmMaximizeRestoreClamshell24},
		{"Maximize / Restore Clamshell Pre-N", wmMaximizeRestoreClamshell23},
		{"Follow Root Activity N / Pre-N", wmFollowRoot},
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

			if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
				return err
			}

			return test.wantedState(ctx, act, d)
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
		{"Unspecified", wmUnspecifiedActivity, checkRestoreResizeable},
		// Window #A.
		{"Portrait", wmPortraitActivity, checkRestoreResizeable},
		// Window #B.
		{"Landscape", wmLandscapeActivity, checkRestoreResizeable},
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

			if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
				return err
			}

			return test.wantedState(ctx, act, d)
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

			return test.wantedStateA(ctx, act, d)
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
		{"Landscape", wmLandscapeActivity, checkMaximizeResizeable},
		{"Unspecified", wmUnspecifiedActivity, checkMaximizeResizeable},
		{"Portrait", wmPortraitActivity, checkPillarboxResizeable},
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

			return checkRestoreResizeable(ctx, act, d)
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
			{"Unspecified", uiClickUnspecified},
			{"Landscape", uiClickLandscape},
			{"Portrait", uiClickPortrait},
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

				if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
					return err
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

				if s, err := act.GetWindowState(ctx); err != nil {
					return err
				} else if s != arc.WindowStateNormal {
					return errors.Errorf("invalid window state: got %q; want %q", s.String(), arc.WindowStateNormal.String())
				}

				if newOrientation, err := uiOrientation(ctx, act, d); err != nil {
					return err
				} else if newOrientation != origOrientation {
					return errors.Errorf("invalid orientation: got %q; want %q", newOrientation, origOrientation)
				}

				if nrActivities, err := uiNumberActivities(ctx, act, d); err != nil {
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

// Helper functions

// checkMaximizeResizeable checks that the window is both maximized and resizeable.
func checkMaximizeResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateMaximized); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmRestore, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkMaximizeNonResizeable checks that the window is both maximized and not resizeable.
func checkMaximizeNonResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateMaximized); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkRestoreResizeable checks that the window is both in restore mode and is resizeable.
func checkRestoreResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmMaximize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkPillarboxResizeable checks that the window is both in pillar-box mode and is resizeable.
func checkPillarboxResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := checkPillarbox(ctx, act, d); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmRestore, wmClose}
	return compareCaption(ctx, act, d, caption)
}

// checkPillarboxNonResizeable checks that the window is both in pillar-box mode and is not resizeable.
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

// compareWMState compares the activity window state with the wanted one.
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

// Helper UI functions
// These functions use UI Automator to get / change the state of ArcWMTest activity.

// uiState represents the state of ArcWMTestApp activity. See:
// http://cs/pi-arc-dev/vendor/google_arc/packages/development/ArcWMTestApp/src/org/chromium/arc/testapp/windowmanager/JsonHelper.java
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

// uiClicks sends a "Click" message to an UI Object.
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

// uiWaitForRestartDialogAndRestart waits for the "Application needs to restart to resize" dialog.
// This dialog appears when a Pre-N application tries to switch between maximized / restored window states.
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
