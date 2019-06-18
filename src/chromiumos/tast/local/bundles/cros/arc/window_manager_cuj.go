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

	// These values must match the strings from ArcWMTestApp defined in BaseActivity#parseCaptionButtons:
	// http://cs/android/vendor/google_arc/packages/development/ArcWMTestApp/src/org/chromium/arc/testapp/windowmanager/BaseActivity.java?l=448
	wmBack     = "back"
	wmMinimize = "minimize"
	wmMaximize = "maximize"
	wmRestore  = "restore"
	wmClose    = "close"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowManagerCUJ,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-p",
		Contacts:     []string{"ricardoq@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcWMTestApp_23.apk", "ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func WindowManagerCUJ(ctx context.Context, s *testing.State) {
	const (
		apk23 = "ArcWMTestApp_23.apk"
		apk24 = "ArcWMTestApp_24.apk"
	)

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
	if err := a.Install(ctx, s.DataPath(apk23)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	type testFunc func(context.Context, *chrome.Conn, *arc.ARC, *ui.Device) error
	for idx, test := range []struct {
		name string
		fn   testFunc
	}{
		// Ordered as they appear on go/arc-wm-p
		{"Default Launch Clamshell SDK24", wmDefaultLaunchClamshell24},
		{"Default Launch Clamshell SDK23", wmDefaultLaunchClamshell23},
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
	// Expected caption buttons.
	wmCaptionBMRC := []string{wmBack, wmMinimize, wmRestore, wmClose}
	wmCaptionBMMC := []string{wmBack, wmMinimize, wmMaximize, wmClose}
	wmCaptionBMC := []string{wmBack, wmMinimize, wmClose}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get tablet mode")
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "could not set tablet mode disabled")
	}

	for _, test := range []struct {
		name          string
		act           string
		wantedState   arc.WindowState
		wantedCaption []string
	}{
		// The are four possible default states (windows #A to #D) from six possible different activities.
		// Window #A.
		{"Landscape + Resize enabled", wmResizeableLandscapeActivity, arc.WindowStateMaximized, wmCaptionBMRC},
		// Window #B.
		{"Landscape + Resize disabled", wmNonResizeableLandscapeActivity, arc.WindowStateMaximized, wmCaptionBMC},
		// Window #A.
		{"Unspecified + Resize enabled", wmResizeableUnspecifiedActivity, arc.WindowStateMaximized, wmCaptionBMRC},
		// Window #B.
		{"Unspecified + Resize disabled", wmNonResizeableUnspecifiedActivity, arc.WindowStateMaximized, wmCaptionBMC},
		// Window #C.
		{"Portrait + Resized enabled", wmResizeablePortraitActivity, arc.WindowStateNormal, wmCaptionBMMC},
		// Window #D.
		// TODO(ricardoq): detect pillarbox mode.
		{"Portrait + Resize disabled", wmNonResizeablePortraitActivity, arc.WindowStateMaximized, wmCaptionBMC},
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

		state, err := act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		if state != test.wantedState {
			return errors.Errorf("invalid window state %v, want %v", state, test.wantedState)
		}

		bn, err := wmCaptionButtons(ctx, d)
		if err != nil {
			return errors.Wrap(err, "could not get caption buttons state")
		}
		if !reflect.DeepEqual(bn, test.wantedCaption) {
			return errors.Errorf("invalid caption buttons %+v, want %+v", bn, test.wantedCaption)
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
	// Expected caption buttons.
	wmCaptionBMMC := []string{wmBack, wmMinimize, wmMaximize, wmClose}
	wmCaptionBMC := []string{wmBack, wmMinimize, wmClose}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get tablet mode")
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "could not set tablet mode disabled")
	}

	for _, test := range []struct {
		name          string
		act           string
		wantedState   arc.WindowState
		wantedCaption []string
	}{
		// The are two possible default states (windows #A to #B) from three possible different activities.
		// Window #A.
		{"Unspecified", wmUnspecifiedActivity, arc.WindowStateNormal, wmCaptionBMMC},
		// Window #A.
		{"Portrait", wmPortraitActivity, arc.WindowStateNormal, wmCaptionBMMC},
		// Window #B.
		{"Landscape", wmLandscapeActivity, arc.WindowStateNormal, wmCaptionBMMC},
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

		state, err := act.GetWindowState(ctx)
		if err != nil {
			return err
		}
		if state != test.wantedState {
			return errors.Errorf("invalid window state %v, want %v", state, test.wantedState)
		}

		bn, err := wmCaptionButtons(ctx, d)
		if err != nil {
			return errors.Wrap(err, "could not get caption buttons state")
		}
		if !reflect.DeepEqual(bn, test.wantedCaption) {
			return errors.Errorf("invalid caption buttons %+v, want %+v", bn, test.wantedCaption)
		}
		// Stopping current activity in order to make it possible to launch a different from the same package.
		if err := act.Stop(ctx); err != nil {
			return errors.Wrapf(err, "could not stop activity %v", test.act)
		}
	}
	return nil
}

// wmCaptionButtons returns the caption buttons that are present in the ArcWMTestApp window.
func wmCaptionButtons(ctx context.Context, d *ui.Device) (buttons []string, err error) {
	s, err := getWMState(ctx, d)
	if err != nil {
		return nil, err
	}
	return s.Buttons, nil
}

// wmState represents the state of ArcWMTestApp activity.
type wmState struct {
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

// getWMState returns the state from the ArcWMTest activity.
// The state is taken by parsing the activity's TextView which contains the state in JSON format.
func getWMState(ctx context.Context, d *ui.Device) (*wmState, error) {
	obj := d.Object(ui.ClassName("android.widget.TextView"), ui.ResourceIDMatches(".+?(/caption_text_view)$"))
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return nil, err
	}
	s, err := obj.GetText(ctx)
	if err != nil {
		return nil, err
	}
	var state wmState
	if err := json.Unmarshal([]byte(s), &state); err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling state")
	}
	return &state, nil
}
