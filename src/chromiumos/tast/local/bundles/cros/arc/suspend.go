// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"fmt"
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
		SoftwareDeps: []string{"chrome", "android_vm"},
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

func readClocks(ctx context.Context, a *arc.ARC, readclocksPath string) (clocks, error) {
	var c clocks
	var ts unix.Timespec

	// Read host clocks
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return clocks{}, errors.Wrap(err, "clock_gettime(CLOCK_BOOTTIME) call failed")
	}
	c.hostBoot = time.Unix(0, ts.Nano())
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return clocks{}, errors.Wrap(err, "clock_gettime(CLOCK_MONOTONIC) call failed")
	}
	c.hostMono = time.Unix(0, ts.Nano())

	// Read guest clocks
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

	return c, nil
}

const allowanceSeconds = 2
const suspendSeconds = 10

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

	c0, err := readClocks(ctx, a, readclocksPath)
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

	c1, err := readClocks(ctx, a, readclocksPath)
	if err != nil {
		s.Fatal("Failed to read clocks: ", err)
	}

	// Make sure that the host has been suspended for the specified duration
	hostBootDiff := c1.hostBoot.Sub(c0.hostBoot)
	hostMonoDiff := c1.hostMono.Sub(c0.hostMono)
	hostSuspendSeconds := hostBootDiff.Seconds() - hostMonoDiff.Seconds()

	if hostSuspendSeconds < suspendSeconds-allowanceSeconds || suspendSeconds+allowanceSeconds < hostSuspendSeconds {
		s.Fatalf("Unexpected host suspend duration, got %f, want %f", hostSuspendSeconds, suspendSeconds)
	}
	s.Logf("host suspended for %f seconds", hostSuspendSeconds)

	// Wait a bit for the virtual suspend injection happens
	testing.Sleep(ctx, time.Second)

	// Check if the same amount of suspend time is injected to the guest
	guestBootDiff := c1.guestBoot.Sub(c0.guestBoot)
	guestMonoDiff := c1.guestMono.Sub(c0.guestMono)
	guestSuspendSeconds := guestBootDiff.Seconds() - guestMonoDiff.Seconds()

	if guestSuspendSeconds < suspendSeconds-allowanceSeconds || suspendSeconds+allowanceSeconds < guestSuspendSeconds {
		s.Fatalf("Suspend time was not injected to the guest properly, got %f, want %f", guestSuspendSeconds, suspendSeconds)
	}
	s.Logf("%f seconds of suspend time was injected", guestSuspendSeconds)
}
