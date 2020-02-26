// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IMEBlockingVK,
		Desc:         "Checks if IME is properly hidden by an ARC dialog in tablet mode",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcImeBlockingTest.apk"},
		Pre:          arc.BootedInTabletMode(),
	})
}

// waitForVKVisibility waits until the virtual keyboard is shown or hidden.
func waitForVKVisibility(ctx context.Context, tconn *chrome.TestConn, shown bool) error {
	root, err := chromeui.Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)

	// Wait for the correct visibility.
	params := chromeui.FindParams{
		Role:  chromeui.RoleTypeKeyboard,
		State: map[chromeui.StateType]bool{chromeui.StateTypeInvisible: !shown},
	}
	return root.WaitForDescendant(ctx, params, true, 30*time.Second)
}

func IMEBlockingVK(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const (
		apk = "ArcImeBlockingTest.apk"
		pkg = "org.chromium.arc.testapp.imeblocking"
		cls = ".MainActivity"
	)

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	s.Log("Starting app")
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}

	const (
		fieldID  = "org.chromium.arc.testapp.imeblocking:id/text"
		buttonID = "org.chromium.arc.testapp.imeblocking:id/button"
	)
	s.Log("Setting up app's initial state")
	field := d.Object(ui.ID(fieldID))
	if err := field.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to find field: ", err)
	}
	if err := field.Click(ctx); err != nil {
		s.Fatal("Failed to click field: ", err)
	}

	s.Log("Waiting for the VK to show up")
	if err := waitForVKVisibility(ctx, tconn, true); err != nil {
		s.Fatal("Failed to show the virtual keyboard: ", err)
	}

	s.Log("Opening a dialog")
	button := d.Object(ui.ID(buttonID))
	if err := button.Click(ctx); err != nil {
		s.Fatal("Failed to click button: ", err)
	}

	if err := d.Object(ui.Text("OK"), ui.PackageName(pkg)).WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("Failed to wait for a dialog: ", err)
	}

	s.Log("Waiting for the VK to hide")
	if err := waitForVKVisibility(ctx, tconn, false); err != nil {
		s.Fatal("Failed to hide the virtual keyboard: ", err)
	}
}
