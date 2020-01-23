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
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
//	wmPkg = "org.chromium.arc.testapp.windowmanager23"

//	wmPortraitActivity    = "org.chromium.arc.testapp.windowmanager.NonResizeablePortraitActivity"
//	wmLandscapeActivity   = "org.chromium.arc.testapp.windowmanager.NonResizeableLandscapeActivity"
//	wmUnspecifiedActivity = "org.chromium.arc.testapp.windowmanager.NonResizeableUnspecifiedActivity"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WMNonresizableClamshell,
		Desc:         "Verifies that Window Manager NC use cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"arthurhsu@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_r", "chrome"},
		Data:         []string{"ArcWMTestApp_23.apk"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMNonresizableClamshell(ctx context.Context, s *testing.State) {
	wmApkToInstall := []string{"ArcWMTestApp_23.apk"}
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
		{"NC01_launch", wmNC01},
		{"NC04_immerse_portrait", wmNC04},
		{"NC05_immerse_nonportrait", wmNC05},
		{"NC07_immerse_maximized_portrait", wmNC07},
		{"NC09_new_activity_follows_root", wmNC09},
		{"NC10_springboard", wmNC10},
		{"NC11_pip", wmNC11},
		{"NC12_hide_shelf", wmNC12},
		{"NC15_resolution_change", wmNC15},
		{"NC17_font_size_change", wmNC17},
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

func wmNC01(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC04(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC05(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC07(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC09(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC10(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC11(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC12(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC15(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmNC17(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}
