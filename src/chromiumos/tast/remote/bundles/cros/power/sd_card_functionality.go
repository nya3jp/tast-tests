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
	"chromiumos/tast/remote/bundles/cros/policy/dututils"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type sdCardTestParams struct {
	powerMode       string
	cbmemSleepState int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SDCardFunctionality,
		Desc:         "Verifies micro SD card functionality before and after performing, shutdown or reboot",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Vars:         []string{"servo", "power.iterations"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Timeout:      15 * time.Minute,
		Params: []testing.Param{{
			Name: "shutdown",
			Val: sdCardTestParams{
				powerMode:       "shutdown",
				cbmemSleepState: 5, // cbmemSleepState must be 5 for coldboot validation.
			},
		}, {
			Name: "reboot",
			Val: sdCardTestParams{
				powerMode:       "reboot",
				cbmemSleepState: 0, // cbmemSleepState must be 0 for warmboot validation.
			},
		}},
	})
}

// SDCardFunctionality checks SD card functionality while performing shutdown and reboot.
// Pre-requisite: SD card must be inserted into DUT SD card slot.
func SDCardFunctionality(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec, ok := s.Var("servo")
	if !ok {
		servoSpec = ""
	}

	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	iterCount := 1 // Use default iteration value.
	if iter, ok := s.Var("power.iterations"); ok {
		if iterCount, err = strconv.Atoi(iter); err != nil {
			s.Fatalf("Failed to parse iteration value %q: %v", iter, err)
		}
	}

	testOpt := s.Param().(sdCardTestParams)

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if err := dututils.EnsureDUTIsOn(ctx, dut, pxy.Servo()); err != nil {
			s.Error("Failed to ensure DUT is powered on: ", err)
		}
	}(cleanupCtx)

	// Performing chrome intital login.
	if err := chromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
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

			if err := waitS5PowerState(ctx, pxy); err != nil {
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

		// Performing chrome login after powering on DUT from coldboot/warmboot.
		if err := chromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
			s.Fatalf("Failed to login to chrome after %q: %v", testOpt.powerMode, err)
		}

		if err := validatePrevSleepState(ctx, dut, testOpt.cbmemSleepState); err != nil {
			s.Fatal("Failed to validate previous sleep state: ", err)
		}
	}
}

// chromeOSLogin performs login to DUT.
func chromeOSLogin(ctx context.Context, d *dut.DUT, rpcHint *testing.RPCHint) error {
	cl, err := rpc.Dial(ctx, d, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	return nil
}

// validatePrevSleepState sleep state from cbmem command output.
func validatePrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	// Command to check previous sleep state.
	const cmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	out, err := dut.Conn().CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		return errors.Wrapf(err, "failed to execute %q command", cmd)
	}

	got := strings.TrimSpace(string(out))
	want := fmt.Sprintf("prev_sleep_state %d", sleepStateValue)

	if got != want {
		return errors.Errorf("unexpected sleep state = got %q, want %q", got, want)
	}
	return nil
}

// waitS5PowerState verify power state S5 after shutdown.
func waitS5PowerState(ctx context.Context, pxy *servo.Proxy) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		pwrState, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get EC power state")
		}
		if pwrState != "S5" {
			return errors.Errorf("unexpected DUT EC power state = got %q, want 'S5'", pwrState)
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
	const (
		dmesgMmcCommand = "dmesg | grep mmc"
		sdMmcSpecFile   = "/sys/kernel/debug/mmc0/ios"
	)

	sdCardRe := regexp.MustCompile(`mmcblk0: mmc0:[\d+\w+\s+]*`)
	sdCardSpecRe := regexp.MustCompile(`timing spec:.[1-9]+.\(sd.*`)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		mmcDmesgOut, err := dut.Conn().CommandContext(ctx, "sh", "-c", dmesgMmcCommand).Output()
		if err != nil {
			return errors.Wrapf(err, "failed to execute %q command", dmesgMmcCommand)
		}

		if got := string(mmcDmesgOut); !sdCardRe.MatchString(got) {
			return errors.Errorf("failed to get MMC info in dmesg = got %q, want match %q", got, sdCardRe)
		}

		sdCardSpecOut, err := dut.Conn().CommandContext(ctx, "cat", sdMmcSpecFile).Output()
		if err != nil {
			return errors.Wrapf(err, "failed to execute 'cat %s' command", sdMmcSpecFile)
		}

		if got := string(sdCardSpecOut); !sdCardSpecRe.MatchString(got) {
			return errors.Errorf("failed to get MMC info in %q file = got %q, want match %q", sdMmcSpecFile, got, sdCardSpecRe)
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 15 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "unable to find micro SD card")
	}
	return nil
}
