// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BatteryStatusOnACRemoval,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Battery status and stop charging upon removal of AC",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.power.BatteryService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Battery()),
		Fixture:      fixture.NormalMode,
		Timeout:      time.Hour,
	})
}

func BatteryStatusOnACRemoval(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	chargerPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 250 * time.Millisecond,
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

	iterations := 5
	for i := 1; i <= iterations; i++ {
		s.Logf("Iteration: %d/%d", i, iterations)
		//Checking initial battery charge.
		initialCharge, err := getChargePercentage(ctx, h)
		if err != nil {
			s.Fatal("Failed to get battery level: ", err)
		}
		// Putting battery within testable range.
		if initialCharge >= 95 {
			s.Log("Stopping power supply")
			if err := h.SetDUTPower(ctx, false); err != nil {
				s.Fatal("Failed to remove charger: ", err)
			}
			request := power.BatteryRequest{MaxPercentage: 90}
			if _, err := client.DrainBattery(ctx, &request); err != nil {
				s.Fatal("Failed to drain battery: ", err)
			}
		}

		s.Log("Plugging power supply")
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
		}, &chargerPollOptions); err != nil {
			s.Fatal("Failed to check if charger is connected via Servo V4: ", err)
		}

		// Verifying battery charging with power_supply_info command.
		s.Log("Checking battery information for charging")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if charging, err := isBatteryCharging(ctx, h); err != nil {
				return errors.Wrap(err, "failed to verify battery information")
			} else if !charging {
				return errors.New("failed to verify expected charging status")
			}
			return nil
		}, &chargerPollOptions); err != nil {
			s.Fatal("Failed to check charging status from power_supply_info: ", err)
		}

		// Charging the DUT for 3%.
		targetCharge := initialCharge + 3
		s.Logf("Waiting for battery to reach %d%%", targetCharge)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			pct, err := getChargePercentage(ctx, h)
			if err != nil {
				// Failed to get battery level so stop trying.
				return testing.PollBreak(err)
			}
			if pct < targetCharge {
				return errors.Errorf("Current battery charge is %d%%, required %d%%", pct, targetCharge)
			}

			return nil
		}, &testing.PollOptions{Timeout: time.Hour, Interval: time.Minute}); err != nil {
			s.Fatal("Failed to charge DUT: ", err)
		}

		s.Log("Stopping power supply")
		if err := h.SetDUTPower(ctx, false); err != nil {
			s.Fatal("Failed to remove charger: ", err)
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
				return err
			} else if attached {
				return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
			}
			return nil
		}, &chargerPollOptions); err != nil {
			s.Fatal("Failed to check if charger is disconnected via Servo V4: ", err)
		}

		// Discharging DUT for 3%.
		s.Logf("Discharging DUT till %d%%", initialCharge)
		request := power.BatteryRequest{MaxPercentage: float32(initialCharge)}
		if _, err := client.DrainBattery(ctx, &request); err != nil {
			s.Fatal("Failed to drain battery: ", err)
		}

		// Verifying battery charging with power_supply_info command.
		s.Log("Checking battery information for charging")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if charging, err := isBatteryCharging(ctx, h); err != nil {
				return errors.Wrap(err, "failed to verify battery information")
			} else if charging {
				return errors.New("failed to verify expected charging status")
			}
			return nil
		}, &chargerPollOptions); err != nil {
			s.Fatal("Failed to check charging status from power_supply_info: err")
		}
	}
}

// isBatteryCharging returns true if battery is charging.
func isBatteryCharging(ctx context.Context, h *firmware.Helper) (bool, error) {
	regex := `state:(\s+\w+\s?\w+)`
	expMatch := regexp.MustCompile(regex)

	out, err := h.DUT.Conn().CommandContext(ctx, "power_supply_info").Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to retrieve power supply info from DUT")
	}

	matches := expMatch.FindStringSubmatch(string(out))
	if len(matches) < 2 {
		return false, errors.Errorf("failed to match regex %q in %q", expMatch, string(out))
	}

	return strings.TrimSpace(matches[1]) != "Discharging", nil
}

// getChargePercentage returns battery charge percentage.
func getChargePercentage(ctx context.Context, h *firmware.Helper) (int, error) {
	var err error = nil
	currentMAH := 0
	maxMAH := 0
	testing.Poll(ctx, func(ctx context.Context) error {
		currentMAH, err = h.Servo.GetBatteryChargeMAH(ctx)
		if err != nil {
			return err
		}
		maxMAH, err = h.Servo.GetBatteryFullChargeMAH(ctx)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: time.Second})
	if err != nil {
		return -1, err
	}

	return int(100 * float32(currentMAH) / float32(maxMAH)), nil
}
