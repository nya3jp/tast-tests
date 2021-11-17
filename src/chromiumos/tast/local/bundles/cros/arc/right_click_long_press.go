// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RightClickLongPress,
		Desc:         "Checks right click is properly converted to long press in compat mode",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      4 * time.Minute,
	})
}

func RightClickLongPress(ctx context.Context, s *testing.State) {
	const (
		apk          = "ArcLongPressTest.apk"
		pkg          = "org.chromium.arc.testapp.longpress"
		activityName = ".MainActivity"
	)

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"), chrome.ExtraArgs("--enable-features=ArcRightClickLongPress"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := a.Install(ctx, arc.APKPath(apk), adb.InstallOptionFromPlayStore); err != nil {
		s.Fatal("Failed to install the app: ", err)
	}
	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(ctx, tconn)

	if err := wm.CheckVisibility(ctx, tconn, wm.BubbleDialogClassName, true); err != nil {
		s.Fatal("Failed to wait for splash: ", err)
	}

	if err := wm.CloseSplash(ctx, tconn, wm.InputMethodClick, nil); err != nil {
		s.Fatal("Failed to close splash: ", err)
	}

	bounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}

	dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display mode: ", err)
	}
	bounds = coords.ConvertBoundsFromPXToDP(bounds, dispMode.DeviceScaleFactor)

	if err := mouse.Click(tconn, bounds.CenterPoint(), mouse.RightButton)(ctx); err != nil {
		s.Fatal("Failed to right click the activity: ", err)
	}

	const (
		fieldID  = pkg + ":id/long_press_count"
		expected = "1"
	)

	if err := d.Object(ui.ID(fieldID)).WaitForText(ctx, expected, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for long press: ", err)
	}
}
