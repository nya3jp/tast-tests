// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/ptypes/empty"
)

type powerMode struct {
	// Define powermodes like shutdown, coldReset
	powermode string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PowerModes,
		Desc:         "Verifies that system comes back after shutdown and coldreset",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:mainline", "group:labqual"},
		Params: []testing.Param{{
			Name: "cold_reset",
			Val:  powerMode{powermode: "coldReset"},
		},
			{
				Name: "shutdown",
				Val:  powerMode{powermode: "shutdown"},
			},
		},
	})
}

// PowerModes verifies that system comes back after  shutdown and coldreset.
func PowerModes(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	testOpt := s.Param().(powerMode)
	// Servo setup.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Cleanup.
	defer func(ctx context.Context) {
		s.Log("Performing clean up..")
		if !dut.Connected(ctx) {
			if err := testexec.CommandContext(ctx, "dut-control", "cold_reset:on", "sleep:0.5", "cold_reset:off").Run(); err != nil {
				s.Error("Unable to perform cold reset", err)
			}
		} else {
			s.Log("DUT is UP..")
		}
	}(ctx)

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome : ", err)
	}

	if testOpt.powermode == "coldReset" {
		s.Log("Performing cold reset..")
		if err := dut.Command("ectool", "reboot_ec", "cold", "at-shutdown").Run(ctx); err != nil {
			s.Fatal("Failed to execute ectool reboot_ec cmd : ", err)
		}

		if err := dut.Command("shutdown", "-h", "now").Run(ctx); err != nil {
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
		if err := dut.Command("shutdown", "-h", "now").Run(ctx); err != nil {
			s.Fatal("Failed to run shutdown command : ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatalf("Failed to shutdown DUT within error:", err)
		}
		s.Log("Power Normal Pressing")
		if err := pwrBtnPress(ctx); err != nil {
			s.Fatal("Failed to power normal press:", err)
		}
		cCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		if err := dut.WaitConnect(cCtx); err != nil {
			if err := pwrBtnPress(ctx); err != nil {
				s.Fatal("Failed to power normal press:", err)
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
		// Command to check previous sleep state
		prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	)
	out, err := dut.Command("sh", "-c", prevSleepStateCmd).Output(ctx)
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

// Run DUT control command for power_normal_press.
func pwrBtnPress(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "dut-control", "power_key:press").Run(); err != nil {
		return err
	}
	return nil
}
