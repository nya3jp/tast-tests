// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"time"

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
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func KillProcess(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

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
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, packageName)
	if err != nil {
		s.Fatal("Failed to get window info: ", err)
	}

	// Validity check: the process must exist after creating the activity.
	if exist, err := processExist(ctx, a, packageName); err != nil {
		s.Fatal("Failed to verify whether process exist: ", err)
	} else if !exist {
		s.Fatalf("Process %s does not exist after activity was created", packageName)
	}

	s.Log("Closing Settings activity")
	if err := window.CloseWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to close window: ", err)
	}

	s.Log("Verifying Settings process has been killed")
	// After closing the activity, the process should have been killed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		exist, err := processExist(ctx, a, packageName)
		if err != nil {
			return testing.PollBreak(err)
		}
		if exist {
			return errors.Errorf("process %s was not killed yet", packageName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Settings process is still alive: ", err)
	}
}

// processExist returns whether the kernel process procName exist.
func processExist(ctx context.Context, a *arc.ARC, procName string) (bool, error) {
	// Cannot query the host processes since this test runs both on Container and VM.
	// Querying Android processes instead.
	out, err := a.Command(ctx, "ps", "-A").Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to get processes")
	}

	// Package name is also the process name.
	matched, err := regexp.MatchString(procName+"\n", string(out))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse output")
	}

	return matched, nil
}
