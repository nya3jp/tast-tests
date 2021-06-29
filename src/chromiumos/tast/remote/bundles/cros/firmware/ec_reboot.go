// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECReboot,
		Desc:         "Checks that device will reboot when EC gets the remote requests via UART",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.NormalMode(),
		Data:         pre.Data,
		ServiceDeps:  pre.ServiceDeps,
		SoftwareDeps: pre.SoftwareDeps,
		Vars:         pre.Vars,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECReboot(ctx context.Context, s *testing.State) {
	h := s.PreValue().(*pre.Value).Helper

	type rebootTestCase struct {
		rebootName    string
		rebootCommand string
		shouldBeOn    bool
		collectBootID bool
	}

	for _, tc := range []rebootTestCase{
		{"EC reboot", "reboot", true, true},
		{"EC hard reboot", "reboot hard", true, true},
		{"EC AP-off", "reboot ap-off", false, false},
		// The last "reboot test case" is actually telling the EC to reboot
		// the machine which will eventually lead to the machine being on
		// and returning from AP-off (or any other) state no matter what.
		// We won't collect boot ID here as the previous reboot put the
		// machine in AP-off mode so it wasn't able to grab new boot ID.
		{"EC reboot to power-up", "reboot", true, false},
	} {
		var (
			oldBootID  string
			newBootID  string
			err        error
		)

		if tc.collectBootID {
			if oldBootID, err = h.Reporter.BootID(ctx); err != nil {
				s.Fatal("Failed to fetch current boot ID: ", err)
			}

			if err := h.DUT.Conn().CommandContext(ctx, "sync").Run(); err != nil {
				s.Fatalf("Failed to sync before %s: %w", tc.rebootName, err)
			}
		}

		s.Logf("Rebooting via %s", tc.rebootName)
		if err := h.Servo.RunECCommand(ctx, tc.rebootCommand); err != nil {
			s.Fatalf("Failed to reboot via %s: %w", tc.rebootName, err)
		}

		if tc.shouldBeOn {
			s.Log("Reestablishing connection to DUT")
			if err := h.DUT.WaitConnect(ctx); err != nil {
				s.Fatalf("Failed to reconnect to DUT after rebooting via %s: %w", tc.rebootName, err)
			}
		} else {
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				state, err := h.Servo.GetECSystemPowerState(ctx)
				if err != nil {
					return testing.PollBreak(errors.Wrapf(err, "Failed to get EC system power state"))
				}
				if state != "G3" {
					return errors.New("power state is " + state)
				}
				return nil
			}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
				s.Fatalf("Failed to put system off after rebooting via %s: %w", tc.rebootName, err)
			}
		}

		if tc.collectBootID {
			if newBootID, err = h.Reporter.BootID(ctx); err != nil {
				s.Fatal("Failed to fetch current boot ID: ", err)
			}
			if newBootID == oldBootID {
				s.Fatalf("Failed to reboot via %s, old boot ID is the same as new boot ID", tc.rebootName)
			}
		}

	}
}
