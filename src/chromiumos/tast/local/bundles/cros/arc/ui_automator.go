// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIAutomator,
		Desc:         "Sample test to manipulate an app with UI automator",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Data:         []string{"todo-mvp.apk"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
		}},
		Timeout: 12 * time.Minute,
	})
}

func UIAutomator(ctx context.Context, s *testing.State) {
	const (
		// This is a sample TODO app available at:
		// https://github.com/googlesamples/android-architecture/tree/todo-mvp/
		apk = "todo-mvp.apk"
		pkg = "com.example.android.architecture.blueprints.todomvp"
		cls = "com.example.android.architecture.blueprints.todoapp.tasks.TasksActivity"

		titleID      = "com.example.android.architecture.blueprints.todomvp:id/title"
		addButtonID  = "com.example.android.architecture.blueprints.todomvp:id/fab_add_task"
		titleInputID = "com.example.android.architecture.blueprints.todomvp:id/add_task_title"
		doneButtonID = "com.example.android.architecture.blueprints.todomvp:id/fab_edit_task_done"

		defaultTitle1 = "Build tower in Pisa"
		defaultTitle2 = "Finish bridge in Tacoma"
		customTitle   = "Meet the team at Sagrada Familia"
	)

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Starting app")

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	// Sleep long enough for the screen to turn off
	testing.Sleep(ctx, 8*time.Minute)

	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed starting app: ", err)
	}

	s.Log("Waiting for the app to be displayed")
	if err := ash.WaitForVisible(ctx, tconn, pkg); err != nil {
		s.Fatal("The app window doesn't display if the screen is off: ", err)
		// As a note, if you didn't wait for the window to display, the test would still pass.
		// ARC is capable of interacting with an invisible app.
		// So this issue mostly effects tests that care about the window displaying.
		// Feel free to move the mouse or type a key, you should see the app launch.
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	// Wait until the current activity is idle.
	must(d.WaitForIdle(ctx, 10*time.Second))

	// Click the add button.
	must(d.Object(ui.ID(addButtonID)).Click(ctx))

	// Fill the form and click the done button.
	input := d.Object(ui.ID(titleInputID))

	// Wait until the resource exists.
	must(input.WaitForExists(ctx, 30*time.Second))
	must(input.SetText(ctx, customTitle))
	must(d.Object(ui.ID(doneButtonID)).Click(ctx))

	// Wait until the done button is gone.
	must(d.Object(ui.ID(doneButtonID)).WaitUntilGone(ctx, 5*time.Second))

	// Wait for our new entry to show up.
	must(d.Object(ui.ID(titleID), ui.Text(customTitle)).WaitForExists(ctx, 30*time.Second))

	// Returns UI Device info like bounds, orientation, current activity and more.
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get UI device info: ", err)
	}
	s.Logf("Device info: %+v", info)

	d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0)
}
