// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECTabletModeStress,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Stresses EC tablet mode messaging to validate it works reliably",
		Contacts:     []string{"timvp@google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_ec"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.FormFactor(hwdep.Convertible, hwdep.Chromeslate, hwdep.Detachable)),
	})
}

func ECTabletModeStress(ctx context.Context, s *testing.State) {
	dut := s.DUT()

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	defer func() {
		if err := h.Servo.RunECCommand(ctx, "tabletmode reset"); err != nil {
			s.Fatal("Failed to restore DUT to the original tabletmode setting: ", err)
		}
	}()

	// Chrome instance is necessary to make the EC `tabletmode` command work.
	s.Log("Login to Chrome")
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}
	// Give the DUT time to login and load up Chrome.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal(err, "error in sleeping for 1 second after logging in")
	}

	repeatedSteps := func() {
		// Run EC command to put DUT in tablet mode.
		if err := h.Servo.RunECCommand(ctx, "tabletmode on"); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}

		// Allow the AP to handle the EC host command.
		tabletMode := false
		for i := 0; i < 10; i++ {
			out, err := dut.Conn().CommandContext(ctx, "cat", "/var/lib/power_manager/tablet_mode").Output()
			if err != nil || len(string(out)) != 1 {
				s.Logf("Set tablet mode: Failed to `cat /var/lib/power_manager/tablet_mode`: err = %q, tablet_mode = %q", err, out)
				if err := testing.Sleep(ctx, 250*time.Millisecond); err != nil {
					s.Fatal(err, "error in sleeping after `cat /var/lib/power_manager/tablet_mode`")
				}
				continue
			}
			tabletModeEnabled := "1"
			tabletMode = string(out) == tabletModeEnabled
			if tabletMode {
				break
			}
		}
		if !tabletMode {
			s.Fatal("Failed to set tablet mode")
		}

		// Simulate the lid opening (b/236418065#comment4)
		if err := testing.Sleep(ctx, 7*time.Millisecond); err != nil {
			s.Fatal(err, "error in sleeping after tabletmode on")
		}

		// Run EC command to put DUT in laptop mode.
		if err := h.Servo.RunECCommand(ctx, "tabletmode off"); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}

		// Allow the AP to handle the EC host command.
		laptopMode := false
		for i := 0; i < 10; i++ {
			out, err := dut.Conn().CommandContext(ctx, "cat", "/var/lib/power_manager/tablet_mode").Output()
			if err != nil || len(string(out)) != 1 {
				s.Logf("Set laptop mode: Failed to `cat /var/lib/power_manager/tablet_mode`: err = %q, tablet_mode = %q", err, out)
				if err := testing.Sleep(ctx, 250*time.Millisecond); err != nil {
					s.Fatal(err, "error in sleeping after `cat /var/lib/power_manager/tablet_mode`")
				}
				continue
			}
			tabletModeDisabled := "0"
			laptopMode = string(out) == tabletModeDisabled
			if laptopMode {
				break
			}
		}
		if !laptopMode {
			s.Fatal("Failed to set laptop mode")
		}
	}

	numIterations := 250
	for i := 1; i <= numIterations; i++ {
		s.Log("Iteration: ", i)
		repeatedSteps()
		testing.Sleep(ctx, 250*time.Millisecond)
	}
}
