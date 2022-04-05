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
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MouseKeyEvent,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks mouse buttons emit the correct key events on ARC",
		Contacts:     []string{"nergi@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Timeout:      3 * time.Minute,
	})
}

func MouseKeyEvent(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC
	d := p.UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const (
		apk          = "ArcMouseKeyEventTest.apk"
		pkg          = "org.chromium.arc.testapp.mousekeyevent"
		activityName = ".MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatal("Failed to create an activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start an activity: ", err)
	}
	defer act.Stop(cleanupCtx, tconn)

	if err := mouse.Press(tconn, mouse.ForwardButton)(ctx); err != nil {
		s.Fatal("Failed to press forward button on mouse: ", err)
	}
	if err := mouse.Release(tconn, mouse.ForwardButton)(ctx); err != nil {
		s.Fatal("Failed to release forward button on mouse: ", err)
	}

	if err := mouse.Press(tconn, mouse.BackButton)(ctx); err != nil {
		s.Fatal("Failed to press back button on mouse: ", err)
	}
	if err := mouse.Release(tconn, mouse.BackButton)(ctx); err != nil {
		s.Fatal("Failed to release forward button on mouse: ", err)
	}

	fieldID := pkg + ":id/generated_key_events"
	output := "ACTION_DOWN : KEYCODE_FORWARD\n" +
		"ACTION_UP : KEYCODE_FORWARD\n" +
		"ACTION_DOWN : KEYCODE_BACK\n" +
		"ACTION_UP : KEYCODE_BACK\n"
	if err := d.Object(ui.ID(fieldID), ui.Text(output)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
}
