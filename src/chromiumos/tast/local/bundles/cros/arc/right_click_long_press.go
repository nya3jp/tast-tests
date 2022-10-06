// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RightClickLongPress,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks right click is properly converted to long press in compat mode",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "arcBooted",
		Timeout:      4 * time.Minute,
	})
}

func RightClickLongPress(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const (
		apk          = "ArcLongPressTest.apk"
		pkg          = "org.chromium.arc.testapp.longpress"
		activityName = ".MainActivity"
	)

	cleanupTabletMode, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to set clamshell mode: ", err)
	}
	defer cleanupTabletMode(cleanupCtx)

	if err := a.Install(ctx, arc.APKPath(apk), adb.InstallOptionFromPlayStore); err != nil {
		s.Fatal("Failed to install the app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q: %v", activityName, err)
	}
	defer act.Close()
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	defer act.Stop(cleanupCtx, tconn)

	// Close the splash screen if it's shown.
	if err := wm.CheckVisibility(ctx, tconn, wm.BubbleDialogClassName, true); err == nil {
		if err := wm.CloseSplash(ctx, tconn, wm.InputMethodClick, nil); err != nil {
			s.Fatal("Failed to close splash: ", err)
		}
	}

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkg)
	if err != nil {
		s.Fatal("Failed to get ARC app window info: ", err)
	}

	if err := mouse.Click(tconn, window.TargetBounds.CenterPoint(), mouse.RightButton)(ctx); err != nil {
		s.Fatal("Failed to right click the activity: ", err)
	}

	const fieldID = pkg + ":id/long_press_count"

	if err := d.Object(ui.ID(fieldID)).WaitForText(ctx, "1", 30*time.Second); err != nil {
		s.Fatal("Failed to wait for long press: ", err)
	}
}
