// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HeavyMemoryUser,
		Desc:         "Tests heavy memory use with chrome, arc and vms running",
		Attr:         []string{"informational"},
		Timeout:      10 * time.Minute,
		Data:         []string{"todo-mvp.apk"},
		SoftwareDeps: []string{"android", "chrome_login", "vm_host"},
	})
}

func HeavyMemoryUser(ctx context.Context, s *testing.State) {
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

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

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

	_, err = cr.NewConn(ctx, "https://www.google.com/")
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Restarting Concierge")
	conc, err := vm.NewConcierge(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to start concierge: ", err)
	}

	testvm := vm.NewDefaultVM(conc)
	err = testvm.Start(ctx)
	if err != nil {
		s.Fatal("Failed to start VM: ", err)
	}

	s.Log("Running dd command")
	cmd := testvm.Command(ctx, "dd", "if=/dev/urandom", "of=/mnt/stateful/lxd_conf/foo", "bs=3M", "count=1K")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		output := string(out[:])
		s.Log(output)
		s.Fatal("Failed to run dd: ", err)
	}
	cmd.DumpLog(ctx)
	output := string(out[:])
	s.Log(output)
	err = testvm.Stop(ctx)
	if err != nil {
		s.Fatal("Failed to close VM: ", err)
	}

}
