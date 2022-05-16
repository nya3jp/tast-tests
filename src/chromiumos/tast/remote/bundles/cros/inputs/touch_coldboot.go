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
		Timeout:      15 * time.Minute,
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
	dut := s.DUT()

	performEVTest := func() {
		// Connect to the gRPC service on the DUT.
		cl, err := rpc.Dial(ctx, dut, s.RPCHint())
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
		if err := powercontrol.ShutdownAndWaitForPowerState(ctx, h.ServoProxy, dut, powerState); err != nil {
			s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", powerState, err)
		}

		if err := powercontrol.PowerOntoDUT(ctx, h.ServoProxy, dut); err != nil {
			s.Fatal("Failed to wake up DUT: ", err)
		}

		performEVTest()

		// Performing prev_sleep_state check.
		if err := powercontrol.ValidatePrevSleepState(ctx, dut, 5); err != nil {
			s.Fatal("Failed to validate previous sleep state: ", err)
		}
	}
}

// deviceScanner returns the evtest scanner for the touch screen device.
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
