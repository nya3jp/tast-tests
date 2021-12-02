// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type sdCardTestParams struct {
	powerMode            string
	cbmemSleepStateValue int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SDCardFunctionality,
		Desc:         "Verifies micro SD card functionality before and after performing, shutdown or reboot", // Pre-requisite: SD card must be inserted into DUT SD card slot.
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Vars:         []string{"servo", "power.iterations"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{{
			Name: "shutdown",
			Val: sdCardTestParams{
				powerMode:            "shutdown",
				cbmemSleepStateValue: 5,
			},
			Timeout: 15 * time.Minute,
		}, {
			Name: "reboot",
			Val: sdCardTestParams{
				powerMode:            "reboot",
				cbmemSleepStateValue: 0,
			},
			Timeout: 15 * time.Minute,
		}},
	})
}

func SDCardFunctionality(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	iterCount := 2 // Use default iteration value.
	if iter, ok := s.Var("power.iterations"); ok {
		if iterCount, err = strconv.Atoi(iter); err != nil {
			s.Fatalf("Failed to parse iteration value %q: %v", iter, err)
		}
	}

	testOpt := s.Param().(sdCardTestParams)

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to wake up DUT at cleanup: ", err)
			}
		}
	}(ctxForCleanUp)

	if _, err := chromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	for i := 1; i <= iterCount; i++ {
		s.Logf("Iteration: %d/%d", i, iterCount)
		if err := sdCardDetection(ctx, dut); err != nil {
			s.Fatal("Failed to detect SD card: ", err)
		}

		if testOpt.powerMode == "shutdown" {
			powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if err := dut.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
				s.Fatal("Failed to execute shutdown command: ", err)
			}
			sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := dut.WaitUnreachable(sdCtx); err != nil {
				s.Fatal("Failed to wait for unreachable: ", err)
			}

			if err := validateS5PowerState(ctx, pxy); err != nil {
				s.Fatal("Failed to enter S5 after shutdown: ", err)
			}

			if err := powerOntoDUT(ctx, pxy, dut); err != nil {
				s.Fatal("Failed to wake up DUT: ", err)
			}
		}

		if testOpt.powerMode == "reboot" {
			if err := dut.Reboot(ctx); err != nil {
				s.Fatal("Failed to reboot DUT: ", err)
			}
			waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
			defer cancel()
			if err := dut.WaitConnect(waitCtx); err != nil {
				s.Fatal("Failed to wait connect DUT after reboot: ", err)
			}
		}

		if _, err := chromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
			s.Fatalf("Failed to login to chrome after %q: %v", testOpt.powerMode, err)
		}

		if err := validatePrevSleepState(ctx, dut, testOpt.cbmemSleepStateValue); err != nil {
			s.Fatalf("Failed Previous Sleep state is not %v: %v", testOpt.cbmemSleepStateValue, err)
		}
	}
}

// chromeOSLogin performs login to DUT.
func chromeOSLogin(ctx context.Context, d *dut.DUT, rpcHint *testing.RPCHint) (*empty.Empty, error) {
	cl, err := rpc.Dial(ctx, d, rpcHint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	return client.CloseChrome(ctx, &empty.Empty{})
}

// validatePrevSleepState sleep state from cbmem command output.
func validatePrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	const (
		// Command to check previous sleep state.
		prevSleepStateCmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	)
	out, err := dut.Conn().CommandContext(ctx, "sh", "-c", prevSleepStateCmd).Output()
	if err != nil {
		return errors.Wrapf(err, "failed to execute %q command", prevSleepStateCmd)
	}

	actualOut := strings.TrimSpace(string(out))
	expectedOut := fmt.Sprintf("prev_sleep_state %d", sleepStateValue)

	if !strings.Contains(actualOut, expectedOut) {
		return errors.Errorf("expected %q, but got %q", expectedOut, actualOut)
	}
	return nil
}

// validateS5PowerState verify power state S5 after shutdown.
func validateS5PowerState(ctx context.Context, pxy *servo.Proxy) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get ec power state")
		}
		if pwrState != "S5" {
			return errors.New("DUT not in S5 state")
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// powerOntoDUT performs power normal press to wake DUT.
func powerOntoDUT(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power button press")
	}
	if err := dut.WaitConnect(waitCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect DUT")
	}
	return nil
}

// sdCardDetection performs SD card detection validation.
func sdCardDetection(ctx context.Context, dut *dut.DUT) error {
	dmesgMmcCommand := "dmesg | grep mmc"
	sdMmcSpecFile := "/sys/kernel/debug/mmc0/ios"
	sdCardRe := regexp.MustCompile(`mmcblk0: mmc0:[\d+\w+\s+]*`)
	sdCardSpecRe := regexp.MustCompile(`timing spec:.[1-9]+.\(sd.*`)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		mmcDmesgOut, err := dut.Conn().CommandContext(ctx, "sh", "-c", dmesgMmcCommand).Output()
		if err != nil {
			return errors.Wrapf(err, "failed to execute %q command", dmesgMmcCommand)
		}

		if !sdCardRe.MatchString(string(mmcDmesgOut)) {
			return errors.Errorf("failed to get mmc info in dmesg, expected %q but got %q", sdCardRe, string(mmcDmesgOut))
		}

		sdCardSpecOut, err := dut.Conn().CommandContext(ctx, "cat", sdMmcSpecFile).Output()
		if err != nil {
			return errors.Wrapf(err, "failed to execute 'cat %s' command", sdMmcSpecFile)
		}

		if !sdCardSpecRe.MatchString(string(sdCardSpecOut)) {
			return errors.Errorf("failed to get mmc info in %q file, expected %q but got %q", sdMmcSpecFile, sdCardSpecRe, string(sdCardSpecOut))
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "please insert micro SD card to DUT")
	}
	return nil
}
