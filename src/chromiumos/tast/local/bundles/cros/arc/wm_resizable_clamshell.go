// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
//	wmPkg = "org.chromium.arc.testapp.windowmanager24"

//	wmPortraitActivity    = "org.chromium.arc.testapp.windowmanager.ResizeablePortraitActivity"
//	wmLandscapeActivity   = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"
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

	tconn, d, err := commonWMSetUp(ctx, s, wmApkToInstall)
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
		{"RC01_launch", wmRC01},
		{"RC02_maximize_portrait", wmRC02},
		{"RC03_maximize_nonportrait", wmRC03},
		{"RC04_immerse_portrait", wmRC04},
		{"RC05_immerse_nonportrait", wmRC05},
		{"RC06_immerse_ignored", wmRC06},
		{"RC07_immerse_maximized_portrait", wmRC07},
		{"RC08_immerse_maximized_nonportrait", wmRC08},
		{"RC09_new_activity_follows_root", wmRC09},
		{"RC10_springboard", wmRC10},
		{"RC11_pip", wmRC11},
		{"RC12_hide_shelf", wmRC12},
		{"RC13_freeform_resize", wmRC13},
		{"RC14_snap_to_half_screen", wmRC14},
		{"RC15_resolution_change", wmRC15},
		{"RC17_font_size_change", wmRC17},
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

	err = restoreTabletSettings(tabletModeEnabled, ctx, tconn)
	if err != nil {
		s.Fatal("Failed to restore tablet mode settings", err)
	}
}

func wmRC01(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC02(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC03(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC04(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC05(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC06(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC07(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC08(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC09(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC10(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC11(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC12(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC13(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC14(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC15(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRC17(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}
