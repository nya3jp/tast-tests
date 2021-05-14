// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ADBOverUSB,
		Desc:     "Checks that arc(vm)-adbd job is up and running when adb-over-usb feature available",
		Contacts: []string{"shuanghu@chromium.org", "arc-eng@google.com"},
		HardwareDeps: hwdep.D(
			// Available boards info, please refer to doc https://www.chromium.org/chromium-os/chrome-os-systems-supporting-adb-debugging-over-usb
			hwdep.Model("eve", "atlas", "nocturne", "soraka"),
		),
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 10 * time.Minute,
	})
}

func ADBOverUSB(ctx context.Context, s *testing.State) {
	// This test will reboot ARC, create a new ARC instance here instead of using ArcBooted fixture.
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer func() {
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close Chrome while (re)booting ARC: ", err)
		}
	}()

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer func() {
		if a != nil {
			a.Close(ctx)
		}
	}()

	oldPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get init PID before reboot: ", err)
	}

	// Enable USB Device Controller(udc)
	s.Log("Enable udc via crossystem")
	output, err := testexec.CommandContext(ctx, "crossystem", "dev_enable_udc").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to read dev_enable_udc: ", err)
	}

	udcEnabled := strings.TrimSpace(string(output))

	if udcEnabled != "1" {
		cleanup, err := enableUdc(ctx, s, a, oldPID)
		if err != nil {
			s.Fatal("Failed to enable dev_enable_udc: ", err)
		}
		defer cleanup()
	}

	s.Log("Checking status of arc(vm)-adbd job")
	if !(upstart.JobExists(ctx, "arc-adbd")) && !(upstart.JobExists(ctx, "arcvm-adbd")) {
		s.Fatal("Missing: arc(vm)-adbd job does not exist")
	}

	// Check status of arc-adbd
	checkJobStatusIfExist(ctx, s, "arc-adbd")

	// Check status of arcvm-adbd
	checkJobStatusIfExist(ctx, s, "arcvm-adbd")

}

func enableUdc(ctx context.Context, s *testing.State, a *arc.ARC, oldPID int32) (func(), error) {
	if err := testexec.CommandContext(ctx, "crossystem", "dev_enable_udc=1").Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to enable dev_enable_udc")
	}
	reboot(ctx, s, a, oldPID)
	cleanup := func() {
		if err := testexec.CommandContext(ctx, "crossystem", "dev_enable_udc=0").Run(testexec.DumpLogOnError); err != nil {
			testing.ContextLog(ctx, "Failed to disable dev_enable_udc: ", err)
		}
	}
	return cleanup, nil
}

func checkJobStatusIfExist(ctx context.Context, s *testing.State, job string) {
	if upstart.JobExists(ctx, job) {
		if err := upstart.WaitForJobStatus(ctx, job, upstartcommon.StartGoal, upstartcommon.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
			s.Fatalf("Failed: job %v is not up and running", job)
		}
	}
}

func reboot(ctx context.Context, s *testing.State, a *arc.ARC, oldPID int32) {
	s.Log("Running reboot command via ADB")
	if err := a.Command(ctx, "reboot").Run(); err != nil {
		s.Fatal("Failed to run reboot command via ADB: ", err)
	}

	s.Log("Waiting for old init process to exit")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := process.NewProcess(oldPID); err == nil {
			return errors.New("Old init PID still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
		s.Fatal("Failed to wait for old init process to exit: ", err)
	}

	a.Close(ctx)

	// Reboot Android and re-establish ADB connection.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		// We can assume a == nil at this point.
		s.Fatal("Failed to restart ARC: ", err)
	}

	// Additional check rebooting.
	newPID, err := arc.InitPID()
	if err != nil {
		s.Fatal("Failed to get init PID after reboot: ", err)
	}
	if newPID == oldPID {
		s.Fatal("Failure: init PID did not change")
	}
}
