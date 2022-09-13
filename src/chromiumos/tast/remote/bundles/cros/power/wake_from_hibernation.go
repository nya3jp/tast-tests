// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WakeFromHibernation,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Wake from hibernation by AC plug",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
	})
}

func WakeFromHibernation(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	s.Log("Stopping power supply")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to remove charger: ", err)
	}
	defer h.SetDUTPower(cleanupCtx, true)

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to log in to chrome: ", err)
	}

	s.Log("Capturing EC log")
	if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
		s.Fatal("Failed to capture EC UART: ", err)
	}
	defer h.Servo.SetOnOff(cleanupCtx, servo.ECUARTCapture, servo.Off)

	// Read the uart stream just to make sure there isn't buffered data.
	if _, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
		s.Fatal("Failed to read UART: ", err)
	}

	// Run EC command to put DUT in hibernate.
	if err := h.Servo.RunECCommand(ctx, "hibdelay 5"); err != nil {
		s.Fatal("Failed to run EC command: ", err)
	}

	powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to execute shutdown command: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		lines, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to read UART"))
		}
		for _, l := range strings.Split(lines, "\r\n") {
			if strings.Contains(l, "Hibernate due to G3 idle") {
				return nil
			}
		}
		return errors.New("failed to check hibernate in EC logs")
	}, &testing.PollOptions{Interval: 200 * time.Millisecond, Timeout: time.Minute}); err != nil {
		s.Error("Failed to parse EC logs and verify hibernation: ", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	s.Log("Waiting for DUT unreachable")
	if err := dut.WaitUnreachable(waitCtx); err != nil {
		s.Fatal("Failed wait for unreachable: ", err)
	}
}
