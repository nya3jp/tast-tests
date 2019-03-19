// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func:         UIAutomator,
		Desc:         "Sample test to manipulate an app with UI automator",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"todo-mvp.apk"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
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
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Starting app")

	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	if err := a.Command(ctx, "am", "start", "-W", pkg+"/"+cls).Run(); err != nil {
		s.Fatal("Failed starting app: ", err)
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
	must(input.WaitForExistsWithDefaultTimeout(ctx))
	must(input.SetText(ctx, customTitle))
	must(d.Object(ui.ID(doneButtonID)).Click(ctx))

	// Wait for our new entry to show up.
	must(d.Object(ui.ID(titleID), ui.Text(customTitle)).WaitForExistsWithDefaultTimeout(ctx))

	// Returns UI Device info like bounds, orientation, current activity and more.
	info, err := d.GetInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get UI device info: ", err)
	}
	s.Logf("Device info: %+v", info)

	d.PressKeyCode(ctx, ui.KEYCODE_BACK, 0)
}
