// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/remote/firmware"
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
		Vars:         []string{"servo"},
		Pre:          pre.NormalMode(),
		Data:         []string{firmware.ConfigFile},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECReboot(ctx context.Context, s *testing.State) {
	var (
		oldBootID  string
		newBootID  string
		powerState string
		err        error
	)
	const (
		reconnectTimeout = 3 * time.Minute
	)

	h := s.PreValue().(*pre.Value).Helper

	type rebootTestCase struct {
		rebootMode    string
		rebootCommand string
		shouldBeOn    bool
		collectBootID bool
	}
	for _, tc := range []rebootTestCase{
		{"EC reboot", "reboot", true, true},
		{"EC hard reboot", "reboot hard", true, true},
		{"EC AP-off", "reboot ap-off", false, false},
		{"EC reboot to power-up", "reboot", true, false},
	} {
		if tc.collectBootID {
			if oldBootID, err = h.Reporter.BootID(ctx); err != nil {
				s.Fatal("Failed to fetch current boot ID: ", err)
			}
		
			if err := h.DUT.Conn().CommandContext(ctx, "sync").Run(); err != nil {
				s.Fatalf("Failed to sync before %s: %w", tc.rebootMode, err)
			}
		}

		s.Logf("Rebooting via %s", tc.rebootMode)
		if err := h.Servo.RunECCommand(ctx, tc.rebootCommand); err != nil {
			s.Fatalf("Failed to reboot via %s: %w", tc.rebootMode, err)
		}

		if tc.shouldBeOn {
			s.Log("Reestablishing connection to DUT")
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				return h.DUT.WaitConnect(ctx)
			}, &testing.PollOptions{Timeout: reconnectTimeout}); err != nil {
				s.Fatalf("Failed to reconnect to DUT after rebooting via %s: %w", tc.rebootMode, err)
			}
		} else {
			if powerState, err = h.Servo.GetECSystemPowerState(ctx); err != nil {
				s.Fatal("Failed to get EC system power state: ", err)
			}
			if powerState != "G3" {
				s.Fatalf("Failed to reboot via %s, power state is %s", tc.rebootMode, powerState)
			}

		}

		if tc.collectBootID {
			if newBootID, err = h.Reporter.BootID(ctx); err != nil {
				s.Fatal("Failed to fetch current boot ID: ", err)
			}
			if newBootID == oldBootID {
				s.Fatalf("Failed to reboot via %s, old boot ID is the same as new boot ID", tc.rebootMode)
			}
		}

	}
}
