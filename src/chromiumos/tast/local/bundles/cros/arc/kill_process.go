// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KillProcess,
		Desc:         "Verifies that the kernel process is gone after killing an activity",
		Contacts:     []string{"ricardoq@chromium.org", "arc-gaming+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

func KillProcess(ctx context.Context, s *testing.State) {

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	const packageName = "com.android.settings"
	act, err := arc.NewActivity(a, packageName, ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting Settings activity")
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}
	defer act.Stop(ctx)

	// Activity needs to wait for idle after it is started.
	if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, packageName)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}

	s.Log("Closing Settings activity")
	if err := window.CloseWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to close window: ", err)
	}

	s.Log("Verifying Settings process has been killed")
	// After closing the activity, the process should have been killed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		procs, err := process.Processes()
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get get processes"))
		}
		for _, p := range procs {
			name, err := p.Name()
			if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to get get process' name"))
			}
			if name == packageName {
				return errors.Errorf("process %s was not killed yet", packageName)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Settings process is still alive: ", err)
	}
}
