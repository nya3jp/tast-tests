// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchIntent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks launching an activity with extra values works",
		Contacts:     []string{"yhanada@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func LaunchIntent(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const (
		apk           = "ArcLaunchIntentTest.apk"
		pkg           = "org.chromium.arc.testapp.intent"
		activity      = ".MainActivity"
		buttonID      = pkg + ":id/button"
		intExtraID    = pkg + ":id/int_extra"
		textExtraID   = pkg + ":id/string_extra"
		parcelExtraID = pkg + ":id/parcel_extra"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activity)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q: %v", activity, err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q: %v", activity, err)
	}
	defer act.Stop(cleanupCtx, tconn)

	if err := d.Object(ui.ID(buttonID)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the button: ", err)
	}
	if err := d.Object(ui.ID(buttonID)).Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}

	// Check that the extra values are passed to the launched activity correctly
	if err := d.Object(ui.ID(intExtraID), ui.Text("103")).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the int extra text: ", err)
	}
	if err := d.Object(ui.ID(textExtraID), ui.Text("test")).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the int extra text: ", err)
	}
	if err := d.Object(ui.ID(parcelExtraID), ui.Text("parcelable test")).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the int extra text: ", err)
	}
}
