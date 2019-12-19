// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Window Manager CUJ Helper functions

// checkMaximizeResizeable checks that the window is both maximized and resizeable.
func checkMaximizeResizeable(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonMaximizeAndRestore | ash.CaptionButtonClose
	return compareCaption(ctx, tconn, act.PackageName(), wanted)
}

// checkMaximizeNonResizeable checks that the window is both maximized and not resizeable.
func checkMaximizeNonResizeable(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonClose
	return compareCaption(ctx, tconn, act.PackageName(), wanted)
}

// checkRestoreResizeable checks that the window is both in restore mode and is resizeable.
func checkRestoreResizeable(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonMaximizeAndRestore | ash.CaptionButtonClose
	return compareCaption(ctx, tconn, act.PackageName(), wanted)
}

// checkPillarboxResizeable checks that the window is both in pillar-box mode and is resizeable.
func checkPillarboxResizeable(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := checkPillarbox(ctx, tconn, act, d); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonMaximizeAndRestore | ash.CaptionButtonClose
	return compareCaption(ctx, tconn, act.PackageName(), wanted)
}

// checkPillarboxNonResizeable checks that the window is both in pillar-box mode and is not resizeable.
func checkPillarboxNonResizeable(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := checkPillarbox(ctx, tconn, act, d); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonClose
	return compareCaption(ctx, tconn, act.PackageName(), wanted)
}

// checkPillarbox checks that the window is in pillar-box mode.
func checkPillarbox(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}

	const wanted = wmPortrait
	o, err := uiOrientation(ctx, act, d)
	if err != nil {
		return err
	}
	if o != wanted {
		return errors.Errorf("invalid orientation %v; want %v", o, wanted)
	}

	return nil
}

// compareCaption compares the activity caption buttons with the wanted one.
// Returns nil only if they are equal.
func compareCaption(ctx context.Context, tconn *chrome.Conn, pkgName string, wantedCaption ash.CaptionButtonStatus) error {
	info, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return err
	}
	// We should compare both visible and enabled buttons.
	if info.CaptionButtonEnabledStatus != wantedCaption {
		return errors.Errorf("unexpected CaptionButtonEnabledStatus value: want %q, got %q",
			wantedCaption.String(), info.CaptionButtonEnabledStatus.String())
	}
	if info.CaptionButtonVisibleStatus != wantedCaption {
		return errors.Errorf("unexpected CaptionButtonVisibleStatus value: want %q, got %q",
			wantedCaption.String(), info.CaptionButtonVisibleStatus.String())
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

// uiState represents the state of ArcWMTestApp activity. See:
// http://cs/pi-arc-dev/vendor/google_arc/packages/development/ArcWMTestApp/src/org/chromium/arc/testapp/windowmanager/JsonHelper.java
type uiState struct {
	Orientation string      `json:"orientation"`
	ActivityNr  int         `json:"activityNr"`
	Rotation    int         `json:"rotation"`
	Accel       interface{} `json:"accel"`
}

// getUIState returns the state from the ArcWMTest activity.
// The state is taken by parsing the activity's TextView which contains the state in JSON format.
func getUIState(ctx context.Context, act *arc.Activity, d *ui.Device) (*uiState, error) {
	// Before fetching the UI data, click on "Refresh" button to make sure the data is updated.
	if err := uiClick(ctx, d,
		ui.ID("org.chromium.arc.testapp.windowmanager:id/button_refresh"),
		ui.ClassName("android.widget.Button")); err != nil {
		return nil, errors.Wrap(err, "failed to click on Refresh button")
	}

	// In case the application is still refreshing, let it finish before fetching the data.
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for idle")
	}

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

// uiClick sends a "Click" message to an UI Object.
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
	return act.WaitForResumed(ctx, 10*time.Second)
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
	return act.WaitForResumed(ctx, 10*time.Second)
}

// waitUntilActivityIsReady waits until the given activity is ready. The "wait" is performed both
// at the Ash and Android sides. Additionally, it waits until the "Refresh" button exists.
// act must be a "org.chromium.arc.testapp.windowmanager" activity, otherwise the "Refresh" button check
// will fail.
func waitUntilActivityIsReady(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
		return err
	}
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		return err
	}
	if err := act.WaitForResumed(ctx, 10*time.Second); err != nil {
		return err
	}
	obj := d.Object(
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.ID("org.chromium.arc.testapp.windowmanager:id/button_refresh"))
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}
	return nil
}

// waitUntilFrameMatchesCondition waits until the package's window has a frame that matches the given condition.
func waitUntilFrameMatchesCondition(ctx context.Context, tconn *chrome.Conn, pkgName string, visible bool, mode ash.FrameMode) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
		if err != nil {
			// The window may not yet be known to the Chrome side, so don't stop polling here.
			return errors.Wrap(err, "failed to get ARC window info")
		}

		if info.IsFrameVisible != visible {
			return errors.Errorf("unwanted window frame visibility: %t", info.IsFrameVisible)
		}

		if info.FrameMode != mode {
			return errors.Errorf("unwanted window frame mode: got %s, want %s", info.FrameMode, mode)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// commonWMSetup creates ui.Device, the caller is supposed to close that.
func commonWMSetUp(ctx context.Context, s *testing.State, apks []string) (*chrome.Conn, *ui.Device, error) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	for _, apk := range apks {
		if err := a.Install(ctx, s.DataPath(apk)); err != nil {
			return nil, nil, err
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, err
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return nil, nil, err
	}

	return tconn, d, nil
}

// restoreTabletSettings clean up possible changes caused by tests.
func restoreTabletSettings(tabletModeEnabled bool, ctx context.Context, tconn *chrome.Conn) error {
	if tabletModeEnabled {
		// Be nice and restore tablet mode to its original state on exit.
		ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			return err
		}
		// TODO(ricardoq): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return err
		}
	}

	return nil
}
