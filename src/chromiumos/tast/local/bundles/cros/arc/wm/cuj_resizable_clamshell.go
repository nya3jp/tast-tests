// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
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
	wmResizeableLandscapeActivity   = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"
	wmResizeableUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
	wmResizeablePortraitActivity    = "org.chromium.arc.testapp.windowmanager.ResizeablePortraitActivity"

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
type wmTestStateFunc func(context.Context, *chrome.Conn, *arc.Activity, *ui.Device) error

// uiClickFunc represents a function that "clicks" on a certain widget using UI Automator.
type uiClickFunc func(context.Context, *arc.Activity, *ui.Device) error

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowManagerCUJRC,
		Desc:         "Verifies that Window Manager Critical User Journey behaves as described in go/arc-wm-r",
		Contacts:     []string{"ricardoq@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_r", "chrome"},
		Data:         []string{"ArcWMTestApp_24.apk", "ArcPipTastTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

// WindowManagerCUJRC tests CUJs for resizable app on clamshell mode
func WindowManagerCUJRC(ctx context.Context, s *testing.State) {
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

	for _, apk := range []string{"ArcWMTestApp_24.apk", "ArcPipTastTest.apk"} {
		if err := a.Install(ctx, s.DataPath(apk)); err != nil {
			s.Fatal("Failed installing app: ", err)
		}
	}

	type testFunc func(context.Context, *chrome.Conn, *arc.ARC, *ui.Device) error
	for idx, test := range []struct {
		name string
		fn   testFunc
	}{
		{"RC01: default Launch behavior", rc01DefaultLaunchBehavior},
		{"RC06: immerse via API ignored if windowed", rc06ImmerseViaAPIIgnoredIfWindowed},
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

// rc01DefaultLaunchBehavior tests two different apps: whitelisted and non-whitelisted. For each app, the test will
// launch the app in clamshell mode with 3 SDK-N activities having different orientations.
// It verifies that their default launch state is the expected one, as defined in: go/arc-wm-r RC01.
func rc01DefaultLaunchBehavior(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name        string
		pkg         string
		act         string
		wantedState wmTestStateFunc
	}{
		// The are four possible default states (windows #A to #D) from six possible different activities.
		{"Landscape, default", wmPkg24, wmResizeableLandscapeActivity, checkMaximizeResizeable},
		{"Unspecified, default", wmPkg24, wmResizeableUnspecifiedActivity, checkMaximizeResizeable},
		{"Portrait, default", wmPkg24, wmResizeablePortraitActivity, checkRestoreResizeable},
		// TODO(arthurhsu): enable whitelist testing
	} {
		if err := func() error {
			testing.ContextLogf(ctx, "Running subtest %q", test.name)
			act, err := arc.NewActivity(a, test.pkg, test.act)
			if err != nil {
				return err
			}
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				return err
			}
			// Stop activity at exit time so that the next WM test can launch a different activity from the same package.
			defer act.Stop(ctx)
			if err := waitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			return test.wantedState(ctx, tconn, act, d)
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}

// rc06LightsOutIgnored verifies that an N activity cannot go from restored to fullscreen mode as defined in:
// go/arc-wm-p "Clamshell: lights out and fullscreen ignored" (slides #22-#23)
func wmLightsOutIgnored(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	for _, test := range []struct {
		name     string
		pkg      string
		activity string
	}{
		{"Landscape + Resize enabled + N", wmPkg24, wmResizeableLandscapeActivity},
		{"Portrait + Resize enabled + N", wmPkg24, wmResizeablePortraitActivity},
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
			if err := waitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
				return err
			}

			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateNormal); err != nil {
				return err
			}

			// Clicking on "Immersive" button should not change the state of the restored window.
			if err := uiClickImmersive(ctx, act, d); err != nil {
				return err
			}

			// TODO(crbug.com/1010469): This tries to verify that nothing changes, which is very hard.
			if err := waitUntilActivityIsReady(ctx, tconn, act, d); err != nil {
				return err
			}

			if ws, err := act.GetWindowState(ctx); err != nil {
				return err
			} else if ws != arc.WindowStateNormal {
				return errors.Errorf("invalid window state: got %q; want %q", ws.String(), arc.WindowStateNormal.String())
			}
			return nil
		}(); err != nil {
			return errors.Wrapf(err, "%q subtest failed", test.name)
		}
	}
	return nil
}
