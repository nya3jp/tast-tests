// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

type suspendConfig struct {
	// Run suspend this many times
	numTrials int
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Suspend,
		Desc: "Checks the behavior of ARC around suspend/resume",
		Contacts: []string{
			"hikalium@chromium.org",
			"cros-platform-kernel-core@google.com",
		},
		SoftwareDeps: []string{"chrome", "android_vm" /*, "virtual_susupend_injection"*/},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "arcBooted",
		Timeout:      5 * time.Minute,
	})
}

type clocks struct {
	hostBoot  time.Time
	hostMono  time.Time
	guestBoot time.Time
	guestMono time.Time
}

type readclocksTimespec struct {
	Seconds     int64 `json:"tv_sec"`
	NanoSeconds int64 `json:"tv_nsec"`
}

type readclocksOutput struct {
	Boot readclocksTimespec `json:"CLOCK_BOOTTIME"`
	Mono readclocksTimespec `json:"CLOCK_MONOTONIC"`
	TSC  int64
}

func readClocks(ctx context.Context, s *testing.State, a *arc.ARC, readclocksPath string) (clocks, error) {
	var c clocks
	var ts unix.Timespec

	// Read host clocks
	s.Log("Reading host clock")
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return clocks{}, errors.Wrap(err, "clock_gettime(CLOCK_BOOTTIME) call failed")
	}
	c.hostBoot = time.Unix(0, ts.Nano())
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return clocks{}, errors.Wrap(err, "clock_gettime(CLOCK_MONOTONIC) call failed")
	}
	c.hostMono = time.Unix(0, ts.Nano())

	// Read guest clocks
	s.Log("Reading guest clock")
	output, err := a.Command(ctx, readclocksPath).Output()
	if err != nil {
		return clocks{}, errors.Wrap(err, "failed to run readclocks binary")
	}
	var guestClocks readclocksOutput
	err = json.Unmarshal(output, &guestClocks)
	if err != nil {
		return clocks{}, errors.Wrap(err, "failed to parse readclocks output")
	}
	c.guestMono = time.Unix(guestClocks.Mono.Seconds, guestClocks.Mono.NanoSeconds)
	c.guestBoot = time.Unix(guestClocks.Boot.Seconds, guestClocks.Boot.NanoSeconds)

	s.Log("readClocks done")

	return c, nil
}

const suspendSeconds = 10 // Longer than watchdog thresholds in the guest kernel
const hostSuspendAllowanceSeconds = suspendSeconds / 2
const boottimeDiffAllowanceSeconds = 1

func Suspend(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC

	readclocksPath, err := a.PushFileToTmpDir(ctx, "/usr/local/libexec/tast/helpers/local/cros/arc.Suspend.readclocks")
	if err != nil {
		s.Fatal("Failed to push test binary to ARC: ", err)
	}
	defer a.Command(ctx, "rm", readclocksPath).Run()

	if err := a.Command(ctx, "chmod", "0777", readclocksPath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to change test binary permissions: ", err)
	}

	numTrials := 10
	for i := 0; i < numTrials; i++ {
		s.Logf("Trial %d/%d", i+1, numTrials)
		c0, err := readClocks(ctx, s, a, readclocksPath)
		if err != nil {
			s.Fatal("Failed to read clocks: ", err)
		}

		// Put the machine into suspend
		s.Logf("Suspending the machine for %d seconds", suspendSeconds)
		err = testexec.CommandContext(ctx, "powerd_dbus_suspend", "--delay=0",
			fmt.Sprintf("--suspend_for_sec=%d", suspendSeconds),
			fmt.Sprintf("--wakeup_timeout=%d", suspendSeconds+30)).Run()
		if err != nil {
			s.Fatal("powerd_dbus_suspend failed: ", err)
		}
		s.Log("Resumed")

		// Wait a bit for the virtual suspend injection happens
		s.Log("Sleep 2 seconds")
		testing.Sleep(ctx, 2*time.Second)

		c1, err := readClocks(ctx, s, a, readclocksPath)
		if err != nil {
			s.Fatal("Failed to read clocks: ", err)
		}

		// Make sure that the host has been suspended for the specified duration
		hostBootDiff := c1.hostBoot.Sub(c0.hostBoot)
		hostMonoDiff := c1.hostMono.Sub(c0.hostMono)
		hostSuspendSeconds := hostBootDiff.Seconds() - hostMonoDiff.Seconds()

		if math.Abs(hostSuspendSeconds-suspendSeconds) > hostSuspendAllowanceSeconds {
			s.Fatalf("Unexpected host suspend duration, got %f, want %d", hostSuspendSeconds, suspendSeconds)
		}
		s.Logf("host suspended for %f seconds", hostSuspendSeconds)

		// Check if the same amount of suspend time is injected to the guest
		guestBootDiff := c1.guestBoot.Sub(c0.guestBoot)
		guestMonoDiff := c1.guestMono.Sub(c0.guestMono)
		guestSuspendSeconds := guestBootDiff.Seconds() - guestMonoDiff.Seconds()

		if math.Abs(guestSuspendSeconds-hostSuspendSeconds) > boottimeDiffAllowanceSeconds {
			s.Fatalf("Suspend time was not injected to the guest properly, got %f, want %f", guestSuspendSeconds, hostSuspendSeconds)
		}
		s.Logf("%f seconds of suspend time was injected to the guest", guestSuspendSeconds)
	}
}
