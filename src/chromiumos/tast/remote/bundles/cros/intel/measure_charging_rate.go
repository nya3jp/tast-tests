// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	requiredBatteryPercent = 70
	maxBatteryPercent      = 100
	chargeCheckInterval    = time.Minute
	chargeCheckTimeout     = time.Hour
	batteryLevelTimeout    = 20 * time.Second // default servo comm timeout is 10s, battery check requires two.
	batteryLevelInterval   = time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MeasureChargingRate,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measuring charging rate in suspend mode (S0ix)",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.power.BatteryService"},
		SoftwareDeps: []string{"chrome", "crossystem"},
		HardwareDeps: hwdep.D(hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      120 * time.Minute,
	})
}

func MeasureChargingRate(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	cl, err := rpc.Dial(ctx, h.DUT, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(cleanupCtx)

	client := power.NewBatteryServiceClient(cl.Conn)
	if _, err := client.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.Close(cleanupCtx, &empty.Empty{})

	defer func(ctx context.Context) {
		s.Log("Plugging power supply")
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Error("Failed to connect charger: ", err)
		}
	}(cleanupCtx)

	// Checking initial battery charge.
	initialCharge, err := getChargePercentage(ctx, h)
	if err != nil {
		s.Fatal("Failed to get battery level: ", err)
	}
	// Putting battery within testable range.
	if initialCharge >= requiredBatteryPercent {
		s.Logf("Current charge is %v; Required charge is %v; Stopping power supply & draining battery", initialCharge, requiredBatteryPercent)
		if err := h.SetDUTPower(ctx, false); err != nil {
			s.Fatal("Failed to remove charger: ", err)
		}
		request := power.BatteryRequest{MaxPercentage: requiredBatteryPercent}
		if _, err := client.DrainBattery(ctx, &request); err != nil {
			s.Fatal("Failed to drain battery: ", err)
		}
	}

	if err := h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to connect charger: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return err
		} else if !attached {
			return errors.New("charger is not attached - use Servo V4 Type-C or supply RPM vars")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Failed to check if charger is connected via Servo V4: ", err)
	}

	s.Log("Performing cold reboot")
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
		s.Fatal("Failed to perform mode aware reboot: ", err)
	}

	newCl, err := rpc.Dial(ctx, h.DUT, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer newCl.Close(cleanupCtx)

	newClient := power.NewBatteryServiceClient(newCl.Conn)
	if _, err := newClient.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer newClient.Close(cleanupCtx, &empty.Empty{})

	slpOpSetPre, pkgOpSetPre, err := powercontrol.SlpAndC10PackageValues(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values before suspend-resume: ", err)
	}

	// Emulate DUT lid closing.
	if err := h.Servo.CloseLid(ctx); err != nil {
		s.Fatal("Failed to close DUT's lid: ", err)
	}

	testing.Poll(ctx, func(ctx context.Context) error {
		s.Log("Checking lid state after closing lid")
		lidState, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to check the final lid state")
		}
		if lidState != string(servo.LidOpenNo) {
			return errors.Errorf("failed to check DUT lid state, expected: %q got: %q", servo.LidOpenNo, lidState)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	chargeBeforeSleep, err := getChargePercentage(ctx, h)
	if err != nil {
		s.Fatal("Failed to get battery level: ", err)
	}

	// For 10 minutes, observe battery charging status.
	s.Log("Charging DUT for 10 minutes after closing lid")
	if err := testing.Sleep(ctx, 10*time.Minute); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Checking battery charge during 10 minutes.
	chargeAfterSleep, err := getChargePercentage(ctx, h)
	if err != nil {
		s.Fatal("Failed to get battery level: ", err)
	}

	totalTime := getChargingTime((chargeAfterSleep - chargeBeforeSleep), 10)
	s.Log("Total Time to Full Charge: ", totalTime)
	if totalTime > 180 {
		s.Fatal("Failed: Total battery charging time is more than 3 hours")
	}

	s.Logf("Waiting for battery to reach %d%%", maxBatteryPercent)
	if err := waitForCharge(ctx, h, maxBatteryPercent); err != nil {
		s.Fatalf("Failed to reach target %d%%, %s", maxBatteryPercent, err.Error())
	}

	// Emulate DUT lid opening.
	if err := h.Servo.OpenLid(ctx); err != nil {
		s.Fatal("Failed to open DUT's lid: ", err)
	}
	testing.Poll(ctx, func(ctx context.Context) error {
		s.Log("Checking lid state after opening lid")
		lidState, err := h.Servo.LidOpenState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to check the final lid state")
		}
		if lidState != string(servo.LidOpenYes) {
			return errors.Errorf("failed to check DUT lid state, expected: %q got: %q", servo.LidOpenYes, lidState)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	slpOpSetPost, pkgOpSetPost, err := powercontrol.SlpAndC10PackageValues(ctx, h.DUT)
	if err != nil {
		s.Fatal("Failed to get SLP counter and C10 package values after suspend-resume: ", err)
	}

	if err := powercontrol.AssertSLPAndC10(slpOpSetPre, slpOpSetPost, pkgOpSetPre, pkgOpSetPost); err != nil {
		s.Fatal("Failed to verify SLP and C10 state values: ", err)
	}

}

// getChargingTime returns the total time taken to charge battery completely.
func getChargingTime(change, time int) int {
	return (time / change) * 100
}

// waitForCharge charges the DUT to required charge percentage.
func waitForCharge(ctx context.Context, h *firmware.Helper, target int) error {
	// Make sure AC power is connected.
	// The original setting will be restored automatically when the test ends.
	if err := h.SetDUTPower(ctx, true); err != nil {
		return errors.Wrap(err, "failed to set DUT power")
	}

	err := testing.Poll(ctx, func(ctx context.Context) error {
		pct, err := getChargePercentage(ctx, h)
		if err != nil {
			// Failed to get battery level so stop trying.
			return testing.PollBreak(err)
		}

		if pct < target {
			return errors.Errorf("Current battery charge is %d%%, required %d%%", pct, target)
		}

		return nil
	}, &testing.PollOptions{Timeout: chargeCheckTimeout, Interval: chargeCheckInterval})

	if err != nil {
		return errors.Wrap(err, "failed to get charge percentage")
	}

	// Disable Servo power to DUT.
	if err := h.SetDUTPower(ctx, false); err != nil {
		return errors.Wrap(err, "failed to set DUT power")
	}

	return nil
}

// getChargePercentage returns the current charge of the battery.
func getChargePercentage(ctx context.Context, h *firmware.Helper) (int, error) {
	// Attempt to determine the battery percentage.
	// Each servo communication attempt is retried to account for any transient
	// communication problems.
	var err error = nil
	currentMAH := 0
	maxMAH := 0

	testing.Poll(ctx, func(ctx context.Context) error {
		currentMAH, err = h.Servo.GetBatteryChargeMAH(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get battery charge MAH")
		}

		maxMAH, err = h.Servo.GetBatteryFullChargeMAH(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get battery full charge MAH")
		}

		return nil

	}, &testing.PollOptions{Timeout: batteryLevelTimeout, Interval: batteryLevelInterval})

	if err != nil {
		return -1, errors.Wrap(err, "failed to get battery charge details")
	}
	return int(100 * float32(currentMAH) / float32(maxMAH)), nil
}
