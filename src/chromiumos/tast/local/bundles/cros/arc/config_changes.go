// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConfigChanges,
		Desc:         "Checks that configChanges works as intended",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcOrientationTest.apk"},
		Pre:          arc.Booted(),
		Timeout:      3 * time.Minute,
	})
}

func ConfigChanges(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const (
		apk = "ArcOrientationTest.apk"
		pkg = "org.chromium.arc.testapp.orientation"
		cls = ".MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed starting app: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed starting app: ", err)
	}
	defer act.Stop(ctx)

	const (
		textID   = "org.chromium.arc.testapp.orientation:id/resume_count"
		buttonID = "org.chromium.arc.testapp.orientation:id/button"
	)

	button := d.Object(ui.ID(buttonID))
	if err := button.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the button: ", err)
	}
	if err := button.Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
}
