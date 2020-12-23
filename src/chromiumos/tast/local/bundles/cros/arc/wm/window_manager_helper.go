// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wm provides Window Manager Helper functions.
package wm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// Pkg23 Apk compiled against target SDK 23 (Pre-N)
	Pkg23 = "org.chromium.arc.testapp.windowmanager23"
	// Pkg24 Apk compiled against target SDK 24 (N)
	Pkg24 = "org.chromium.arc.testapp.windowmanager24"
	// Pkg24Secondary is used for the tests that need to run two ARC activities at the same time.
	Pkg24Secondary = "org.chromium.arc.testapp.windowmanager24.inmaximizedlist"

	// APKNameArcWMTestApp23 APK name for ArcWMTestApp_23.apk
	APKNameArcWMTestApp23 = "ArcWMTestApp_23.apk"

	// APKNameArcWMTestApp24 APK name for ArcWMTestApp_24.apk
	APKNameArcWMTestApp24 = "ArcWMTestApp_24.apk"

	// APKNameArcPipSimpleTastTest APK name for ArcPipSimpleTastTest.apk
	APKNameArcPipSimpleTastTest = "ArcPipSimpleTastTest.apk"

	// APKNameArcWMTestApp24Secondary APK name for ArcWMTestApp_24_InMaximizedList.apk.
	APKNameArcWMTestApp24Secondary = "ArcWMTestApp_24_InMaximizedList.apk"

	// ResizableLandscapeActivity used by the subtests.
	ResizableLandscapeActivity = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"
	// NonResizableLandscapeActivity used by the subtests.
	NonResizableLandscapeActivity = "org.chromium.arc.testapp.windowmanager.NonResizeableLandscapeActivity"
	// ResizableUnspecifiedActivity used by the subtests.
	ResizableUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
	// NonResizableUnspecifiedActivity used by the subtests.
	NonResizableUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.NonResizeableUnspecifiedActivity"
	// ResizablePortraitActivity used by the subtests.
	ResizablePortraitActivity = "org.chromium.arc.testapp.windowmanager.ResizeablePortraitActivity"
	// NonResizablePortraitActivity used by the subtests.
	NonResizablePortraitActivity = "org.chromium.arc.testapp.windowmanager.NonResizeablePortraitActivity"
	// LandscapeActivity used by the subtests.
	LandscapeActivity = "org.chromium.arc.testapp.windowmanager.LandscapeActivity"
	// UnspecifiedActivity used by the subtests.
	UnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.UnspecifiedActivity"
	// PortraitActivity used by the subtests.
	PortraitActivity = "org.chromium.arc.testapp.windowmanager.PortraitActivity"

	// Landscape and Portrait constraints come from:
	// http://cs/android/vendor/google_arc/packages/development/ArcWMTestApp/src/org/chromium/arc/testapp/windowmanager/BaseActivity.java?l=411
	// Landscape used by the subtests.
	Landscape = "landscape"
	// Portrait used by the subtests.
	Portrait = "portrait"

	// TimeReservedForStop is the time that is reserved to stop an activity after the execution is complete.
	TimeReservedForStop = 500 * time.Millisecond
	// RotationAnimationDuration is the time to wait for an animation to complete.
	RotationAnimationDuration = 750 * time.Millisecond
)

// CheckFunc represents a function that checks certain criteria for tests.
type CheckFunc func(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error

// TestFunc represents a function that tests if the window is in a certain state.
type TestFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device) error

// TestCase represents a struct for test names and their func.
type TestCase struct {
	Name string
	Func TestFunc
}

// CheckMaximizeResizable checks that the window is both maximized and resizable.
func CheckMaximizeResizable(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonMaximizeAndRestore | ash.CaptionButtonClose
	return CompareCaption(ctx, tconn, act.PackageName(), wanted)
}

// CheckMaximizeNonResizable checks that the window is both maximized and not resizable.
func CheckMaximizeNonResizable(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonClose
	return CompareCaption(ctx, tconn, act.PackageName(), wanted)
}

// CheckRestoreResizable checks that the window is both in restore mode and is resizable.
func CheckRestoreResizable(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonMaximizeAndRestore | ash.CaptionButtonClose
	return CompareCaption(ctx, tconn, act.PackageName(), wanted)
}

// CheckPillarboxResizable checks that the window is both in pillar-box mode and is resizable.
func CheckPillarboxResizable(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := CheckPillarbox(ctx, tconn, act, d); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonMaximizeAndRestore | ash.CaptionButtonClose
	return CompareCaption(ctx, tconn, act.PackageName(), wanted)
}

// CheckPillarboxNonResizable checks that the window is both in pillar-box mode and is not resizable.
func CheckPillarboxNonResizable(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := CheckPillarbox(ctx, tconn, act, d); err != nil {
		return err
	}
	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonClose
	return CompareCaption(ctx, tconn, act.PackageName(), wanted)
}

// CheckPillarbox checks that the window is in pillar-box mode.
func CheckPillarbox(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		return err
	}

	const wanted = Portrait
	o, err := UIOrientation(ctx, act, d)
	if err != nil {
		return err
	}
	if o != wanted {
		return errors.Errorf("invalid orientation %v; want %v", o, wanted)
	}

	return nil
}

// CheckMaximizeToFullscreenToggle checks window's bounds transisionning from max to fullscreen.
func CheckMaximizeToFullscreenToggle(ctx context.Context, tconn *chrome.TestConn, maxWindowCoords coords.Rect, fullscreenWindow ash.Window) error {
	if maxWindowCoords.Left != fullscreenWindow.TargetBounds.Left ||
		maxWindowCoords.Top != fullscreenWindow.TargetBounds.Top ||
		maxWindowCoords.Width != fullscreenWindow.TargetBounds.Width ||
		maxWindowCoords.Height >= fullscreenWindow.TargetBounds.Height {
		return errors.Errorf("invalid fullscreen window bounds compared to maximize window bounds, got: %s, want bigger than: %s", fullscreenWindow.TargetBounds, maxWindowCoords)
	}

	if fullscreenWindow.IsFrameVisible {
		return errors.Errorf("invalid frame visibility, got: %t, want: false", fullscreenWindow.IsFrameVisible)
	}

	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.New("failed to get display info")
	}
	if primaryDisplayInfo == nil {
		return errors.New("no primary display info found")
	}

	if primaryDisplayInfo.WorkArea.Top != fullscreenWindow.TargetBounds.Top ||
		primaryDisplayInfo.WorkArea.Height != fullscreenWindow.TargetBounds.Height {
		return errors.Errorf("invalid fullscreen window bounds compared to display work area, got: Top=%d, Height=%d, want: Top=%d, Height=%d", fullscreenWindow.TargetBounds.Top, fullscreenWindow.TargetBounds.Height, primaryDisplayInfo.WorkArea.Top, primaryDisplayInfo.WorkArea.Height)
	}

	return nil
}

// CheckMaximizeWindowInTabletMode checks the activtiy covers display's work area in maximize mode.
func CheckMaximizeWindowInTabletMode(ctx context.Context, tconn *chrome.TestConn, maximizeWindow ash.Window) error {
	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.New("failed to get display info")
	}
	if primaryDisplayInfo == nil {
		return errors.New("no primary display info found")
	}

	if maximizeWindow.IsFrameVisible {
		return errors.Errorf("invalid frame visibility, got: %t, want: false", maximizeWindow.IsFrameVisible)
	}

	if primaryDisplayInfo.WorkArea.Left != maximizeWindow.TargetBounds.Left ||
		primaryDisplayInfo.WorkArea.Top != maximizeWindow.TargetBounds.Top ||
		primaryDisplayInfo.WorkArea.Width != maximizeWindow.TargetBounds.Width ||
		primaryDisplayInfo.WorkArea.Height != maximizeWindow.TargetBounds.Height {
		return errors.Errorf("invalid maximize window bounds compared to display work area, got: %s, want: %s", maximizeWindow.TargetBounds, primaryDisplayInfo.WorkArea)
	}

	return nil
}

// WaitForShelfAnimationComplete waits for 5 seconds to shelf animation complete.
func WaitForShelfAnimationComplete(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		shelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
		if err != nil {
			return testing.PollBreak(err)
		}
		if shelfInfo.IsShelfWidgetAnimating {
			return errors.New("shelf is still animating")
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// WaitForARCAppWindowState waits for conditions to make sure ARC App window is in correct state and animation finished.
func WaitForARCAppWindowState(ctx context.Context, tconn *chrome.TestConn, windowState ash.WindowStateType, windowID int, isFrameVisible bool) error {
	if err := ash.WaitForARCAppWindowState(ctx, tconn, Pkg24, windowState); err != nil {
		return errors.Wrap(err, "failed to wait for ARC App window state")
	}
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return errors.Wrap(err, "failed to wait for window finish animating")
	}
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == windowID && w.IsFrameVisible == isFrameVisible
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for frame to get hidden")
	}

	return nil
}

// CompareCaption compares the activity caption buttons with the wanted one.
// Returns nil only if they are equal.
func CompareCaption(ctx context.Context, tconn *chrome.TestConn, pkgName string, wantedCaption ash.CaptionButtonStatus) error {
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

// WaitForDisplayOrientation waits for the display to rotate to the desired orientation.
func WaitForDisplayOrientation(ctx context.Context, tconn *chrome.TestConn, desiredOrientation display.OrientationType) error {
	rotationAngle := display.Rotate0
	if desiredOrientation == display.OrientationPortraitPrimary {
		rotationAngle = display.Rotate270
	}
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the display info")
	}
	return display.WaitForDisplayRotation(ctx, tconn, info.ID, rotationAngle)
}

// RotateDisplay rotates the screen by the given rotation angle. It returns a cleanup function that should be called to restore the device rotation to the original state.
func RotateDisplay(ctx context.Context, tconn *chrome.TestConn, angle display.RotationAngle) (func() error, error) {
	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return nil, err
	}

	if err := display.SetDisplayRotationSync(ctx, tconn, primaryDisplayInfo.ID, angle); err != nil {
		return nil, err
	}

	return func() error {
		return display.SetDisplayRotationSync(ctx, tconn, primaryDisplayInfo.ID, display.Rotate0)
	}, nil
}

// OrientationFromBounds returns orientation from the given bounds.
func OrientationFromBounds(bounds coords.Rect) string {
	if bounds.Height >= bounds.Width {
		return Portrait
	}
	return Landscape
}

// ToggleFullscreen toggles fullscreen by injecting the Zoom Toggle keycode.
func ToggleFullscreen(ctx context.Context, tconn *chrome.TestConn) error {
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
	if err := UIClick(ctx, d,
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

// UIOrientation returns the current orientation of the ArcWMTestApp window.
func UIOrientation(ctx context.Context, act *arc.Activity, d *ui.Device) (string, error) {
	s, err := getUIState(ctx, act, d)
	if err != nil {
		return "", err
	}
	return s.Orientation, nil
}

// UINumberActivities returns the number of activities present in the ArcWMTestApp stack.
func UINumberActivities(ctx context.Context, act *arc.Activity, d *ui.Device) (int, error) {
	s, err := getUIState(ctx, act, d)
	if err != nil {
		return 0, err
	}
	return s.ActivityNr, nil
}

// UIClick sends a "Click" message to an UI Object.
// The UI Object is selected from opts, which are the selectors.
func UIClick(ctx context.Context, d *ui.Device, opts ...ui.SelectorOption) error {
	obj := d.Object(opts...)
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return err
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "could not click on widget")
	}
	return nil
}

// UIClickUnspecified clicks on the "Unspecified" radio button that is present in the ArcWMTest activity.
func UIClickUnspecified(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.RadioButton"),
		ui.TextMatches("(?i)Unspecified")); err != nil {
		return errors.Wrap(err, "failed to click on Unspecified radio button")
	}
	return nil
}

// UIClickLandscape clicks on the "Landscape" radio button that is present in the ArcWMTest activity.
func UIClickLandscape(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.RadioButton"),
		ui.TextMatches("(?i)Landscape")); err != nil {
		return errors.Wrap(err, "failed to click on Landscape radio button")
	}
	return nil
}

// UIClickPortrait clicks on the "Portrait" radio button that is present in the ArcWMTest activity.
func UIClickPortrait(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.RadioButton"),
		ui.TextMatches("(?i)Portrait")); err != nil {
		return errors.Wrap(err, "failed to click on Portrait radio button")
	}
	return nil
}

// UIClickRootActivity clicks on the "Root Activity" checkbox that is present on the ArcWMTest activity.
func UIClickRootActivity(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.CheckBox"),
		ui.TextMatches("(?i)Root Activity")); err != nil {
		return errors.Wrap(err, "failed to click on Root Activity checkbox")
	}
	return nil
}

// UIClickImmersive clicks on the "Immersive" button that is present on the ArcWMTest activity.
func UIClickImmersive(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)Immersive")); err != nil {
		return errors.Wrap(err, "failed to click on Immersive button")
	}
	return nil
}

// UIClickNormal clicks on the "Normal" button that is present on the ArcWMTest activity.
func UIClickNormal(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)Normal")); err != nil {
		return errors.Wrap(err, "failed to click on Normal button")
	}
	return nil
}

// UIClickLaunchActivity clicks on the "Launch Activity" button that is present in the ArcWMTest activity.
func UIClickLaunchActivity(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.PackageName(act.PackageName()),
		ui.ClassName("android.widget.Button"),
		ui.TextMatches("(?i)Launch Activity")); err != nil {
		return errors.Wrap(err, "failed to click on Launch Activity button")
	}
	return d.WaitForIdle(ctx, 10*time.Second)
}

// UIWaitForRestartDialogAndRestart waits for the "Application needs to restart to resize" dialog.
// This dialog appears when a Pre-N application tries to switch between maximized / restored window states.
// See: http://cs/pi-arc-dev/frameworks/base/core/java/com/android/internal/policy/DecorView.java
func UIWaitForRestartDialogAndRestart(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := UIClick(ctx, d,
		ui.ClassName("android.widget.Button"),
		ui.ID("android:id/button1"),
		ui.TextMatches("(?i)Restart")); err != nil {
		return errors.Wrap(err, "failed to click on Restart button")
	}
	return d.WaitForIdle(ctx, 10*time.Second)
}

// WaitUntilActivityIsReady waits until the given activity is ready. The "wait" is performed both
// at the Ash and Android sides. Additionally, it waits until the "Refresh" button exists.
// act must be a "org.chromium.arc.testapp.windowmanager" activity, otherwise the "Refresh" button check
// will fail.
func WaitUntilActivityIsReady(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity, d *ui.Device) error {
	if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
		return err
	}
	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
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

// WaitUntilFrameMatchesCondition waits until the package's window has a frame that matches the given condition.
func WaitUntilFrameMatchesCondition(ctx context.Context, tconn *chrome.TestConn, pkgName string, visible bool, mode ash.FrameMode) error {
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

// ChangeDisplayZoomFactor changes the ChromeOS display zoom factor.
func ChangeDisplayZoomFactor(ctx context.Context, tconn *chrome.TestConn, dispID string, zoomFactor float64) error {
	p := display.DisplayProperties{DisplayZoomFactor: &zoomFactor}
	if err := display.SetDisplayProperties(ctx, tconn, dispID, p); err != nil {
		return errors.Wrap(err, "failed to set zoom factor")
	}
	return nil
}

// SetupAndRunTestCases sets up the environment for tests and runs testcases.
func SetupAndRunTestCases(ctx context.Context, s *testing.State, isTabletMode bool, testCases []TestCase) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	if err := a.Install(ctx, arc.APKPath(APKNameArcWMTestApp24)); err != nil {
		s.Fatal("Failed to install APK: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTabletMode)
	if err != nil {

		s.Fatal("Failed to ensure if tablet mode is ",
			func() string {
				if isTabletMode {
					return "enabled:"
				}
				return "disabled:"
			}(), err)
	}
	defer cleanup(ctx)

	for _, test := range testCases {
		s.Logf("Running test %q", test.Name)

		if err := test.Func(ctx, tconn, a, d); err != nil {
			path := fmt.Sprintf("%s/screenshot-cuj-failed-test-%s.png", s.OutDir(), test.Name)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("%s test failed: %v", test.Name, err)
		}
	}
}

// GetButtonBounds is used to get button bounds in a given package name.
func GetButtonBounds(ctx context.Context, d *ui.Device, actPkgName string) (coords.Rect, error) {
	// Get a buttons info.
	button := d.Object(ui.PackageName(actPkgName),
		ui.ClassName("android.widget.Button"),
		ui.ID("org.chromium.arc.testapp.windowmanager:id/button_show"))

	if err := button.WaitForExists(ctx, 10*time.Second); err != nil {
		return coords.Rect{}, err
	}
	buttonBounds, err := button.GetBounds(ctx)
	if err != nil {
		return coords.Rect{}, err
	}

	return buttonBounds, nil
}

// EnsureARCFontScaleChanged changes the android font scale via settings and waits until the font scale changes completely.
func EnsureARCFontScaleChanged(ctx context.Context, a *arc.ARC, fontScale float64) error {
	cmd := a.Command(ctx, "settings", "put", "system", "font_scale", fmt.Sprintf("%f", fontScale))
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "unable to run adb shell command")
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		fs, err := GetARCFontScale(ctx, a)
		if err != nil {
			return testing.PollBreak(err)
		}

		if fs != fontScale {
			return testing.PollBreak(errors.Errorf("unable to wait for font scale to change: got %f, want %f", fs, fontScale))
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// GetARCFontScale gets the font scale from the android settings.
func GetARCFontScale(ctx context.Context, a *arc.ARC) (float64, error) {
	cmd := a.Command(ctx, "settings", "get", "system", "font_scale")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, errors.Wrap(err, "unable to run adb shell command")
	}
	outStr := strings.TrimSpace(string(output))
	fs, err := strconv.ParseFloat(outStr, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid font_scale: %q", outStr)
	}

	return fs, nil
}

// CheckVerticalTabletSplit helps to assert window bounds in vertical split mode.
func CheckVerticalTabletSplit(ctx context.Context, tconn *chrome.TestConn, displayWorkArea coords.Rect) error {
	overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24Secondary)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}
	underActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for under activity")
	}
	// Over activity must be snapped to the left.
	if overActivityWInfo.BoundsInRoot.Left != 0 ||
		overActivityWInfo.BoundsInRoot.Top != 0 ||
		overActivityWInfo.BoundsInRoot.Width >= displayWorkArea.Width/2 ||
		overActivityWInfo.BoundsInRoot.Height != displayWorkArea.Height {
		return errors.Errorf("invalid snapped to the left activity bounds, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left = 0, Top = 0, Width < %d, Height = %d",
			overActivityWInfo.BoundsInRoot.Left, overActivityWInfo.BoundsInRoot.Top, overActivityWInfo.BoundsInRoot.Width, overActivityWInfo.BoundsInRoot.Height, displayWorkArea.Width/2, displayWorkArea.Height)
	}
	// Under activity must be snapped to the right.
	if underActivityWInfo.BoundsInRoot.Left <= displayWorkArea.Width/2 ||
		underActivityWInfo.BoundsInRoot.Top != 0 ||
		underActivityWInfo.BoundsInRoot.Width >= displayWorkArea.Width/2 ||
		underActivityWInfo.BoundsInRoot.Height != displayWorkArea.Height ||
		underActivityWInfo.BoundsInRoot.Left+underActivityWInfo.BoundsInRoot.Width != displayWorkArea.Width {
		return errors.Errorf("invalid snapped to the right activity bounds, got: Left = %d, Top = %d, Width = %d, Height = %d, Right = %d; want: Left > %d, Top = 0, Width < %d, Height = %d, Right = %d",
			underActivityWInfo.BoundsInRoot.Left, underActivityWInfo.BoundsInRoot.Top, underActivityWInfo.BoundsInRoot.Width, underActivityWInfo.BoundsInRoot.Height,
			underActivityWInfo.BoundsInRoot.Left+underActivityWInfo.BoundsInRoot.Width, displayWorkArea.Width/2, displayWorkArea.Width/2, displayWorkArea.Height, displayWorkArea.Width)
	}

	return nil
}

// CheckHorizontalTabletSplit helps to assert window bounds in horizontal split mode.
func CheckHorizontalTabletSplit(ctx context.Context, tconn *chrome.TestConn, displayWorkArea coords.Rect) error {
	underActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for under activity")
	}
	overActivityWInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, Pkg24Secondary)
	if err != nil {
		return errors.Wrap(err, "failed to get arc app window info for over activity")
	}

	// Over activity must be snapped to the top.
	if overActivityWInfo.BoundsInRoot.Left != 0 ||
		overActivityWInfo.BoundsInRoot.Top != 0 ||
		overActivityWInfo.BoundsInRoot.Width != displayWorkArea.Width ||
		overActivityWInfo.BoundsInRoot.Height >= displayWorkArea.Height/2 {
		return errors.Errorf("invalid snapped to the top activity bounds, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left = 0, Top = 0, Width = %d, Height < %d",
			overActivityWInfo.BoundsInRoot.Left, overActivityWInfo.BoundsInRoot.Top, overActivityWInfo.BoundsInRoot.Width, overActivityWInfo.BoundsInRoot.Height,
			displayWorkArea.Width, displayWorkArea.Height/2)
	}
	// Under activity must be snapped to the bottom.
	if underActivityWInfo.BoundsInRoot.Left != 0 ||
		underActivityWInfo.BoundsInRoot.Top <= displayWorkArea.Height/2 ||
		underActivityWInfo.BoundsInRoot.Width != displayWorkArea.Width ||
		underActivityWInfo.BoundsInRoot.Height >= displayWorkArea.Height/2 ||
		underActivityWInfo.BoundsInRoot.Top+underActivityWInfo.BoundsInRoot.Height != displayWorkArea.Height {
		return errors.Errorf("invalid snapped to the bottom activity bounds, got: Left = %d, Top = %d, Width = %d, Height = %d; want: Left = 0, Top > %d, Width = %d, Height < %d",
			underActivityWInfo.BoundsInRoot.Left, underActivityWInfo.BoundsInRoot.Top, underActivityWInfo.BoundsInRoot.Width, underActivityWInfo.BoundsInRoot.Height,
			displayWorkArea.Height/2, displayWorkArea.Width, displayWorkArea.Height/2)
	}

	return nil
}
