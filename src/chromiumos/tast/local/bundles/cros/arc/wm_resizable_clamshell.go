// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	wmPkg = "org.chromium.arc.testapp.windowmanager24"

	//	wmPortraitActivity    = "org.chromium.arc.testapp.windowmanager.ResizeablePortraitActivity"
	wmLandscapeActivity = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"

//	wmUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.ResizeableUnspecifiedActivity"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMResizableClamshell,
		Desc:         "Verifies that Window Manager RC use cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"arthurhsu@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_r", "chrome"},
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMResizableClamshell(ctx context.Context, s *testing.State) {
	wmApkToInstall := []string{"ArcWMTestApp_24.apk"}
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, d, err := wm.CommonWMSetUp(ctx, s, wmApkToInstall)
	if err != nil {
		s.Fatal("Failed to setup test: ", err)
	}
	defer d.Close()

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}

	type testFunc func(context.Context, *chrome.Conn, *arc.ARC, *ui.Device) error
	for _, test := range []struct {
		name string
		fn   testFunc
	}{
		//		{"RC01_launch", wmRC01},
		//		{"RC02_maximize_portrait", wmRC02},
		//		{"RC03_maximize_nonportrait", wmRC03},
		//		{"RC04_immerse_portrait", wmRC04},
		//		{"RC05_immerse_nonportrait", wmRC05},
		//		{"RC06_immerse_ignored", wmRC06},
		//		{"RC07_immerse_maximized_portrait", wmRC07},
		//		{"RC08_immerse_maximized_nonportrait", wmRC08},
		//		{"RC09_new_activity_follows_root", wmRC09},
		//		{"RC10_springboard", wmRC10},
		//		{"RC11_pip", wmRC11},
		//		{"RC12_hide_shelf", wmRC12},
		{"RC13_freeform_resize", wmRC13},
		//		{"RC14_snap_to_half_screen", wmRC14},
		//		{"RC15_resolution_change", wmRC15},
		//		{"RC17_font_size_change", wmRC17},
	} {
		s.Logf("Running test %q", test.name)

		// Reset WM state to default values.
		if err := a.Command(ctx, "am", "broadcast", "-a", "android.intent.action.arc.cleartaskstate").Run(); err != nil {
			s.Fatal("Failed to clear task states: ", err)
		}

		if err := test.fn(ctx, tconn, a, d); err != nil {
			path := fmt.Sprintf("%s/screenshot-cuj-failed-test-%s.png", s.OutDir(), test.name)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("%s test failed: %v", test.name, err)
		}
	}

	err = wm.RestoreTabletSettings(ctx, tconn, tabletModeEnabled)
	if err != nil {
		s.Fatal("Failed to restore tablet mode settings: ", err)
	}
}

func wmRC13(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	act, err := arc.NewActivity(a, wmPkg, wmLandscapeActivity)
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
	to := arc.NewPoint(right, origBounds.Top+origBounds.Height/2)
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
