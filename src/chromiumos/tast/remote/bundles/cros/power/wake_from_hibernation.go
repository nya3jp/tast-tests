// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Desc:         "Wake from hibernation by lid close/Open",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
	})
}

func WakeFromHibernation(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	s.Log("Stopping power supply")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to remove charger: ", err)
	}

	// Perform a Chrome login.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	s.Log("Capturing EC log")
	if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
		s.Fatal("Failed to capture EC UART: ", err)
	}

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
		if lines, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
			s.Fatal("Failed to read UART: ", err)
		} else if lines != "" {
			for _, l := range strings.Split(lines, "\r\n") {
				if strings.Contains(l, "Hibernate due to G3 idle") {
					return nil
				}
			}
		}
		return errors.New("failed to check hibernate in EC logs")
	}, &testing.PollOptions{Interval: time.Millisecond * 200, Timeout: 60 * time.Second}); err != nil {
		s.Error("Failed to parse EC logs and verify hibernation: ", err)
	}

	s.Log("Closing lid, waiting for DUT to become unreachable")
	if err := h.Servo.SetString(ctx, "lid_open", "no"); err != nil {
		s.Fatal("Failed to close lid: ", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	s.Log("Waiting for DUT unreachable")
	if err := dut.WaitUnreachable(waitCtx); err != nil {
		s.Fatal("Failed wait for unreachable: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		s.Log("Open Lid in cleanup")
		if err := h.Servo.SetString(ctx, "lid_open", "yes"); err != nil {
			s.Fatal("Failed to open lid: ", err)
		}
		if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.Off); err != nil {
			s.Error("Failed to disable capture EC UART: ", err)
		}
		s.Log("Plugging back power supply")
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Error("Failed to plug charger: ", err)
		}
	}(cleanupCtx)
}
