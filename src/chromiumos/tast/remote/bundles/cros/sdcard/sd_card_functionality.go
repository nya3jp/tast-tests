// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sdcard

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/policy/dututils"
	"chromiumos/tast/remote/bundles/cros/sdcard/powercontrol"
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
		Vars:         []string{"servo", "sdcard.functionality_iterations"},
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

	iterCount := 1 // Default to one iteration.
	if iter, ok := s.Var("sdcard.functionality_iterations"); ok {
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
	if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
		s.Fatal("Failed to login to chrome: ", err)
	}

	for i := 1; i <= iterCount; i++ {
		s.Logf("Iteration: %d/%d", i, iterCount)
		if err := sdCardDetection(ctx, dut); err != nil {
			s.Fatal("Failed to detect SD card: ", err)
		}

		if testOpt.powerMode == "shutdown" {
			powerState := "S5"
			if err := powercontrol.ShutdownAndWaitForPowerState(ctx, pxy, dut, powerState); err != nil {
				s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", powerState, err)
			}

			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
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
		if err := powercontrol.ChromeOSLogin(ctx, dut, s.RPCHint()); err != nil {
			s.Fatalf("Failed to login to chrome after %q: %v", testOpt.powerMode, err)
		}

		if err := powercontrol.ValidatePrevSleepState(ctx, dut, testOpt.cbmemSleepState); err != nil {
			s.Fatal("Failed to validate previous sleep state: ", err)
		}
	}
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
