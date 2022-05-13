// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OnboardKeyboardFunctionalityCheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies on-board keyboard functionality check with suspend-resume and coldboot operation",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.AudioService"},
		VarDeps:      []string{"servo"},
		Params: []testing.Param{{
			Name:              "suspend_resume",
			Val:               "suspendStressTest",
			ExtraHardwareDeps: hwdep.D(hwdep.X86()),
			Timeout:           10 * time.Minute,
		}, {
			Name:              "coldboot",
			Val:               "shutdownStressTest",
			ExtraHardwareDeps: hwdep.D(hwdep.ChromeEC()),
			Timeout:           15 * time.Minute,
		},
		}})
}

func OnboardKeyboardFunctionalityCheck(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	dut := s.DUT()
	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	// Performs a Chrome login.
	loginChrome := func() (*rpc.Client, error) {
		cl, err := rpc.Dial(ctx, dut, s.RPCHint())
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
		}

		audioService := ui.NewAudioServiceClient(cl.Conn)
		if _, err := audioService.New(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to login Chrome: ", err)
		}
		return cl, nil
	}

	// Perform initial Chrome login.
	cl, err := loginChrome()
	if err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Error("Failed to power on DUT at cleanup: ", err)
			}
			wtCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := dut.WaitConnect(wtCtx); err != nil {
				s.Error("Failed to wait connect to DUT at cleanup: ", err)
			}
		}
		if err := dut.Conn().CommandContext(ctx, "rm", "-rf", "/tmp/kbEventlog.txt").Run(); err != nil {
			s.Error("Failed to remove temporary file: ", err)
		}
	}(ctxForCleanUp)

	if err := performKeyboardEvents(ctx, dut, cl); err != nil {
		s.Fatal("Failed to perform on-board keyboard evtest events: ", err)
	}

	testMode := s.Param().(string)
	if testMode == "suspendStressTest" {
		slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
		if err != nil {
			s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
		}

		suspendStressTestCounter := 10
		if err := powercontrol.PerformSuspendStressTest(ctx, dut, suspendStressTestCounter); err != nil {
			s.Fatal("Failed to perform suspend stress test: ", err)
		}

		if err := performKeyboardEvents(ctx, dut, cl); err != nil {
			s.Fatal("Failed to perform on-board keyboard evtest events after suspend stress test: ", err)
		}

		slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, dut)
		if err != nil {
			s.Fatal("Failed to get SLP counter and C10 package values after suspend-resume: ", err)
		}

		if slpOpSetPre == slpOpSetPost {
			s.Fatalf("Failed: SLP counter value %q should be different from the one before suspend %q", slpOpSetPost, slpOpSetPre)
		}

		if slpOpSetPost == 0 {
			s.Fatal("Failed SLP counter value must be non-zero, got: ", slpOpSetPost)
		}

		if pkgOpSetPre == pkgOpSetPost {
			s.Fatalf("Failed: Package C10 value %q must be different from the one before suspend %q", pkgOpSetPost, pkgOpSetPre)
		}

		if pkgOpSetPost == "0x0" || pkgOpSetPost == "0" {
			s.Fatal("Failed: Package C10 should be non-zero")
		}
	}

	if testMode == "shutdownStressTest" {
		iter := 10
		for i := 1; i <= iter; i++ {
			s.Logf("Iteration: %d/%d", i, iter)
			if err := performColdboot(ctx, dut, pxy); err != nil {
				s.Fatal("Failed to perform coldboot: ", err)
			}

			// Perform a Chrome login after power on from shutdown state.
			cl, err = loginChrome()
			if err != nil {
				s.Fatal("Failed to login Chrome after shutdown: ", err)
			}

			// After powering on from shutdown, perform Chrome login
			// and check for on-board keyboard functional check.
			if err := performKeyboardEvents(ctx, dut, cl); err != nil {
				s.Fatal("Failed to perform on-board keyboard evtest events after shutdown: ", err)
			}

			// Verfifies prev_sleep_state is 5 for coldboot.
			cbmemSleepState := 5
			if err := powercontrol.ValidatePrevSleepState(ctx, dut, cbmemSleepState); err != nil {
				s.Fatal("Failed to validate previous sleep state: ", err)
			}
		}
	}
}

// performColdboot peforms shutdown, verifies S5 state and powers on DUT.
func performColdboot(ctx context.Context, dut *dut.DUT, pxy *servo.Proxy) error {
	powerState := "S5"
	if err := powercontrol.ShutdownAndWaitForPowerState(ctx, pxy, dut, powerState); err != nil {
		return errors.Wrapf(err, "failed to shutdown and wait for %q powerstate", powerState)
	}

	if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
		return errors.Wrap(err, "failed to power on DUT")
	}
	return nil
}

// keyboardEventNumber returns USB Keyboard evtest event number.
func keyboardEventNumber(ctx context.Context, dut *dut.DUT) (int, error) {
	out, _ := dut.Conn().CommandContext(ctx, "evtest").CombinedOutput()
	re := regexp.MustCompile(`(?i)/dev/input/event([0-9]+):.*AT Translated Set . keyboard.*`)
	result := re.FindStringSubmatch(string(out))
	keyboardEventNum := ""
	if len(result) > 0 {
		keyboardEventNum = result[1]
	} else {
		return 0, errors.New("failed to find keyboard in evtest command output")
	}
	eventNum, err := strconv.Atoi(keyboardEventNum)
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert string to integer")
	}
	return eventNum, nil
}

// performKeyboardEvents performs USB Keyboard key press events.
func performKeyboardEvents(ctx context.Context, dut *dut.DUT, cl *rpc.Client) error {
	eventLogFile := "/tmp/kbEventlog.txt"
	evtestRecordCmd := "evtest /dev/input/event"

	// Remove temporary log file, if any present before creating it.
	if err := dut.Conn().CommandContext(ctx, "rm", "-rf", eventLogFile).Run(); err != nil {
		return errors.Wrap(err, "failed to remove temporary file")
	}

	eventNum, err := keyboardEventNumber(ctx, dut)
	if err != nil {
		return errors.Wrap(err, "failed to get on-board keyboard evtest event number")
	}

	// Perform evtest command to record all events and save in temporary file.
	go func() {
		err = dut.Conn().CommandContext(ctx, "bash", "-c", fmt.Sprintf("%s%d > %s &", evtestRecordCmd, eventNum, eventLogFile)).Run()
	}()
	if err != nil {
		return errors.Wrap(err, "failed to perform evtest events record")
	}

	// Perform USB Key press.
	audioService := ui.NewAudioServiceClient(cl.Conn)
	pressKeys := []string{"c", "h", "r", "o", "m", "e"}
	for _, key := range pressKeys {
		accelKeys := &ui.AudioServiceRequest{Expr: key}
		if _, err := audioService.KeyboardAccel(ctx, accelKeys); err != nil {
			return errors.Wrapf(err, "failed to press on-board keyboard %q key", key)
		}
	}

	// Stopping evtest record process.
	if err := dut.Conn().CommandContext(ctx, "sudo", "pkill", "evtest").Run(); err != nil {
		return errors.Wrap(err, "failed to kill evtest process")
	}

	catOutput, err := dut.Conn().CommandContext(ctx, "cat", eventLogFile).Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute cat command")
	}

	// Validating On-board Keyboard key press is recorded in log file output.
	keysPattern := []string{"KEY_C", "KEY_H", "KEY_R", "KEY_O", "KEY_M", "KEY_E"}
	for _, key := range keysPattern {
		keyRe := regexp.MustCompile(fmt.Sprintf(`\(%s\).*value 0`, key))
		match := keyRe.FindAllString(string(catOutput), -1)
		if len(match) == 0 {
			return errors.Errorf("failed to press On-board Keyboard %q key", key)
		}
	}
	return nil
}
