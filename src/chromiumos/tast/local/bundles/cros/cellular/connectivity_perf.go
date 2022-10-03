// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConnectivityPerf,
		Desc:         "Measure the time it takes to enable, disable, connect, and disconnect from a Cellular Service ",
		Contacts:     []string{"madhavadas@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular_crosbolt", "cellular_crosbolt_perf_nightly"},
		HardwareDeps: hwdep.D(hwdep.Cellular()),
		Timeout:      10 * time.Minute,
		Fixture:      "cellular",
	})
}

func ConnectivityPerf(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	verifyCellularConnectivity := func(ctx context.Context) error {
		perfValues := perf.NewValues()

		// Test Disable / Enable / Connect / Disconnect.
		// Run the test a second time to test Disable after Connect/Disconnect.
		// Run the test a third time to help test against flakiness.
		for i := 0; i < 3; i++ {
			testing.ContextLogf(ctx, "Disable %d", i)
			disableTime, err := helper.Disable(ctx)
			if err != nil {
				return errors.Wrapf(err, "disable failed on attempt %d", i)
			}
			perfValues.Append(perf.Metric{
				Name:      "cellular_disable_time",
				Unit:      "seconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, disableTime.Seconds())
			testing.ContextLogf(ctx, "Enable %d", i)
			enableTime, err := helper.Enable(ctx)
			if err != nil {
				return errors.Wrapf(err, "enable failed on attempt %d", i)
			}
			perfValues.Append(perf.Metric{
				Name:      "cellular_enable_time",
				Unit:      "seconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, enableTime.Seconds())
			testing.ContextLogf(ctx, "Connect %d", i)
			connectTime, err := helper.ConnectToDefault(ctx)
			if err != nil {
				return errors.Wrapf(err, "connect failed on attempt %d", i)
			}
			perfValues.Append(perf.Metric{
				Name:      "cellular_connect_time",
				Unit:      "seconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, connectTime.Seconds())
			testing.ContextLogf(ctx, "Disconnect %d", i)
			disconnectTime, err := helper.Disconnect(ctx)
			if err != nil {
				return errors.Wrapf(err, "disconnect failed on attempt %d", i)
			}
			perfValues.Append(perf.Metric{
				Name:      "cellular_disconnect_time",
				Unit:      "seconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, disconnectTime.Seconds())
		}

		if err := perfValues.Save(s.OutDir()); err != nil {
			return errors.Wrap(err, "failed saving perf data")
		}
		return nil
	}

	if err := helper.RunTestOnCellularInterface(ctx, verifyCellularConnectivity); err != nil {
		s.Fatal("Failed to run test on cellular interface: ", err)
	}

}
