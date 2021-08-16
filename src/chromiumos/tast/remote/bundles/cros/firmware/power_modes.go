// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/ptypes/empty"
)

type powerModeTestParams struct {
	// Define powermodes like shutdown, coldReset
	powermode string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PowerModes,
		Desc:         "Verifies that system comes back after shutdown and coldreset",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService", "tast.cros.firmware.BiosService"},
		SoftwareDeps: []string{"chrome", "reboot", "crossystem", "flashrom"},
		Data:         []string{firmware.ConfigFile},
		Vars:         []string{"servo"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Params: []testing.Param{{
			Name: "coldreset",
			Pre:  pre.NormalMode(),
			Val:  powerModeTestParams{powermode: "coldreset"},
		}, {
			Name: "shutdown",
			Pre:  pre.NormalMode(),
			Val:  powerModeTestParams{powermode: "shutdown"},
		},
		},
	})
}

// PowerModes verifies that system comes back after shutdown and coldreset.
func PowerModes(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	testOpt := s.Param().(powerModeTestParams)
	pv := s.PreValue().(*pre.Value)
	h := pv.Helper
	// Servo setup.
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Error opening servo: ", err)
	}

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome : ", err)
	}

	if testOpt.powermode == "coldreset" {
		s.Log("Performing cold reset..")
		if err := dut.Conn().Command("ectool", "reboot_ec", "cold", "at-shutdown").Run(ctx); err != nil {
			s.Fatal("Failed to execute ectool reboot_ec cmd : ", err)
		}

		if err := dut.Conn().Command("shutdown", "-h", "now").Run(ctx); err != nil {
			s.Fatal("Failed to execute shutdown command : ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to wake up DUT", err)
		}

		if err := validatePrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Previous Sleep state is not 5 : ", err)
		}
	}

	if testOpt.powermode == "shutdown" {
		s.Log("Performing shutdown...")
		if err := dut.Conn().Command("shutdown", "-h", "now").Run(ctx); err != nil {
			s.Fatal("Failed to run shutdown command : ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatalf("Failed to shutdown DUT within error:", err)
		}
		s.Log("Power Normal Pressing")
		if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
			s.Fatal("Failed to set powerstate to ON : ", err)
		}
		cCtx, cancel := ctxutil.Shorten(ctx, time.Minute)
		defer cancel()
		// Setting power state ON, once again if system fails to boot.
		if err := dut.WaitConnect(cCtx); err != nil {
			if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
				s.Fatal("Failed to set powerstate to ON : ", err)
			}
			if err := dut.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to wake up DUT", err)
			}
		}
		if err := validatePrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Previous Sleep state is not 5 : ", err)
		}
	}
}

// Validate previous sleep state from cbmem command output.
func validatePrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	const (
		// Command to check previous sleep state.
		prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	)
	out, err := dut.Conn().Command("sh", "-c", prevSleepStateCmd).Output(ctx)
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(strings.Split(strings.Replace(string(out), "\n", "", -1), " ")[1])
	if err != nil {
		return err
	}
	if count != sleepStateValue {
		return errors.Errorf("Previous sleep state must be %d", sleepStateValue)
	}
	return nil
}
