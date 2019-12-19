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
		Func:         WMResizableConversion,
		Desc:         "Verifies that Window Manager RV use cases behaves as described in go/arc-wm-r",
		Contacts:     []string{"arthurhsu@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_r", "chrome"},
		Data:         []string{"ArcWMTestApp_24.apk"},
		Pre:          arc.Booted(),
		Timeout:      8 * time.Minute,
	})
}

func WMResizableConversion(ctx context.Context, s *testing.State) {
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
		{"RV19_landscape", wmRV19},
		{"RV20_portrait", wmRV20},
		{"RV21_undefined_orientation", wmRV21},
		{"RV22_split_screen", wmRV22},
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

func wmRV19(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRV20(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRV21(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}

func wmRV22(ctx context.Context, tconn *chrome.Conn, a *arc.ARC, d *ui.Device) error {
	// TODO(b/140203139): implement
	return nil
}
