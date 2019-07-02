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
	// Apk compiled against target SDK 24 (N)
	wmPkg24 = "org.chromium.arc.testapp.windowmanager24"

	// Different activities used by the subtests.
	wmResizeableLandscapeActivity      = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"
	wmNonResizeableLandscapeActivity   = "org.chromium.arc.testapp.windowmanager.NonResizeableLandscapeActivity"
	wmResizeableUnspecifiedActivity    = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
	wmNonResizeableUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.NonResizeableUnspecifiedActivity"
	wmResizeablePortraitActivity       = "org.chromium.arc.testapp.windowmanager.ResizeablePortraitActivity"
	wmNonResizeablePortraitActivity    = "org.chromium.arc.testapp.windowmanager.NonResizeablePortraitActivity"

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

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowManagerCUJ,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-p",
		Contacts:     []string{"ricardoq@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func WindowManagerCUJ(ctx context.Context, s *testing.State) {
	const apk24 = "ArcWMTestApp_24.apk"

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

	if err := a.Install(ctx, s.DataPath(apk24)); err != nil {
		s.Fatal("Failed installing app: ", err)
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

// Helper functions

func checkMaximizeResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateMaximized); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmRestore, wmClose}
	return compareCaption(ctx, act, d, caption)
}

func checkMaximizeNonResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateMaximized); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

func checkRestoreResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWMState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmMaximize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

func checkPillarboxNonResizeable(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := checkPillarbox(ctx, act, d); err != nil {
		return err
	}
	caption := []string{wmBack, wmMinimize, wmClose}
	return compareCaption(ctx, act, d, caption)
}

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
