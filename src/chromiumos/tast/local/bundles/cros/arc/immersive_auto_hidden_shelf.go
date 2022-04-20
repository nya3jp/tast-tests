// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImmersiveAutoHiddenShelf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Tests that the shelf is auto-hidden after launching an immersive ARC application",
		Contacts: []string{
			"yulunwu@chromium.org",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      120 * time.Second,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// ImmersiveAutoHiddenShelf checks that the shelf is auto-hidden and shown when
// an ARC app transitions in and out of the immersive state.
func ImmersiveAutoHiddenShelf(ctx context.Context, s *testing.State) {
	const (
		ResizeableLandscapeActivity = "org.chromium.arc.testapp.windowmanager.ResizeableLandscapeActivity"
	)

	p := s.FixtValue().(*arc.PreData)
	a := p.ARC
	cr := p.Chrome
	d := p.UIDevice

	s.Log("Creating Test API connection")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to enter clamshell mode: ", err)
	}
	defer cleanup(ctx)

	primaryDisplayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find primary display info: ", err)
	}

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}

	// Disable auto-hidden shelf for this test.
	if origShelfBehavior != ash.ShelfBehaviorNeverAutoHide {
		if err := ash.SetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
			s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
		}
		if err := ash.WaitForHotseatToUpdateAutoHideState(ctx, tconn, false); err != nil {
			s.Fatal("Failed verify shelf is shown without any open windows: ", err)
		}
	}

	// Restore shelf state to original behavior after test completion.
	defer ash.SetShelfBehavior(ctx, tconn, primaryDisplayInfo.ID, origShelfBehavior)

	act, err := arc.NewActivity(a, wm.Pkg24, ResizeableLandscapeActivity)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Logf("Starting activity: %s/%s", wm.Pkg24, ResizeableLandscapeActivity)
	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed start activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateMaximized); err != nil {
		s.Fatal("Failed to make activity fullscreen: ", err)
	}

	// Click on the "immersive" button in the activity.
	if err := wm.UIClickImmersive(ctx, act, d); err != nil {
		s.Fatal("Failed to click the immersive button: ", err)
	}

	// Check that the shelf is auto-hidden.
	if err := ash.WaitForHotseatToUpdateAutoHideState(ctx, tconn, true); err != nil {
		s.Fatal("Shelf should be hidden when ARC app has an active immersive activity: ", err)
	}

	// Click on the "normal" button in the activity.
	if err := wm.UIClickNormal(ctx, act, d); err != nil {
		s.Fatal("Failed to click the normal button: ", err)
	}

	// Check that the shelf is restored.
	if err := ash.WaitForHotseatToUpdateAutoHideState(ctx, tconn, false); err != nil {
		s.Fatal("Shelf should be not be hidden normally: ", err)
	}
}
