// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TaskManager,
		Desc:         "Checks Task Manager Integration with Arc",
		Fixture:      "arcBooted",
		Contacts:     []string{"rnanjappan@chromium.org", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      arc.BootTimeout + 2*time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func TaskManager(ctx context.Context, s *testing.State) {

	const (
		// This is a plain hello world app.
		apk        = "ArcAppValidityTest.apk"
		pkg        = "org.chromium.arc.testapp.appvaliditytast"
		cls        = ".MainActivity"
		maxAttempt = 35
	)

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	taskManager := nodewith.Name("Task Manager").ClassName("TaskManagerView").First()
	arcEntry := nodewith.Name("App: org.chromium.arc.testapp.appvaliditytast").Ancestor(taskManager)
	ui := uiauto.New(tconn)

	if err := kb.Accel(ctx, "Search+Esc"); err != nil {
		s.Fatal("Failed to launch Task Manager: ", err)
	}

	// Scroll down the task manager list when ARC
	// entry is not found on first page until its found.
	found := false
	for j := 0; j < maxAttempt; j++ {
		if err := ui.Exists(arcEntry)(ctx); err != nil {
			s.Log("Down Press Attemmpt: ", j)
			if err := kb.Accel(ctx, "Down"); err != nil {
				s.Fatal("Failed to press Down: ", err)
			}
		} else {
			found = true
			break
		}
	}

	if found == false {
		s.Fatal("Failed to find ARC entry")
	}

	if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
		s.Fatal("Failed to exit Task Manager: ", err)
	}

}
