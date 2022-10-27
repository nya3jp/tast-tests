// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks Task Manager Integration with Arc",
		Fixture:      "arcBooted",
		Contacts:     []string{"cpiao@google.com", "cros-arc-te@google.com"},
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
		apk = "ArcAppValidityTest.apk"
		pkg = "org.chromium.arc.testapp.appvaliditytast"
		cls = ".MainActivity"
		// An estimate task count in the task manager list.
		//TODO(b/185606104): Identify end of task manager list.
		maxScroll = 100
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
	if err = act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	taskManager := nodewith.Name("Task Manager").ClassName("TaskManagerView").First()
	arcEntry := nodewith.Name("App: " + pkg).Ancestor(taskManager)
	ui := uiauto.New(tconn)

	// Use a shortened context for tests to reserve time for cleanup.
	cleanCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	if err := kb.Accel(ctx, "Search+Esc"); err != nil {
		s.Fatal("Failed to launch Task Manager: ", err)
	}
	// TODO(b/186370767): Add task manager cleanup to Chrome.ResetState.
	defer func() {
		if kb.Accel(cleanCtx, "Ctrl+W"); err != nil {
			s.Error("Failed to exit Task Manager: ", err)
		}
	}()

	if err := ui.WaitUntilExists(taskManager)(ctx); err != nil {
		s.Fatal("Failed to open Task Manager: ", err)
	}

	// Scroll down the task manager list when ARC
	// entry is not found on first page until its found.
	arcEntryFound := false
	for j := 0; j < maxScroll; j++ {
		if err := ui.Exists(arcEntry)(ctx); err == nil {
			arcEntryFound = true
			break
		}
		if err := kb.Accel(ctx, "Down"); err != nil {
			s.Fatal("Failed to press Down: ", err)
		}
	}

	if !arcEntryFound {
		s.Fatal("Failed to find ARC entry")
	}
}
