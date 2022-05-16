// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"bufio"
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TouchColdboot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Touchscreen: Cold boot (S0-S5) with operation for 10 cycles",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.inputs.TouchscreenService"},
		HardwareDeps: hwdep.D(hwdep.TouchScreen(), hwdep.X86()),
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
	})
}

func TouchColdboot(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	performEVTest := func() {
		// Connect to the gRPC service on the DUT.
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}

		// Declare a rpc service for detecting touchscreen.
		touchscreen := inputs.NewTouchscreenServiceClient(cl.Conn)

		// Start a logged-in Chrome session, which is required prior to TouchscreenTap
		if _, err := touchscreen.NewChrome(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to start a new Chrome for the touchscreen service: ", err)
		}
		defer touchscreen.CloseChrome(ctx, &empty.Empty{})

		devPath, err := touchscreen.FindPhysicalTouchscreen(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to get touchscreen device path: ", err)
		}

		scannTouchscreen, err := deviceScanner(ctx, h, devPath.Path)
		if err != nil {
			s.Fatal("Failed to get touchscreen scanner: ", err)
		}

		// Emulate the action of tapping on a touch screen.
		if _, err := touchscreen.TouchscreenTap(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to perform a tap on the touch screen: ", err)
		}

		if err := evtestMonitor(scannTouchscreen); err != nil {
			s.Fatal("Failed to monitor evtest for touchscreen: ", err)
		}
	}

	iterations := 10
	for i := 1; i <= iterations; i++ {
		s.Logf("Iteration: %d/%d", i, iterations)

		performEVTest()

		powerState := "S5"
		if err := shutdownAndWaitForPowerState(ctx, h, powerState); err != nil {
			s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", powerState, err)
		}

		// Delay for some time to ensure the dut shutdown
		if err := testing.Sleep(ctx, 10*time.Second); err != nil {
			s.Error("Failed to sleep: ", err)
		}

		if err := powerOntoDUT(ctx, h); err != nil {
			s.Fatal("Failed to wake up DUT: ", err)
		}

		performEVTest()

		// Perfoming prev_sleep_state check.
		if err := powercontrol.ValidatePrevSleepState(ctx, s.DUT(), 5); err != nil {
			s.Fatal("Failed to validate previous sleep state: ", err)
		}
	}

}

// deviceScanner returns the evtest scanner for the touch screen device
func deviceScanner(ctx context.Context, h *firmware.Helper, devPath string) (*bufio.Scanner, error) {
	// Declare a rpc service for detecting touchscreen.
	cmd := h.DUT.Conn().CommandContext(ctx, "evtest", devPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start scanner")
	}

	scanner := bufio.NewScanner(stdout)
	return scanner, nil
}

// evtestMonitor is used to check whether events sent to the devices are picked up by the evtest.
func evtestMonitor(scanner *bufio.Scanner) error {
	evtestRe := regexp.MustCompile(`Event.*time.*code\s(\d*)\s\(` + `BTN_TOUCH` + `\)`)
	const scanTimeout = 5 * time.Second
	text := make(chan string)
	go func() {
		for scanner.Scan() {
			text <- scanner.Text()
		}
		close(text)
	}()
	for {
		select {
		case <-time.After(scanTimeout):
			return errors.New("failed to detect events within expected time")
		case out := <-text:
			if match := evtestRe.FindStringSubmatch(out); match != nil {
				return nil
			}
		}
	}
}

// shutdownAndWaitForPowerState verifies powerState(S5 or G3) after shutdown.
func shutdownAndWaitForPowerState(ctx context.Context, h *firmware.Helper, powerState string) error {
	powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := h.DUT.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return errors.Wrap(err, "failed to execute shutdown command")
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := h.DUT.WaitUnreachable(waitCtx); err != nil {
		return errors.Wrap(err, "failed to wait for unreachable")
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		got, err := h.Servo.GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get EC power state")
		}
		if want := powerState; got != want {
			return errors.Errorf("unexpected DUT EC power state = got %q, want %q", got, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// powerOntoDUT performs power normal press to wake DUT.
func powerOntoDUT(ctx context.Context, h *firmware.Helper) error {
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power button press")
	}
	return h.WaitConnect(waitCtx)
}
