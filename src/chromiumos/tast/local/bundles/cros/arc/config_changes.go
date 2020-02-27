// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConfigChanges,
		Desc:         "Verifies that configChanges property in AndroidManifest.xml prevents an activity to restart on the configuration update",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      3 * time.Minute,
	})
}

func ConfigChanges(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}
	if len(infos) == 0 {
		s.Fatal("No display found")
	}
	var info *display.Info
	for i := range infos {
		if infos[i].IsInternal {
			info = &infos[i]
		}
	}
	if info == nil {
		s.Log("No internal display found. Default to the first display")
		info = &infos[0]
	}

	if info.Bounds.Height > info.Bounds.Width {
		rot := 90
		if err := display.SetDisplayProperties(ctx, tconn, info.ID, display.DisplayProperties{Rotation: &rot}); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		// Restore the initial rotation.
		defer func() {
			if err := display.SetDisplayProperties(ctx, tconn, info.ID, display.DisplayProperties{Rotation: &info.Rotation}); err != nil {
				s.Fatal("Failed to restore the initial display rotation: ", err)
			}
		}()
	}

	const (
		apk = "ArcConfigChangesTest.apk"
		pkg = "org.chromium.arc.testapp.configchanges"
		cls = ".MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
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
		resumeCountID = "org.chromium.arc.testapp.configchanges:id/resume_count"
		buttonID      = "org.chromium.arc.testapp.configchanges:id/button"
	)

	resumeCount := d.Object(ui.ID(resumeCountID))
	if err := resumeCount.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the label: ", err)
	}

	// Get how many times onResume() is called for this activity.
	initCount, err := resumeCount.GetText(ctx)
	if err != nil {
		s.Fatal("Failed to get text: ", err)
	}

	initBounds, err := act.WindowBounds(ctx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}

	if err := d.Object(ui.ID(buttonID)).Click(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		updatedBounds, err := act.WindowBounds(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get window bounds"))
		}
		if initBounds == updatedBounds {
			return errors.Errorf("window bounds did not change: %v", initBounds)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to wait for the window bounds to change: ", err)
	}

	// In case the test is broken, the activity may be still relaunching at this point.
	// Wait for it to be relaunched.
	if err := resumeCount.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find the label: ", err)
	}

	updatedCount, err := resumeCount.GetText(ctx)
	if err != nil {
		s.Fatal("Failed to get text: ", err)
	}

	if initCount != updatedCount {
		s.Fatal("The activity relaunched between orientation change")
	}
}
