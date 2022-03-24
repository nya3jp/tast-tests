// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeepSleep,
		Desc:         "Estimate battery life in deep sleep state, as a replacement for manual test 1.10.1",
		Contacts:     []string{"hc.tsai@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_bringup"},
		Vars:         []string{"firmware.hibernate_time", "board", "model"},
		HardwareDeps: hwdep.D(hwdep.Battery(), hwdep.ChromeEC()),
		Timeout:      260 * time.Minute, // 4hrs 20mins
		Fixture:      fixture.NormalMode,
	})
}

// DeepSleep has been tested to pass with Servo V4, Servo V4 + ServoMicro, Servo V4 + ServoMicro in dual V4 mode.
// Verified fail on SuzyQ because it charges the battery during the test.
func DeepSleep(ctx context.Context, s *testing.State) {
	// requiredBatteryLife is the number of days the battery must last when in hibernate mode.
	const requiredBatteryLife = 100 * 24 * time.Hour
	// hibernateDelay is the time after the EC hibernate command where it still writes output.
	const hibernateDelay = 1 * time.Second
	// g3PollOptions is the time to wait for the DUT to reach G3 after power off.
	g3PollOptions := testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 3 * time.Second,
	}
	// postWakePollOptions is the time to wait for the battery after waking up from hibernate.
	postWakePollOptions := testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 250 * time.Millisecond,
	}
	// getChargerPollOptions is the time to retry the GetChargerAttached command. Unexpected EC uart logging can make it fail.
	getChargerPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 250 * time.Millisecond,
	}

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	board, _ := s.Var("board")
	model, _ := s.Var("model")
	h.OverridePlatform(ctx, board, model)

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

	_, err := h.Servo.PreferDebugHeader(ctx)
	if err != nil {
		s.Fatal("PreferDebugHeader: ", err)
	}

	// Run this code in a func to make sure the power is on at the end.
	var start time.Time
	var mahStart, max int
	func(ctx context.Context) {
		s.Log("Stopping power supply")
		if err := h.SetDUTPower(ctx, false); err != nil {
			s.Fatal("Failed to remove charger: ", err)
		}
		defer func(ctx context.Context) {
			s.Log("Connecting power supply")
			if err := h.SetDUTPower(ctx, true); err != nil {
				s.Fatal("Failed to attach charger: ", err)
			}
		}(ctx)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
				return err
			} else if attached {
				return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
			}
			return nil
		}, &getChargerPollOptions); err != nil {
			s.Fatal("Check for charger failed: ", err)
		}

		s.Log("Pressing power button to make DUT into deep sleep mode")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
			s.Fatal("Failed to set a KeypressControl by servo: ", err)
		}
		h.DisconnectDUT(ctx)

		s.Log("Waiting until power state is G3")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			state, err := h.Servo.GetECSystemPowerState(ctx)
			if err != nil {
				if strings.Contains(err.Error(), "Timed out waiting for interfaces to become available") {
					return err
				}
				return testing.PollBreak(errors.Wrap(err, "failed to get power state"))
			}
			if state != "G3" {
				return errors.New("power state is " + state)
			}
			return nil
		}, &g3PollOptions); err != nil {
			s.Fatal("Failed to wait power state to be G3: ", err)
		}

		max, err = h.Servo.GetBatteryFullChargeMAH(ctx)
		if err != nil {
			s.Fatal("Failed to get full charge mAh: ", err)
		}
		s.Logf("Battery max capacity: %dmAh", max)

		mahStart, err = h.Servo.GetBatteryChargeMAH(ctx)
		if err != nil {
			s.Fatal("Failed to get charge mAh: ", err)
		}
		start = time.Now()
		s.Logf("Battery charge: %dmAh", mahStart)

		if h.Config.Hibernate {
			s.Log("Hibernating")
			if err = h.Servo.ECHibernate(ctx, servo.UseConsole); err != nil {
				s.Fatal("Failed to run EC command: ", err)
			}
		} else {
			s.Log("Skipping hibernate, because this DUT doesn't support it")
		}

		h.DisconnectDUT(ctx)

		sleepUntil := time.Now().Add(sleep)
		s.Logf("Sleeping for %s until %s", sleep, sleepUntil)
		for time.Now().Before(sleepUntil) {
			sleepDuration := time.Until(sleepUntil)
			if sleepDuration > time.Minute {
				sleepDuration = time.Minute
			}
			if err = testing.Sleep(ctx, sleepDuration); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}
			if _, err = h.Servo.Echo(ctx, "ping"); err != nil {
				s.Log("Failed to ping servo, reconnecting: ", err)
				err = h.ServoProxy.Reconnect(ctx)
				if err != nil {
					s.Log("Failed to reconnect to servo: ", err)
				}
			}
		}
	}(ctx)

	// DUT will probably still booting at end of test.
	// pre.NormalMode().Close() will cause an extra reboot here if we don't wait.
	defer func() {
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
			s.Fatal("Failed to set a KeypressControl by servo: ", err)
		}
		newCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		h.WaitConnect(newCtx)
	}()

	mahEnd := 0
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		mahEnd, err = h.Servo.GetBatteryChargeMAH(ctx)
		return err
	}, &postWakePollOptions); err != nil {
		s.Fatal("GetBatteryChargeMAH failed after retries, is DUT off?: ", err)
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
