// Copyright 2022 The ChromiumOS Authors.
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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoPowerBehavior,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Wilco devices can wake from pressing power, but not from connecting AC",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"wilco"},
		Fixture:      fixture.NormalMode,
	})
}

func WilcoPowerBehavior(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	d := s.DUT()

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	s.Log("Removing charger")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Unable to remove charger: ", err)
	}

	s.Logf("Pressing power button for %s to put DUT in deep sleep", h.Config.HoldPwrButtonPowerOff)
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
		s.Fatal("Failed to hold power button: ", err)
	}

	s.Log("Waiting for DUT to power OFF")
	waitUnreachableCtx, cancelUnreachable := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelUnreachable()

	if err := d.WaitUnreachable(waitUnreachableCtx); err != nil {
		s.Fatal("DUT did not power down: ", err)
	}

	s.Log("Verifying DUT's AP is off")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		apState, err := h.Servo.RunCR50CommandGetOutput(ctx, "ccdstate", []string{`AP:(\s+\w+)`})
		if err != nil {
			return errors.Wrap(err, "failed to run cr50 command")
		}

		if strings.TrimSpace(apState[0][1]) != "off" {
			return errors.Wrapf(err, "unexpected AP state: %s", strings.TrimSpace(apState[0][1]))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Failed to verify DUT's AP is off: ", err)
	}

	s.Log("Connecting charger")
	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Unable to connect charger: ", err)
	}

	// Connecting charger would not wake Wilco devices from deep sleep.
	// Expect a timeout in waiting for DUT to reconnect.
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelWaitConnect()
	err := d.WaitConnect(waitConnectCtx)
	switch err.(type) {
	case nil:
		s.Fatal("DUT woke up unexpectedly")
	default:
		if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
			s.Fatal("Unexpected error occurred: ", err)
		}
	}
	s.Log("DUT remained offline")

	s.Logf("Pressing power button for %s seconds to wake DUT", servo.Dur(h.Config.HoldPwrButtonPowerOn))
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOn)); err != nil {
		s.Fatal("Failed to press power key via servo: ", err)
	}

	waitConnectFromPressPowerCtx, cancelWaitConnectFromPressPower := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelWaitConnectFromPressPower()
	s.Log("Checking that DUT wakes up from a press on power button")
	if err := d.WaitConnect(waitConnectFromPressPowerCtx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
}
