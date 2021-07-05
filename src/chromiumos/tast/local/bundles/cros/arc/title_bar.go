// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TitleBar,
		Desc:         "Test the Title Bar of the ARC App and Its buttons",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      arc.BootTimeout + 2*time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func TitleBar(ctx context.Context, s *testing.State) {
	const (
		apk     = "ArcAppValidityTest.apk"
		pkgName = "org.chromium.arc.testapp.appvaliditytast"
		cls     = ".MainActivity"
	)

	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get : ", err)
	}

	// Force Chrome to be in clamshell mode, where title bar buttons are visible.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to get : ", err)
	}
	// Restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(cleanupCtx, tconn, tabletModeEnabled)

	act, err := arc.NewActivity(a, pkgName, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(ctx, tconn)

	wanted := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonMaximizeAndRestore | ash.CaptionButtonClose

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get activity's window info.
		info, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
		if err != nil {
			return errors.Wrapf(err, "failed to ARC window infomation for package name %s", pkgName)
		}

		if info.CaptionButtonEnabledStatus != wanted {
			return errors.Errorf("Wanted %s got %s", wanted.String(), info.CaptionButtonEnabledStatus.String())
		}
		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Caption button check failed : ", err)
	}

	ui := uiauto.New(tconn)
	if t, ok := arc.Type(); ok && t == arc.VM {
		s.Log("ARC-R")
		if err := uiauto.Combine("maximize and restore it back",
			ui.LeftClick(nodewith.Name("Maximize").Role(role.Button)),
			ui.LeftClick(nodewith.Name("Restore").Role(role.Button)),
		)(ctx); err != nil {
			s.Fatal("Failed to Maximize and Restore it back : ", err)
		}
	} else if ok && t == arc.Container {
		s.Log("ARC-P")
		if err := uiauto.Combine("restore and maximize it back",
			ui.LeftClick(nodewith.Name("Restore").Role(role.Button)),
			ui.LeftClick(nodewith.Name("Maximize").Role(role.Button)),
		)(ctx); err != nil {
			s.Fatal("Failed to Restore and Maximize it back : ", err)
		}
	} else {
		s.Errorf("Unsupported ARC type %d", t)
	}

	s.Log("Press Back Buttton")
	if err := d.PressKeyCode(ctx, androidui.KEYCODE_BACK, 0); err != nil {
		s.Fatal("Failed to enter KEYCODE_BACK: ", err)
	}

	s.Log("Restart activity")
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start activity: ", err)
	}

	s.Log("Tap on Close ")
	if err := ui.LeftClick(nodewith.Name("Close").Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to Close: ", err)
	}

}
