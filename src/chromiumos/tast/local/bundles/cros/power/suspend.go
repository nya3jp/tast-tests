// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"time"

	"chromiumos/tast/local/clock"
	"chromiumos/tast/local/dbusutil"
	pow "chromiumos/tast/local/power"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Suspend,
		Desc:         "Checks that powerd is able to suspend the system",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"powerd"},
	})
}

func Suspend(s *testing.State) {
	const (
		// Amount of time to suspend.
		duration = 10 * time.Second

		// Minimum time difference expected between monotonic and boot-time
		// clocks while suspending. If the clocks increased at the same rate,
		// then we probably didn't actually suspend.
		minClockDiff = 5 * time.Second

		// Suspend-related D-Bus signals emitted by powerd.
		suspendImminent = dbusutil.PowerManagerInterface + ".SuspendImminent"
		suspendDone     = dbusutil.PowerManagerInterface + ".SuspendDone"
	)

	// Start listening for D-Bus signals from powerd.
	sw, err := dbusutil.NewSignalWatcherForSystemBus(s.Context(), dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.PowerManagerPath,
		Interface: dbusutil.PowerManagerInterface,
	})
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close()

	// Filter out the signals we care about.
	ch := make(chan *dbus.Signal)
	go func() {
		for sig := range sw.Signals {
			if sig.Name == suspendImminent || sig.Name == suspendDone {
				ch <- sig
			}
		}
		close(ch)
	}()

	s.Log("Asking powerd to suspend for ", duration)
	monoStart := clock.Now(clock.Monotonic)
	bootStart := clock.Now(clock.BootTime)
	if err := pow.Suspend(pow.SuspendOptions{Duration: duration}); err != nil {
		s.Fatal("Failed to suspend: ", err)
	}

	monoEnd := clock.Now(clock.Monotonic) - monoStart
	bootEnd := clock.Now(clock.BootTime) - bootStart
	s.Log("Suspend request finished")
	s.Log("Elapsed CLOCK_MONOTONIC time: ", monoEnd.Round(time.Millisecond))
	s.Log("Elapsed CLOCK_BOOTTIME time: ", bootEnd.Round(time.Millisecond))
	if diff := bootEnd - monoEnd; diff < minClockDiff {
		s.Errorf("Clocks differed by %v; expected roughly %v -- did the system not suspend?",
			diff, duration)
	}

	// Check that we received the D-Bus signals in the expected order.
	// TODO(derat): Decode protobufs and check signal contents.
	for _, m := range []string{suspendImminent, suspendDone} {
		s.Logf("Checking for %s D-Bus signal from powerd", m)
		select {
		case sig := <-ch:
			if sig.Name == m {
				s.Logf("Got expected %s signal", sig.Name)
			} else {
				s.Errorf("Got unexpected %s signal", sig.Name)
			}
		case <-s.Context().Done():
			s.Fatal("Didn't get %s signal: %v", m, s.Context().Err())
		}
	}
}
