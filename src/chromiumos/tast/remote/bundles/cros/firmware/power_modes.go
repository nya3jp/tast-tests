// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

type powerModeTestParams struct {
	powermode firmware.ResetType
}

const (
	coldReset firmware.ResetType = "coldreset"
	shutDown  firmware.ResetType = "shutdown"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PowerModes,
		Desc:         "Verifies that system comes back after shutdown and coldreset",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com", "cros-fw-engprod@google.com"},
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Vars: []string{"servo",
			"firmware.mode", // Optional. Expecting "tablet". By default firmware.mode will be "clamshell".
		},
		Attr:    []string{"group:firmware", "firmware_experimental"},
		Fixture: fixture.NormalMode,
		Params: []testing.Param{{
			Name: "coldreset",
			Val:  powerModeTestParams{powermode: coldReset},
		}, {
			Name: "shutdown",
			Val:  powerModeTestParams{powermode: shutDown},
		},
		},
	})
}

// PowerModes verifies that system comes back after shutdown and coldreset.
func PowerModes(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	dut := s.DUT()
	testOpt := s.Param().(powerModeTestParams)

	// Servo setup.
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed opening servo: ", err)
	}

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initHys := m[2]

	defaultMode := "clamshell"
	if mode, ok := s.Var("firmware.mode"); ok {
		defaultMode = mode
	}

	if defaultMode == "tablet" {
		// Set tabletModeAngle to 0 to force the DUT into tablet mode.
		testing.ContextLog(ctx, "Put DUT into tablet mode")
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(); err != nil {
			s.Fatal("Failed to set DUT into tablet mode: ", err)
		}
	}

	defer func(ctx context.Context) {
		s.Log("Performing Cleanup")
		if !dut.Connected(ctx) {
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
				s.Fatal("Failed to set powerstate to ON at cleanup: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	screenLockService := ui.NewScreenLockServiceClient(cl.Conn)
	if _, err := screenLockService.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to login chrome: ", err)
	}
	defer screenLockService.CloseChrome(ctx, &empty.Empty{})

	if testOpt.powermode == "coldreset" {
		s.Log("Performing cold reset")
		if err := dut.Conn().CommandContext(ctx, "ectool", "reboot_ec", "cold", "at-shutdown").Run(); err != nil {
			s.Fatal("Failed to execute ectool reboot_ec cmd: ", err)
		}

		if err := dut.Conn().CommandContext(ctx, "shutdown", "-h", "now").Run(); err != nil {
			s.Fatal("Failed to execute shutdown command: ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to wake up DUT: ", err)
		}

		if err := validatePrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Previous Sleep state is not 5: ", err)
		}
	}

	if testOpt.powermode == "shutdown" {
		s.Log("Performing shutdown")
		if err := dut.Conn().CommandContext(ctx, "shutdown", "-h", "now").Run(); err != nil {
			s.Fatal("Failed to run shutdown command: ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to shutdown DUT: ", err)
		}
		s.Log("Power Normal Pressing")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
			s.Fatal("Failed to set powerstate to ON: ", err)
		}
		cCtx, cancel := ctxutil.Shorten(ctx, time.Minute)
		defer cancel()
		// Setting power state ON, once again if system fails to boot.
		if err := dut.WaitConnect(cCtx); err != nil {
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
				s.Fatal("Failed to set powerstate to ON: ", err)
			}
			if err := dut.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to wake up DUT: ", err)
			}
		}
		if err := validatePrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Previous Sleep state is not 5: ", err)
		}
	}
}

// validatePrevSleepState sleep state from cbmem command output.
func validatePrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	const (
		// Command to check previous sleep state.
		prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	)
	out, err := dut.Conn().CommandContext(ctx, "sh", "-c", prevSleepStateCmd).Output()
	if err != nil {
		return err
	}
	if count, err := strconv.Atoi(strings.Split(strings.Replace(string(out), "\n", "", -1), " ")[1]); err != nil {
		return err
	} else if count != sleepStateValue {
		return errors.Errorf("previous sleep state must be %d", sleepStateValue)
	}
	return nil
}
