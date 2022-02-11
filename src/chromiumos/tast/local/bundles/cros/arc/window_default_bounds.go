// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowDefaultBounds,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test default window size behavior",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Fixture:      "arcBooted",
	})
}

func WindowDefaultBounds(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
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
		// TODO(crbug.com/1002958): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	// Reset WM state to default values.

	if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to clear task states: ", err)
	}

	s.Run(ctx, "Default size", func(ctx context.Context, s *testing.State) {
		if err := wmSystemDefaultHandling(ctx, tconn, a); err != nil {
			path := filepath.Join(s.OutDir(), "screenshot-default-size-failed-test.png")
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Fatal("subtest failed: ", err)
		}
	})

	s.Run(ctx, "Manifest specified size", func(ctx context.Context, s *testing.State) {
		if err := wmSpecifiedSizeHandling(ctx, tconn, a); err != nil {
			path := filepath.Join(s.OutDir(), "screenshot-specified-size-failed-test.png")
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Fatal("subtest failed: ", err)
		}
	})
}

// wmSystemDefaultHandling verifies that applications which use the metadata flag
// <meta-data android:name="WindowManagerPreference:FreeformWindowSize" android:value="system-default" />
// will restore to 80% of the screen size.
func wmSystemDefaultHandling(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	const (
		// wmSystemDefaultActivity denotes an activity which follows the 'new system default size style' of 80% screen size.
		wmSystemDefaultActivity = "org.chromium.arc.testapp.windowmanager.NewDefaultSizeActivity"
		// wmNormalDefaultActivity denotes an activity which follows the 'normal restore size style' of phone size.
		wmNormalDefaultActivity = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
	)

	// wmSizeTestFunc represents a function that tests if the window has a certain size.
	type wmSizeTestFunc func(context.Context, *chrome.TestConn, *arc.Activity) error

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
			act, err := arc.NewActivity(a, wm.Pkg24, test.act)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
				return err
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx, tconn)

			if err := compareWindowState(ctx, act, arc.WindowStateMaximized); err != nil {
				return err
			}

			if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventNormal); err != nil {
				return err
			}

			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
				return err
			}

			if err := test.wantedRestoredState(ctx, tconn, act); err != nil {
				return err
			}

			if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventMaximize); err != nil {
				return err
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
				return err
			}

			return compareWindowState(ctx, act, arc.WindowStateMaximized)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// checkPhoneSizeRestored checks that the window is in restored size portrait sized phone size.
func checkPhoneSizeRestored(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity) error {
	if err := compareWindowState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	_, workArea, err := screenSizeAndInternalWorkArea(ctx, tconn)
	if err != nil {
		return err
	}
	if err := checkCentered(bounds, *workArea); err != nil {
		return err
	}

	if bounds.Width >= bounds.Height {
		return errors.Errorf("the phone sized window is not portrait sized: got (%d, %d)", bounds.Width, bounds.Height)
	}
	// We could consider checking now the phone window size (currently 412dp, 732dp).
	// However - beside the fact that this gets changed once in a while by UX +
	// there is a chance that the window gets cropped on low res devices. As such a
	// direct test for the size does not seem to be important enough.
	// => For now we are happy to simply see that it is portrait sized.
	return nil
}

// check80PercentRestored checks that the window has 80% of the screen size in the restored state.
func check80PercentRestored(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity) error {
	if err := compareWindowState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}
	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		return err
	}
	screen, workArea, err := screenSizeAndInternalWorkArea(ctx, tconn)
	if err != nil {
		return err
	}
	if err := checkCentered(bounds, *workArea); err != nil {
		return err
	}
	const (
		// defaultSizePercentage is the size of a restored window in percents of the screen size.
		defaultSizePercentage = 80.0
		// epsilonFractionInPercent is the allowable derivation of the screensize in percent for the new default size handling.
		epsilonFractionInPercent = 2.0
	)

	// Check that the size is ~80% of the screen size (not the work space).
	deltaFractionX := math.Abs(defaultSizePercentage - 100.0*float64(bounds.Width)/float64(screen.Width))
	if deltaFractionX > epsilonFractionInPercent {
		return errors.Errorf("the width of the window diverts too much: got %f%%; wants <= %f%%", deltaFractionX, defaultSizePercentage)
	}

	deltaFractionY := math.Abs(defaultSizePercentage - 100.0*float64(bounds.Height)/float64(screen.Height))
	if deltaFractionY > epsilonFractionInPercent {
		return errors.Errorf("the height of the window diverts too much: got %f%%; wants <= %f%%", deltaFractionY, defaultSizePercentage)
	}
	return nil
}

// wmSpecifiedSizeHandling verifies that applications which sets the layout parameter
// <layout android:defaultWidth="XXX" android:defaultHeight="XXX" />
// will be launched with the specified size.
func wmSpecifiedSizeHandling(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	const (
		wmSizeSpecifiedActivity = "org.chromium.arc.testapp.windowmanager.SizeSpecifiedActivity"
		epsilon                 = 2 // used to compare obtained bounds size and expected size in DP.
	)

	act, err := arc.NewActivity(a, wm.Pkg24, wmSizeSpecifiedActivity)
	if err != nil {
		return err
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		return err
	}
	defer act.Stop(ctx, tconn)

	density, err := act.DisplayDensity(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get physical display density")
	}

	if err := compareWindowState(ctx, act, arc.WindowStateNormal); err != nil {
		return err
	}

	bounds, err := act.SurfaceBounds(ctx)
	if err != nil {
		return err
	}

	got := coords.ConvertBoundsFromPXToDP(bounds, density).Size()
	want := coords.Size{Width: 600, Height: 500} // in DP, specified in AndroidManifest.xml
	if !similarSize(got, want, epsilon) {
		return errors.Errorf("the activity doesn't have an expected size: got %+v; want %+v", got, want)
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
		return errors.Errorf("invalid window state: got %v; want %v", state, wanted)
	}
	return nil
}

// screenSizeAndInternalWorkArea returns the screen size and the workspace in pixels of the currently selected internal display.
func screenSizeAndInternalWorkArea(ctx context.Context, tconn *chrome.TestConn) (*coords.Size, *coords.Rect, error) {
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		// This could be fizz which does not have an internal screen.
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to get any display info")
		}
		for i := range infos {
			if infos[i].IsPrimary {
				dispInfo = &infos[i]
				break
			}
		}
		if dispInfo == nil {
			return nil, nil, errors.New("failed to get any display info")
		}
		testing.ContextLog(ctx, "Could not get an internal display. Trying with the primary one")
	}

	mode, err := dispInfo.GetSelectedMode()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get selected display mode")
	}

	displaySize := coords.ConvertBoundsFromDPToPX(dispInfo.Bounds, mode.DeviceScaleFactor).Size()
	workArea := coords.ConvertBoundsFromDPToPX(dispInfo.WorkArea, mode.DeviceScaleFactor)
	return &displaySize, &workArea, nil
}

// checkCentered is checking that a given rectangle is (roughly) in the middle of the screen.
// We cannot do an exact job here as we might see rounding issues in X because of dp/px translations.
// For Y we have the additional problem that the caption height is unknown to Android in Pi
// as it is not part of the window, and Android will guess a height.
func checkCentered(bounds, workArea coords.Rect) error {
	const (
		// screenCenterVerticalEpsilon is the allowable epsilon for rounding of the vertical center derivation from the screen center.
		screenCenterVerticalEpsilon = 3
		// screenCenterHorizontalEpsilon same as above only horizontal - we need to allow for a caption height delta between Chrome and Android.
		screenCenterHorizontalEpsilon = 25
	)

	deltaX := int(math.Abs((float64(bounds.Left) + float64(bounds.Width)/2.0 - (float64(workArea.Left) + float64(workArea.Width)/2.0))))
	if deltaX > screenCenterVerticalEpsilon {
		return errors.Errorf("window is not horizontally centered: got %dpx; want less than %dpx", deltaX, screenCenterVerticalEpsilon)
	}

	deltaY := int(math.Abs((float64(bounds.Top) + float64(bounds.Height)/2.0) - (float64(workArea.Top) + float64(workArea.Height)/2.0)))
	if deltaY > screenCenterHorizontalEpsilon {
		return errors.Errorf("window is not vertically not centered: got %dpx; want less than %dpx", deltaY, screenCenterHorizontalEpsilon)
	}

	// This expects that the caption is not part of the window (P case, might not be true for R).
	if bounds.Top < 0 {
		return errors.Errorf("a window should never go negative, making the caption inaccessible: got %d", bounds.Top)
	}

	if bounds.Height >= workArea.Height || bounds.Width >= workArea.Width {
		return errors.Errorf("a window should never be bigger than the workspace: got (%d, %d); wants <= (%d, %d)", bounds.Width, bounds.Height, workArea.Width, workArea.Height)
	}

	return nil
}

// similarSize compares two coords.Size whether they are similar by epsilon.
func similarSize(l, r coords.Size, epsilon int) bool {
	abs := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	return abs(l.Width-r.Width) <= epsilon && abs(l.Height-r.Height) <= epsilon
}
