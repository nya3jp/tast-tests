// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type shutdownModeTestParams struct {
	shutdownmode string
}

const (
	powerButton string = "powerbutton"
	powerOff    string = "poweroff"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShutdownMode,
		Desc:         "Verifies that system comes back after power button press and poweroff",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
		Params: []testing.Param{{
			Name: "power_button",
			Val:  shutdownModeTestParams{shutdownmode: powerButton},
		}, {
			Name: "poweroff_command",
			Val:  shutdownModeTestParams{shutdownmode: powerOff},
		},
		},
	})
}

func ShutdownMode(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	testOpt := s.Param().(shutdownModeTestParams)
	const (
		// cmdTimeout is a short duration used for sending commands.
		cmdTimeout = 3 * time.Second
	)
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// Logging into chrome.
	chromeLogin := func() {
		s.Log("Login to Chrome")
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
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
	pwrOnDUT := func() {
		s.Log("Power Normal Pressing")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to power button press: ", err)
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Log("Failed to wake up DUT. Retrying")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power button press: ", err)
			}
			if err := dut.WaitConnect(waitCtx); err != nil {
				s.Fatal("Failed to wait connect DUT: ", err)
			}
		}
	}
	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			pwrOnDUT()
		}
	}(ctx)
	if testOpt.shutdownmode == "powerbutton" {
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur(h.Config.HoldPwrButtonPowerOff)); err != nil {
			s.Fatal("Failed to power button press: ", err)
		}
		if err := dut.WaitUnreachable(waitCtx); err != nil {
			s.Fatal("Failed to shutdown DUT: ", err)
		}
		if err := validateG3PowerSate(ctx, h); err != nil {
			s.Fatal("Failed to enter G3 after shutdown: ", err)
		}
		pwrOnDUT()
		chromeLogin()
		if err := validateCbmemPrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed Previous Sleep state is not 5: ", err)
		}
	}
	if testOpt.shutdownmode == "poweroff" {
		powerOffCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()
		if err := h.DUT.Conn().CommandContext(powerOffCtx, "poweroff").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to power off DUT: ", err)
		}
		if err := dut.WaitUnreachable(waitCtx); err != nil {
			s.Fatal("Failed to shutdown DUT: ", err)
		}
		if err := validateG3PowerSate(ctx, h); err != nil {
			s.Fatal("Failed to enter G3 after shutdown: ", err)
		}
		pwrOnDUT()
		if err := validateCbmemPrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed Previous Sleep state is not 5: ", err)
		}
		haltCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()
		if err := h.DUT.Conn().CommandContext(haltCtx, "halt").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			s.Fatal("Failed to halt DUT: ", err)
		}
		if err := dut.WaitUnreachable(waitCtx); err != nil {
			s.Fatal("Failed to shutdown DUT: ", err)
		}
		if err := validateG3PowerSate(ctx, h); err != nil {
			s.Fatal("Failed to enter G3 after shutdown: ", err)
		}
		pwrOnDUT()
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
	// Extract prevsleep state from example output "prev_sleep_state 5".
	if count, err := strconv.Atoi(strings.Split(strings.Replace(string(out), "\n", "", -1), " ")[1]); err != nil {
		return err
	} else if count != sleepStateValue {
		return errors.Errorf("previous sleep state must be %d", sleepStateValue)
	}
	return nil
}

// validateG3PowerSate verify power state G3 after shutdown.
func validateG3PowerSate(ctx context.Context, h *firmware.Helper) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := h.Servo.GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get ec power state")
		}
		if pwrState != "G3" {
			return errors.New("DUT not in G3 state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}
