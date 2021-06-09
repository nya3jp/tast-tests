// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeepSleep,
		Desc:         "Estimate battery life in deep sleep state, as a replacement for manual test 1.10.1",
		Contacts:     []string{"hc.tsai@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Vars:         []string{"servo", "firmware.hibernate_time"},
		SoftwareDeps: []string{"crossystem"},
		Data:         []string{firmware.ConfigFile},
		HardwareDeps: hwdep.D(hwdep.Battery(), hwdep.ChromeEC()),
		Timeout:      260 * time.Minute, // 4hrs 20mins
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Pre:          pre.NormalMode(),
	})
}

func DeepSleep(ctx context.Context, s *testing.State) {
	const requiredBatteryLife = 100 * 24 * time.Hour
	h := s.PreValue().(*pre.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	// By default the DUT hibernates for 4 hours. Reduce the duration by
	// providing an optional variable "firmware.hibernate_time".
	sleep := 4 * time.Hour
	if v, ok := s.Var("firmware.hibernate_time"); ok {
		var err error
		if sleep, err = time.ParseDuration(v); err != nil {
			s.Fatalf("Failed to parse duration %s: %v", v, err)
		}
	}

	s.Log("Stopping power supply from servo")
	if err := h.Servo.SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Failed to set servo role: ", err)
	}

	s.Log("Pressing power button to make DUT into deep sleep mode")
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		s.Fatal("Failed to set a KeypressControl by servo: ", err)
	}
	h.DUT.Disconnect(ctx)

	s.Log("Waiting until power state is G3")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		state, err := h.Servo.GetECSystemPowerState(ctx)
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

	max, err := h.Servo.GetBatteryFullChargeMAH(ctx)
	if err != nil {
		s.Fatal("Failed to get full charge mAh: ", err)
	}
	s.Logf("Battery max capacity: %dmAh", max)

	mahStart, err := h.Servo.GetBatteryChargeMAH(ctx)
	if err != nil {
		s.Fatal("Failed to get charge mAh: ", err)
	}
	start := time.Now()
	s.Logf("Battery charge: %dmAh", mahStart)

	s.Log("Hibernating")
	if err = h.Servo.RunECCommand(ctx, "hibernate"); err != nil {
		s.Fatal("Failed to run EC command: ", err)
	}

	if out, err := h.Servo.RunECCommandGetOutput(ctx, "version", []string{".+"}); err == nil {
		s.Logf("Got %v expected error", out)
		s.Fatal("EC is still active after hibernate")
	} else {
		if !strings.Contains(err.Error(), "No data was sent from the pty") {
			s.Fatal("Unexpected EC error: ", err)
		}
	}

	s.Log("Sleeping for ", sleep)
	if err = testing.Sleep(ctx, sleep); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	s.Log("Waking up DUT with short power key press")
	if err = h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
		s.Fatal("Failed to press power key: ", err)
	}

	mahEnd, err := h.Servo.GetBatteryChargeMAH(ctx)
	if err != nil {
		s.Fatal("Failed to get charge mAh: ", err)
	}
	s.Logf("Battery charge: %dmAh", mahEnd)

	var (
		dur   = time.Since(start)
		usage = mahEnd - mahStart
	)
	s.Logf("Battery Usage: %dmAh in %s", usage, dur)

	if usage > 0 {
		days := float64(max) / (float64(usage) / dur.Seconds()) / (24 * time.Hour.Seconds())
		s.Logf("Estimate Battery Life: %f day(s)", days)
		if days < requiredBatteryLife.Hours()/24 {
			s.Errorf("Estimate Battery Life(%f) less than 100 days", days)
		}
	} else {
		// If less than 1 mAh is consumed during the test, we still won't know it passed unless we
		// ran for long enough for it not to be a rounding error. This does assume that
		// the battery is capable of reporting charge in increments of 1mAh, which might not
		// be true.
		minSleepTime := time.Duration(requiredBatteryLife.Nanoseconds() / int64(max))
		if minSleepTime > dur {
			s.Errorf("Inconclusive, please run test for at least %s", minSleepTime)
		}
	}
}
