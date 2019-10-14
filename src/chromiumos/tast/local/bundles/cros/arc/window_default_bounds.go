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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// Apk to test (we use the default WM test app for SDK 24 (N)
	wmPkg = "org.chromium.arc.testapp.windowmanager24"

	// wmSystemDefaultActivity denotes an activity which follows the 'new system default size style' of 80% screen size.
	wmSystemDefaultActivity = "org.chromium.arc.testapp.windowmanager.NewDefaultSizeActivity"
	// wmNormalDefaultActivity denotes an activity which follows the 'normal restore size style' of phone size.
	wmNormalDefaultActivity = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
	// screenCenterVerticalEpsilon is the allowable epsilon for rounding of the vertical center derivation from the screen center.
	screenCenterVerticalEpsilon = 3
	// screenCenterHorizontalEpsilon same as above only horizontal - we need to allow for a caption height delta between Chrome and Android.
	screenCenterHorizontalEpsilon = 20
	// defaultSizePercentage is the size of a restored window in percents of the screen size.
	defaultSizePercentage = 80.0
	// epsilonFractionInPercent is the allowable derivation of the screensize in percent for the new default size handling.
	epsilonFractionInPercent = 2.0
)

// wmTestStateFunc represents a function that tests if the window is in a certain state.
type wmSizeTestFunc func(context.Context, *chrome.Conn, *arc.Activity, *ui.Device) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowDefaultBounds,
		Desc:         "Test default window size behavior",
		Contacts:     []string{"skuhne@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

const pkg = "org.chromium.arc.testapp.windowmanager24"

func WindowDefaultBounds(ctx context.Context, s *testing.State) {
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

	if err := a.Install(ctx, s.DataPath("ArcWMTestApp_24.apk")); err != nil {
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
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}
		// TODO(ricardoq): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	// Reset WM state to default values.
	if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
		s.Fatal("Failed to clear task states: ", err)
	}

	if err := wmSystemDefaultHandling(ctx, tconn, a, d); err != nil {
		path := fmt.Sprintf("%s/screenshot-default-size-failed-test.png", s.OutDir())
		if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
			s.Log("Failed to capture screenshot: ", err)
		}
		s.Error("Default size test failed: ", err)
	}
}

// wmSystemDefaultHandling verifies that applications which use the metadata flag
// <meta-data android:name="WindowManagerPreference:FreeformWindowSize" android:value="system-default" />
// will restore to 80% of the screen size.
func wmSystemDefaultHandling(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name                string
		act                 string
		wantedRestoredState wmSizeTestFunc
	}{
		{"NormalSizeWindow", wmNormalDefaultActivity, checkPhoneSizeRestored},
		{"SystemDefaultSizeWindow", wmSystemDefaultActivity, check80PercentRestored},
	} {
		if err := func() error {
			testing.ContextLogf(ctx, "Running subtest %q", test.name)
			act, err := arc.NewActivity(a, wmPkg, test.act)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				return err
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx)

			if err := checkMaximized(ctx, act, d); err != nil {
				return err
			}

			if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
				return err
			}

			if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
				return err
			}

			if err := test.wantedRestoredState(ctx, tconn, act, d); err != nil {
				return err
			}

			if err := act.SetWindowState(ctx, arc.WindowStateMaximized); err != nil {
				return err
			}
			if err := act.WaitForIdle(ctx, 10*time.Second); err != nil {
				return err
			}

			return checkMaximized(ctx, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// checkPhoneSizeRestored checks that the window is in restored size portrait sized phone size.
func checkPhoneSizeRestored(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := compareWindowState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	_, _, workArea, err := getScreenSizeAndInternalWorkArea(ctx, tconn)
	if err != nil {
		return err
	}
	if err := checkCentered(bounds, workArea); err != nil {
		return err
	}

	if bounds.Width >= bounds.Height {
		return errors.New("the phone sized window is not portrait sized")
	}
	// We could consider checking now the phone window size (currently 412dp, 732dp).
	// However - beside the fact that this gets changed once in a while by UX +
	// there is a chance that the window gets cropped on low res devices. As such a
	// direct test for the size does not seem to be important enough.
	// => For now we are happy to simply see that it is portrait sized.
	return nil
}

// check80PercentRestored checks that the window has 80% of the screen size in the restored state.
func check80PercentRestored(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device) error {
	if err := compareWindowState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	screenWidth, screenHeight, workArea, err := getScreenSizeAndInternalWorkArea(ctx, tconn)
	if err != nil {
		return err
	}
	if err := checkCentered(bounds, workArea); err != nil {
		return err
	}

	// Check that the size is ~80% of the screen size (not the work space).
	deltaFractionX := math.Abs(defaultSizePercentage - 100.0*float64(bounds.Width)/float64(screenWidth))
	if deltaFractionX > epsilonFractionInPercent {
		return errors.Errorf("the width of the window diverts %f percent from the %f percent size", deltaFractionX, defaultSizePercentage)
	}

	deltaFractionY := math.Abs(defaultSizePercentage - 100.0*float64(bounds.Height)/float64(screenHeight))
	if deltaFractionY > epsilonFractionInPercent {
		return errors.Errorf("the height of the window diverts %f percent from the %f percent size", deltaFractionY, defaultSizePercentage)
	}
	return nil
}

// compareWindowState compares the activity window state with the wanted one.
// Returns nil only if they are equal.
func compareWindowState(ctx context.Context, act *arc.Activity, wanted arc.WindowState) error {
	state, err := act.GetWindowState(ctx)
	if err != nil {
		return err
	}
	if state != wanted {
		return errors.Errorf("invalid window state %v, want %v", state, wanted)
	}
	return nil
}

// getScreenSizeAndInternalWorkArea returns the screen size and the workspace in pixels of the currently selected internal display.
func getScreenSizeAndInternalWorkArea(ctx context.Context, tconn *chrome.Conn) (int, int, arc.Rect, error) {
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return 0, 0, arc.Rect{}, errors.Wrap(err, "failed to get internal display info")
	}

	for _, mode := range dispInfo.Modes {
		if mode.IsSelected {
			return mode.WidthInNativePixels, mode.HeightInNativePixels, arc.Rect{
				int(math.Round(float64(dispInfo.WorkArea.Left) * mode.DeviceScaleFactor)),
				int(math.Round(float64(dispInfo.WorkArea.Top) * mode.DeviceScaleFactor)),
				int(math.Round(float64(dispInfo.WorkArea.Width) * mode.DeviceScaleFactor)),
				int(math.Round(float64(dispInfo.WorkArea.Height) * mode.DeviceScaleFactor))}, nil
		}
	}
	return 0, 0, arc.Rect{}, errors.New("failed to get the selected screen mode")
}

// checkMaximized checks that the window is both maximized and resizeable.
func checkMaximized(ctx context.Context, act *arc.Activity, d *ui.Device) error {
	if err := compareWindowState(ctx, act, arc.WindowStateMaximized); err != nil {
		return err
	}
	return nil
}

// checkCentered is checking that a given rectangle is (roughly) in the middle of the screen.
// We cannot do an exact job here as we might see rounding issues in X because of dp/px translations.
// For Y we have the additional problem that the caption height is unknown to Android in NYC and PI
// as it is not part of the window, and Android will guess a height.
func checkCentered(bounds arc.Rect, workArea arc.Rect) error {
	deltaX := math.Abs(float64((bounds.Left + bounds.Width/2) - (workArea.Left + workArea.Width/2)))
	if deltaX > screenCenterVerticalEpsilon {
		return errors.Errorf("window is not horizontally centered - delta=%f", deltaX)
	}

	deltaY := math.Abs(float64((bounds.Top + bounds.Height/2) - (workArea.Top + workArea.Height/2)))
	if deltaY > screenCenterHorizontalEpsilon {
		return errors.Errorf("window is not vertically not centered - delta=%f", deltaY)
	}

	// This expects that the caption is not part of the window (NYC/P case, might not be true for R)
	if bounds.Top <= 0 {
		return errors.New("a window should never go negative, making the caption inaccessible")
	}

	if bounds.Height >= workArea.Height || bounds.Width >= workArea.Width {
		return errors.New("a window should never be bigger than the workspace")
	}

	return nil
}
