// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIAutomator,
		Desc:         "Sample test to manipulate an app with UI automator",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"todo-mvp.apk"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
		}},
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

	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Starting app")

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q: %v", cls, err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the app: ", err)
	}
	defer act.Stop(ctx, tconn)

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: adb/ui returns loggable errors
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
