// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	arcscreenshot "chromiumos/tast/local/bundles/cros/arc/screenshot"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	// FirstExternalDisplayID represents the display ID for the first external display.
	firstExternalDisplayID = "1"

	// Apk compiled against target SDK 24 (N).
	wmPkgMD = "org.chromium.arc.testapp.windowmanager24"
	wmApkMD = "ArcWMTestApp_24.apk"

	settingsPkgMD = "com.android.settings"
	settingsActMD = ".Settings"

	// Different activities used by the subtests.
	nonResizeableUnspecifiedActivityMD = "org.chromium.arc.testapp.windowmanager.NonResizeableUnspecifiedActivity"
	resizeableUnspecifiedActivityMD    = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
)

// Power state for displays.
type displayPowerState int

// As defined in DisplayPowerState here:
// https://cs.chromium.org/chromium/src/third_party/cros_system_api/dbus/service_constants.h
const (
	displayPowerAllOn                 displayPowerState = 0
	displayPowerAllOff                displayPowerState = 1
	displayPowerInternalOffExternalOn displayPowerState = 2
	displayPowerInternalOnExternalOff displayPowerState = 3
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MultiDisplay,
		Desc:     "Mutli-display ARC window management tests",
		Contacts: []string{"ruanc@chromium.org", "niwa@chromium.org", "arc-framework+tast@google.com"},
		// TODO(ruanc): There is no hardware dependency for multi-display. Remove "disabled" attribute once it is supported.
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
	})
}

// MultiDisplay test requires two connected displays.
func MultiDisplay(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	displayInfos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}

	// TODO(ruanc): This part can be removed if hardware dependency for multi-display is available.
	if len(displayInfos) != 2 {
		s.Fatalf("Not enough connected displays: got %d; want 2", len(displayInfos))
	}

	if err := a.Install(ctx, s.DataPath(wmApkMD)); err != nil {
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

	type testFunc func(context.Context, *chrome.Chrome, *arc.ARC) error
	for idx, test := range []struct {
		name string
		fn   testFunc
	}{
		// Based on http://b/129564108.
		{"Launch activity on external display", launchActivityOnExternalDisplay},
		// Based on http://b/110105532.
		{"Activity is visible when other is maximized", maximizeVisibility},
		// Based on http://b/63773037 and http://b/140056612.
		{"Relayout displays", relayoutDisplays},
		// Based on http://b/130897153.
		{"Remove and re-add displays", removeAddDisplay},
	} {
		s.Logf("Running test %q", test.name)

		// Log test result.
		if err := test.fn(ctx, cr, a); err != nil {
			for _, info := range displayInfos {
				path := fmt.Sprintf("%s/screenshot-multi-display-failed-test-%d-%q.png", s.OutDir(), idx, info.ID)
				if err := screenshot.CaptureChromeForDisplay(ctx, cr, info.ID, path); err != nil {
					s.Logf("Failed to capture screenshot for display ID %q: %v", info.ID, err)
				}
			}
			s.Errorf("%q test failed: %v", test.name, err)
		}
	}
}

// launchActivityOnExternalDisplay launches the activity directly on the external display.
func launchActivityOnExternalDisplay(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}
	var externalDisplayID string
	for _, info := range infos {
		if !info.IsInternal {
			externalDisplayID = info.ID
		}
	}

	for _, test := range []struct {
		name    string
		actName string
	}{
		{"Launch resizeable activity on the external display", resizeableUnspecifiedActivityMD},
		{"Launch unresizeable activity on the external display", nonResizeableUnspecifiedActivityMD},
	} {
		if err := func() error {
			testing.ContextLogf(ctx, "Running subtest %q", test.name)
			act, err := arc.NewActivity(a, wmPkgMD, test.actName)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := startActivityOnDisplay(ctx, a, wmPkgMD, test.actName, firstExternalDisplayID); err != nil {
				return err
			}
			defer act.Stop(ctx)

			if err := act.WaitForResumed(ctx, 10*time.Second); err != nil {
				return err
			}
			return ensureWindowOnDisplay(ctx, tconn, wmPkgMD, externalDisplayID)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// maximizeVisibility checks whether the window is visible on one display if another window is maximized on the other display.
func maximizeVisibility(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	// Start settings activity and set it to normal window state.
	settingsAct, err := arc.NewActivity(a, settingsPkgMD, settingsActMD)
	if err != nil {
		return err
	}
	defer settingsAct.Close()

	if err := settingsAct.Start(ctx); err != nil {
		return err
	}
	defer settingsAct.Stop(ctx)
	if err := settingsAct.WaitForResumed(ctx, 10*time.Second); err != nil {
		return err
	}

	if err := ensureSetWindowState(ctx, tconn, settingsPkgMD, ash.WindowStateNormal); err != nil {
		return err
	}

	// Start WM activity on the external display and set it to normal window state.
	wmAct, err := arc.NewActivity(a, wmPkgMD, resizeableUnspecifiedActivityMD)
	if err != nil {
		return err
	}
	defer wmAct.Close()

	if err := startActivityOnDisplay(ctx, a, wmPkgMD, resizeableUnspecifiedActivityMD, firstExternalDisplayID); err != nil {
		return err
	}
	defer wmAct.Stop(ctx)
	if err := wmAct.WaitForResumed(ctx, 10*time.Second); err != nil {
		return err
	}

	// Get external display ID.
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}
	var extDispID string
	for _, info := range infos {
		if !info.IsInternal {
			extDispID = info.ID
			break
		}
	}

	if err := ensureWindowOnDisplay(ctx, tconn, wmPkgMD, extDispID); err != nil {
		return err
	}

	if err := ensureSetWindowState(ctx, tconn, wmPkgMD, ash.WindowStateNormal); err != nil {
		return err
	}

	// Preserve WindowInfo.
	wmWinInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wmPkgMD)
	if err != nil {
		return err
	}

	settingsWinInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, settingsPkgMD)
	if err != nil {
		return err
	}

	for _, test := range []struct {
		name       string
		maxAct     *arc.Activity
		maxPkgName string

		checkPkgName    string
		checkAppWinInfo *ash.Window
	}{
		{"Maximize the activity on primary display", settingsAct, settingsPkgMD, wmPkgMD, wmWinInfo},
		{"Maximize the activity on external display", wmAct, wmPkgMD, settingsPkgMD, settingsWinInfo},
	} {
		if err := func() error {
			if err := ensureSetWindowState(ctx, tconn, test.maxPkgName, ash.WindowStateMaximized); err != nil {
				return err
			}
			if err := ensureWindowStable(ctx, tconn, test.checkPkgName, test.checkAppWinInfo); err != nil {
				return err
			}
			// The black window shows when the activity is not visible on Android side (see: http://b/110105532).
			if err := ensureNoBlackBkg(ctx, cr, tconn); err != nil {
				return err
			}
			// Reset maximized window to normal.
			return ensureSetWindowState(ctx, tconn, test.maxPkgName, ash.WindowStateNormal)
		}(); err != nil {
			return errors.Wrapf(err, "subtest failed when: %q", test.name)
		}

	}
	return nil
}

// relayoutDisplays checks whether the window moves position when relayout displays.
func relayoutDisplays(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}

	var internalDisplayInfo, externalDisplayInfo display.Info
	for _, info := range infos {
		if info.IsInternal {
			internalDisplayInfo = info
		} else if externalDisplayInfo.ID == "" {
			// Get the first external display info.
			externalDisplayInfo = info
		}
	}

	// Start settings Activity on internal display.
	settingsAct, err := arc.NewActivity(a, settingsPkgMD, settingsActMD)
	if err != nil {
		return err
	}
	defer settingsAct.Close()

	if err := settingsAct.Start(ctx); err != nil {
		return err
	}
	defer settingsAct.Stop(ctx)
	if err := ash.WaitForVisible(ctx, tconn, settingsPkgMD); err != nil {
		return err
	}

	// Start wm Activity on external display.
	wmAct, err := arc.NewActivity(a, wmPkgMD, resizeableUnspecifiedActivityMD)
	if err != nil {
		return err
	}
	defer wmAct.Close()

	if err := startActivityOnDisplay(ctx, a, wmPkgMD, resizeableUnspecifiedActivityMD, firstExternalDisplayID); err != nil {
		return err
	}
	defer wmAct.Stop(ctx)
	if err := ash.WaitForVisible(ctx, tconn, wmPkgMD); err != nil {
		return err
	}

	for _, test := range []struct {
		name        string
		windowState ash.WindowStateType
	}{
		{"Windows are normal", ash.WindowStateNormal},
		{"Windows are maximized", ash.WindowStateMaximized},
	} {
		testing.ContextLogf(ctx, "Setting windows to %q", test.windowState)

		if err := ensureSetWindowState(ctx, tconn, settingsPkgMD, test.windowState); err != nil {
			return err
		}
		settingsWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, settingsPkgMD)
		if err != nil {
			return err
		}

		if err := ensureSetWindowState(ctx, tconn, wmPkgMD, test.windowState); err != nil {
			return err
		}
		wmWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wmPkgMD)
		if err != nil {
			return err
		}

		// Relayout external display and make sure the windows will not move their positions or show black background.
		for _, relayout := range []struct {
			name   string
			offset coords.Point
		}{
			{"Relayout external display to the left side of internal display", coords.NewPoint(-externalDisplayInfo.Bounds.Width, 0)},
			{"Relayout external display to the right side of internal display", coords.NewPoint(internalDisplayInfo.Bounds.Width, 0)},
			{"Relayout external display on top of internal display", coords.NewPoint(0, -externalDisplayInfo.Bounds.Height)},
			{"Relayout external display on bottom of internal display", coords.NewPoint(0, internalDisplayInfo.Bounds.Height)},
		} {
			if err := func() error {
				testing.ContextLogf(ctx, "Running %q", relayout.name)
				p := display.DisplayProperties{BoundsOriginX: &relayout.offset.X, BoundsOriginY: &relayout.offset.Y}
				if err := display.SetDisplayProperties(ctx, tconn, externalDisplayInfo.ID, p); err != nil {
					return err
				}
				if err := ensureWindowStable(ctx, tconn, settingsPkgMD, settingsWindowInfo); err != nil {
					return err
				}
				if err := ensureWindowStable(ctx, tconn, wmPkgMD, wmWindowInfo); err != nil {
					return err
				}
				return ensureNoBlackBkg(ctx, cr, tconn)

			}(); err != nil {
				return errors.Wrapf(err, "subtest %q failed when %q", test.name, relayout.name)
			}
		}
	}
	return nil
}

// removeAddDisplay checks whether the window moves to another display and shows inside of display.
// After adding the display back without changing windows, it checks whether the window restores to the previous display.
func removeAddDisplay(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}

	var intDispInfo, extDispInfo display.Info
	for _, info := range infos {
		if info.IsInternal {
			intDispInfo = info
		} else if extDispInfo.ID == "" {
			// Get the first external display info.
			extDispInfo = info
		}
	}

	if intDispInfo.ID == "" || extDispInfo.ID == "" {
		return errors.Wrap(err, "not enough displays: need at least one internal display and one external display")
	}

	// Start settings Activity on internal display.
	settingsAct, err := arc.NewActivity(a, settingsPkgMD, settingsActMD)
	if err != nil {
		return err
	}
	defer settingsAct.Close()

	if err := settingsAct.Start(ctx); err != nil {
		return err
	}
	defer settingsAct.Stop(ctx)
	if err := ensureActivityReady(ctx, tconn, settingsAct); err != nil {
		return err
	}

	// Start wm Activity on external display.
	wmAct, err := arc.NewActivity(a, wmPkgMD, resizeableUnspecifiedActivityMD)
	if err != nil {
		return err
	}
	defer wmAct.Close()

	if err := startActivityOnDisplay(ctx, a, wmPkgMD, resizeableUnspecifiedActivityMD, firstExternalDisplayID); err != nil {
		return err
	}
	defer wmAct.Stop(ctx)
	if err := ensureActivityReady(ctx, tconn, wmAct); err != nil {
		return err
	}

	// Set windows to normal window state.
	if err := ensureSetWindowState(ctx, tconn, settingsPkgMD, ash.WindowStateNormal); err != nil {
		return err
	}
	if err := ensureActivityReady(ctx, tconn, settingsAct); err != nil {
		return err
	}

	if err := ensureSetWindowState(ctx, tconn, wmPkgMD, ash.WindowStateNormal); err != nil {
		return err
	}
	if err := ensureActivityReady(ctx, tconn, wmAct); err != nil {
		return err
	}

	settingsWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, settingsPkgMD)
	if err != nil {
		return err
	}

	wmWindowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, wmPkgMD)
	if err != nil {
		return err
	}

	for _, removeAdd := range []struct {
		name         string
		power        displayPowerState
		origDispInfo display.Info
		destDispInfo display.Info

		moveAct     *arc.Activity
		moveWinInfo *ash.Window
	}{
		// When removing internal display, the window on internal display will move to the external display.
		{"Remove and add internal display", displayPowerInternalOffExternalOn, intDispInfo, extDispInfo, settingsAct, settingsWindowInfo},
		// When removing external display, the window on external display will move to the internal display.
		{"Remove and add external display", displayPowerInternalOnExternalOff, extDispInfo, intDispInfo, wmAct, wmWindowInfo},
	} {
		if err := func() error {
			// Remove one display and the window on the removed display should move to the other display.
			if err := setDisplayPower(ctx, removeAdd.power); err != nil {
				return err
			}
			// Wait for display off.
			if err := waitForDisplay(ctx, tconn, removeAdd.origDispInfo.ID, false, 10*time.Second); err != nil {
				return err
			}
			if err := ensureActivityReady(ctx, tconn, removeAdd.moveAct); err != nil {
				return err
			}

			// Check if the window moves to required display automatically.
			newWinInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, removeAdd.moveAct.PackageName())
			if err != nil {
				return err
			}
			if newWinInfo.DisplayID != removeAdd.destDispInfo.ID {
				return errors.Errorf("failed to move window to another display: got %s; want %s", newWinInfo.DisplayID, removeAdd.destDispInfo.ID)
			}

			if err := ensureWinBoundsInDisplay(newWinInfo.BoundsInRoot, removeAdd.destDispInfo.Bounds); err != nil {
				return err
			}

			// Re-add display and the window should move back to the original display.
			if err := setDisplayPower(ctx, displayPowerAllOn); err != nil {
				return err
			}
			// Wait for display on.
			if err := waitForDisplay(ctx, tconn, removeAdd.origDispInfo.ID, true, 10*time.Second); err != nil {
				return err
			}
			if err := ensureActivityReady(ctx, tconn, removeAdd.moveAct); err != nil {
				return err
			}
			var restoreWinBounds coords.Rect
			testing.Poll(ctx, func(ctx context.Context) error {
				restoreWinInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, removeAdd.moveAct.PackageName())
				if err != nil {
					return err
				}
				if restoreWinInfo.DisplayID != removeAdd.moveWinInfo.DisplayID {
					return errors.Errorf("failed to restore window to original display: got %s; want %s", restoreWinInfo.DisplayID, removeAdd.moveWinInfo.DisplayID)
				}
				restoreWinBounds = restoreWinInfo.BoundsInRoot
				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second})
			return ensureWinBoundsInDisplay(restoreWinBounds, removeAdd.origDispInfo.Bounds)
		}(); err != nil {
			return errors.Wrapf(err, "test removeAddDispaly failed when %q", removeAdd.name)
		}
	}
	return nil
}

// Helper functions.

// ensureWindowOnDisplay checks whether a window is on a certain display.
func ensureWindowOnDisplay(ctx context.Context, tconn *chrome.TestConn, pkgName, dispID string) error {
	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return err
	}
	if windowInfo.DisplayID != dispID {
		return errors.Errorf("invalid display ID: got %q; want %q", windowInfo.DisplayID, dispID)
	}
	return nil
}

// startActivityOnDisplay starts an activity by calling "am start --display" on the given display ID.
// TODO(ruanc): Move this function to proper location (activity.go or Ash) once the external displays has better support.
func startActivityOnDisplay(ctx context.Context, a *arc.ARC, pkgName, actName, dispID string) error {
	cmd := a.Command(ctx, "am", "start", "--display", dispID, pkgName+"/"+actName)
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}
	// "adb shell" doesn't distinguish between a failed/successful run. For that we have to parse the output.
	// Looking for:
	//  Starting: Intent { act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] cmp=com.example.name/.ActvityName }
	//  Error type 3
	//  Error: Activity class {com.example.name/com.example.name.ActvityName} does not exist.
	re := regexp.MustCompile(`(?m)^Error:\s*(.*)$`)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) == 2 {
		testing.ContextLog(ctx, "Failed to start activity: ", groups[1])
		return errors.New("failed to start activity")
	}
	return nil
}

// ensureSetWindowState checks whether the window is in requested window state. If not, make sure to set window state to the requested window state.
func ensureSetWindowState(ctx context.Context, tconn *chrome.TestConn, pkgName string, expectedState ash.WindowStateType) error {
	if state, err := ash.GetARCAppWindowState(ctx, tconn, pkgName); err != nil {
		return err
	} else if state == expectedState {
		return nil
	}
	windowEventMap := map[ash.WindowStateType]ash.WMEventType{
		ash.WindowStateNormal:     ash.WMEventNormal,
		ash.WindowStateMaximized:  ash.WMEventMaximize,
		ash.WindowStateMinimized:  ash.WMEventMinimize,
		ash.WindowStateFullscreen: ash.WMEventFullscreen,
	}
	wmEvent, ok := windowEventMap[expectedState]
	if !ok {
		return errors.Errorf("didn't find the event for window state: %q", expectedState)
	}
	state, err := ash.SetARCAppWindowState(ctx, tconn, pkgName, wmEvent)
	if err != nil {
		return err
	}
	if state != expectedState {
		return errors.Errorf("unexpected window state: got %s; want %s", state, expectedState)
	}
	return nil
}

// ensureWindowStable checks whether the window moves its position.
func ensureWindowStable(ctx context.Context, tconn *chrome.TestConn, pkgName string, expectedWindowInfo *ash.Window) error {
	windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
	if err != nil {
		return errors.Wrapf(err, "failed to get window info for window: %q", pkgName)
	}
	if !reflect.DeepEqual(windowInfo.BoundsInRoot, expectedWindowInfo.BoundsInRoot) || windowInfo.DisplayID != expectedWindowInfo.DisplayID {
		return errors.Errorf("window moves: got bounds %+v (displayID %q); expected bounds %+v (displayID %q)", windowInfo.BoundsInRoot, windowInfo.DisplayID, expectedWindowInfo.BoundsInRoot, expectedWindowInfo.DisplayID)
	}
	return nil
}

// ensureNoBlackBkg checks whether there is black background.
func ensureNoBlackBkg(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return err
	}

	for _, info := range infos {
		img, err := grabScreenshotForDisplay(ctx, cr, info.ID)
		if err != nil {
			return err
		}
		blackPixels := arcscreenshot.CountPixels(img, color.RGBA{0, 0, 0, 255})
		rect := img.Bounds()
		totalPixels := (rect.Max.Y - rect.Min.Y) * (rect.Max.X - rect.Min.X)
		percent := blackPixels * 100 / totalPixels
		testing.ContextLogf(ctx, "Black pixels = %d / %d (%d%%) on display %q", blackPixels, totalPixels, percent, info.ID)

		// "3 percent" is arbitrary.
		if percent > 3 {
			// Save image with black pixels.
			dir, ok := testing.ContextOutDir(ctx)
			if !ok {
				return errors.New("failed to get directory for saving files")
			}
			path := fmt.Sprintf("%s/screenshot-failed-%s.png", dir, info.ID)
			fd, err := os.Create(path)
			if err != nil {
				return errors.Wrap(err, "failed to create screenshot")
			}
			defer fd.Close()
			if err := png.Encode(fd, img); err != nil {
				return errors.Wrap(err, "failed to save screenshot in PNG format")
			}

			testing.ContextLogf(ctx, "Image containing the black pixels: %s", path)
			return errors.Errorf("test failed: contains %d / %d (%d%%) black pixels", blackPixels, totalPixels, percent)
		}
	}
	return nil
}

// ensureWinBoundsInDisplay checks whether the window bounds are inside of display bounds.
func ensureWinBoundsInDisplay(winBounds coords.Rect, displayBounds coords.Rect) error {
	// Convert local window bounds to global window bounds.
	winBounds.Left += displayBounds.Left
	winBounds.Top += displayBounds.Top

	if winBounds.Left < displayBounds.Left || winBounds.Top < displayBounds.Top ||
		winBounds.Left+winBounds.Width > displayBounds.Left+displayBounds.Width ||
		winBounds.Top+winBounds.Height > displayBounds.Top+displayBounds.Height {
		return errors.Errorf("window bounds is out of display bounds: window bounds %+v, display bounds %+v", winBounds, displayBounds)
	}
	return nil
}

// waitForStopAnimating waits until Ash window stops animation.
func waitForStopAnimating(ctx context.Context, tconn *chrome.TestConn, pkgName string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
		if err != nil {
			return err
		}
		if info.IsAnimating {
			return errors.New("the window is still animating")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// waitForDisplay waits until a display on or off.
func waitForDisplay(ctx context.Context, tconn *chrome.TestConn, dispID string, isOn bool, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		infos, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return err
		}
		if isOn {
			found := false
			for _, info := range infos {
				if info.ID == dispID {
					found = true
					break
				}
			}
			if !found {
				return errors.Errorf("failed to set display %s power to %t", dispID, isOn)
			}
		} else {
			for _, info := range infos {
				if info.ID == dispID {
					return errors.Errorf("failed to set display %s power to %t", dispID, isOn)
				}
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// ensureActivityReady waits until given activity is ready.
func ensureActivityReady(ctx context.Context, tconn *chrome.TestConn, act *arc.Activity) error {
	if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
		return err
	}
	if err := act.WaitForResumed(ctx, 10*time.Second); err != nil {
		return err
	}
	if err := waitForStopAnimating(ctx, tconn, act.PackageName(), 10*time.Second); err != nil {
		return err
	}
	return nil
}

// grabScreenshotForDisplay takes a screenshot for a given displayID and returns an image.Image.
func grabScreenshotForDisplay(ctx context.Context, cr *chrome.Chrome, displayID string) (image.Image, error) {
	fd, err := ioutil.TempFile("", "screenshot")
	if err != nil {
		return nil, errors.Wrap(err, "error opening screenshot file")
	}
	defer os.Remove(fd.Name())
	defer fd.Close()

	if err := screenshot.CaptureChromeForDisplay(ctx, cr, displayID, fd.Name()); err != nil {
		return nil, errors.Wrap(err, "failed to capture screenshot")
	}

	img, _, err := image.Decode(fd)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding image")
	}
	return img, nil
}

// setDisplayPower sets the display power by a given power state.
func setDisplayPower(ctx context.Context, power displayPowerState) error {
	const (
		dbusName      = "org.chromium.DisplayService"
		dbusPath      = "/org/chromium/DisplayService"
		dbusInterface = "org.chromium.DisplayServiceInterface"

		setPowerMethod = "SetPower"
	)
	if power < displayPowerAllOn || power > displayPowerInternalOnExternalOff {
		return errors.Errorf("incorrect power value: got %d, want [%d - %d]", power, displayPowerAllOn, displayPowerInternalOnExternalOff)
	}

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return errors.Wrapf(err, "failed to connect to %s", dbusName)
	}

	return obj.CallWithContext(ctx, dbusInterface+"."+setPowerMethod, 0, power).Err
}
