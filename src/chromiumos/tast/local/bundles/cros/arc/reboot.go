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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type rebootConfig struct {
	// Extra args to be paseed to chrome.New().
	chromeArgs []string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Reboot,
		Desc:         "Checks whether Android can be repeatedly rebooted",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val:               rebootConfig{},
			ExtraSoftwareDeps: []string{"android"},
		}, {
			Name: "vm",
			Val: rebootConfig{
				chromeArgs: []string{"--enable-arcvm"},
			},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func Reboot(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(),
		chrome.ExtraArgs(s.Param().(rebootConfig).chromeArgs...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while booting ARC: ", err)
		}
	}()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if a != nil {
			a.Close()
		}
	}()

	const numTrials = 3
	for i := 0; i < numTrials; i++ {
		s.Logf("Trial %d/%d", i+1, numTrials)
		if err := runReboot(ctx, s, &a); err != nil {
			s.Fatal("Failure: ", err)
		}
	}
}

// runReboot reboots Android and re-establishes ADB connection.
// It assumes that Android is already booted and ADB connection is established.
func runReboot(ctx context.Context, s *testing.State, a **arc.ARC) error {
	oldPID, err := arc.InitPID()
	if err != nil {
		return errors.Wrap(err, "failed to get init PID before reboot")
	}

	s.Log("Running reboot command via ADB")
	if err := (*a).Command(ctx, "reboot").Run(); err != nil {
		return errors.Wrap(err, "failed to run reboot command via ADB")
	}

	s.Log("Waiting for old init process to exit")
	if err := waitProcessExit(ctx, oldPID); err != nil {
		return errors.Wrap(err, "failed to wait for old init process to exit")
	}

	(*a).Close()

	// Reboot Android and re-establish ADB connection.
	*a, err = arc.New(ctx, s.OutDir())
	if err != nil {
		// We can assume *a == nil at this point.
		return errors.Wrap(err, "failed to start ARC")
	}

	newPID, err := arc.InitPID()
	if err != nil {
		return errors.Wrap(err, "failed to get init PID after reboot")
	}
	if newPID == oldPID {
		return errors.New("init PID did not change")
	}
	return nil
}

// waitProcessExit waits for a process to exit.
func waitProcessExit(ctx context.Context, pid int32) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := process.NewProcess(pid); err == nil {
			return errors.Errorf("pid %d still exists", pid)
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})
}
