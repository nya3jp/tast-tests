// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeepSleep,
		Desc:         "Estimate battery life in deep sleep state, as a replacement for manual test 1.10.1",
		Contacts:     []string{"hc.tsai@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_slow"},
		Vars:         []string{"servo", "firmware.hibernate_time"},
		SoftwareDeps: []string{"crossystem"},
		HardwareDeps: hwdep.D(hwdep.Battery(), hwdep.ChromeEC()),
		Timeout:      260 * time.Minute, // 4hrs 20mins
	})
}

func DeepSleep(ctx context.Context, s *testing.State) {
	d := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// By default the DUT hibernates for 4 hours. Reduce the duration by
	// providing an optional variable "firmware.hibernate_time".
	sleep := 4 * time.Hour
	if v, ok := s.Var("firmware.hibernate_time"); ok {
		if sleep, err = time.ParseDuration(v); err != nil {
			s.Fatalf("Failed to parse duration %s: %v", v, err)
		}
	}

	s.Log("Stopping power supply from servo")
	if err = pxy.Servo().SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Failed to set servo role: ", err)
	}

	s.Log("Long pressing power button to make DUT into deep sleep mode")
	if err = pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
		s.Fatal("Failed to set a KeypressControl by servo: ", err)
	}

	s.Log("Waiting until power state is G3")
	if err = testing.Poll(ctx, func(ctx context.Context) error {
		state, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get power state"))
		}
		if state != "G3" {
			return errors.New("power state is " + state)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  time.Minute,
		Interval: 3 * time.Second,
	}); err != nil {
		s.Fatal("Failed to wait power state to be G3: ", err)
	}

	max, err := pxy.Servo().GetBatteryFullChargeMAH(ctx)
	if err != nil {
		s.Fatal("Failed to get full charge mAh: ", err)
	}
	s.Logf("Battery max capacity: %dmAh", max)

	mahStart, err := pxy.Servo().GetBatteryChargeMAH(ctx)
	if err != nil {
		s.Fatal("Failed to get charge mAh: ", err)
	}
	start := time.Now()
	s.Logf("Battery charge: %dmAh", mahStart)

	s.Log("Hibernating")
	if err = pxy.Servo().RunECCommand(ctx, "hibernate"); err != nil {
		s.Fatal("Failed to run EC command: ", err)
	}

	s.Log("Sleeping for ", sleep)
	if err = testing.Sleep(ctx, sleep); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Waking up DUT by supplying power from servo")
	if err = pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to set servo role: ", err)
	}

	s.Log("Reconnecting to DUT")
	if err = d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	mahEnd, err := pxy.Servo().GetBatteryChargeMAH(ctx)
	if err != nil {
		s.Fatal("Failed to get charge mAh: ", err)
	}
	s.Logf("Battery charge: %dmAh", mahEnd)

	var (
		dur   = time.Since(start).Seconds()
		usage = mahEnd - mahStart
	)
	s.Logf("Battery Usage: %dmAh in %f seconds", usage, dur)

	if usage > 0 {
		days := float64(max) / (float64(usage) / dur) / (24 * time.Hour.Seconds())
		s.Logf("Estimate Battery Life: %f day(s)", days)
		if days < 100 {
			s.Error("Estimate Battery Life less than 100 days")
		}
	}
}
