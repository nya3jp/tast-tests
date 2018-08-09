// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

import (
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Sample,
		Desc:         "Runs a sample app",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"todo-mvp.apk"},
		Timeout:      3 * time.Minute,
	})
}

func Sample(s *testing.State) {
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

	defer faillog.SaveIfError(s)

	ctx := s.Context()
	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	apptest.Run(s, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		// Wait for the default entries to show up.
		must(d.Object(ui.ID(titleID), ui.Text(defaultTitle1)).WaitForExists(ctx))
		must(d.Object(ui.ID(titleID), ui.Text(defaultTitle2)).WaitForExists(ctx))

		// Click the add button.
		must(d.Object(ui.ID(addButtonID)).Click(ctx))

		// Fill the form and click the done button.
		input := d.Object(ui.ID(titleInputID))
		must(input.WaitForExists(ctx))
		must(input.SetText(ctx, customTitle))
		must(d.Object(ui.ID(doneButtonID)).Click(ctx))

		// Wait for our new entry to show up.
		must(d.Object(ui.ID(titleID), ui.Text(customTitle)).WaitForExists(ctx))
	})
}
