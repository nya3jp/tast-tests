// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

type shutdownModeTestParams struct {
	shutdownmode string
}

const (
	powerButton string = "powerbutton"
	powerOFF    string = "poweroff"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShutdownMode,
		Desc:         "Verifies that system comes back after power button press and poweroff",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "powerbutton",
			Val:  shutdownModeTestParams{shutdownmode: powerButton},
		}, {
			Name: "poweroff",
			Val:  shutdownModeTestParams{shutdownmode: powerOFF},
		},
		},
	})
}

func ShutdownMode(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	testOpt := s.Param().(shutdownModeTestParams)
	// Servo setup
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Logging into chrome
	chromeLogin := func() {
		s.Log("Login to Chrome")
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)
		client := security.NewBootLockboxServiceClient(cl.Conn)
		if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start Chrome : ", err)
		}
	}
	chromeLogin()
	pwrONDUT := func() {
		s.Log("Power Normal Pressing")
		if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
			s.Fatal("Failed to power normal press : ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			s.Log("Failed to wake up DUT. Retrying")
			if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
				s.Fatal("Failed to power normal press : ", err)
			}
			if err := dut.WaitConnect(ctx); err != nil {
				s.Fatal("Failed to wait connect DUT : ", err)
			}
		}
	}
	// Cleanup.
	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			pwrONDUT()
		}
	}(ctx)
	if testOpt.shutdownmode == "powerbutton" {
		if err := pxy.Servo().SetString(ctx, "power_key", "long_press"); err != nil {
			s.Fatal("Failed to power long press: ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			if err := pxy.Servo().SetString(ctx, "power_key", "long_press"); err != nil {
				s.Fatal("Failed to power long press : ", err)
			}
			if err := dut.WaitUnreachable(ctx); err != nil {
				s.Fatal("Failed to shutdown DUT: ", err)
			}
		}
		if err := validateG3PowerSate(ctx, pxy); err != nil {
			s.Fatal("Failed to enter G3 after shutdown: ", err)
		}
		pwrONDUT()
		chromeLogin()
		if err := validateCbmemPrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed Previous Sleep state is not 5: ", err)
		}
	}
	if testOpt.shutdownmode == "poweroff" {
		if err := dut.Conn().CommandContext(ctx, "poweroff").Run(); err != nil {
			s.Fatal("Failed to execute power off cmd: ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to shutdown DUT: ", err)
		}
		if err := validateG3PowerSate(ctx, pxy); err != nil {
			s.Fatal("Failed to enter G3 after shutdown: ", err)
		}
		pwrONDUT()
		if err := validateCbmemPrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed Previous Sleep state is not 5: ", err)
		}
		if err := dut.Conn().CommandContext(ctx, "halt").Run(); err != nil {
			s.Fatal("Failed to execute power off cmd: ", err)
		}
		if err := dut.WaitUnreachable(ctx); err != nil {
			s.Fatal("Failed to shutdown DUT: ")
		}
		if err := validateG3PowerSate(ctx, pxy); err != nil {
			s.Fatal("Failed to enter G3 after shutdown: ", err)
		}
		pwrONDUT()
		if err := validateCbmemPrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed Previous Sleep state is not 5: : ", err)
		}
	}
}

// validateCbmemPrevSleepState sleep state from cbmem command output.
func validateCbmemPrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
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

// validateG3PowerSate verify power state G3 after shutdwon.
func validateG3PowerSate(ctx context.Context, pxy *servo.Proxy) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get ec power state")
		}
		if pwrState != "G3" {
			return errors.New("DUT not in G3 state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}
