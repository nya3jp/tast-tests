// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HostCellularSpeedtest,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Runs Speedtest on cellular interface",
		Contacts:     []string{"madhavadas@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular_crosbolt", "cellular_crosbolt_perf_nightly"},
		HardwareDeps: hwdep.D(hwdep.Cellular()),
		Timeout:      4 * time.Minute,
	})
}

func HostCellularSpeedtest(ctx context.Context, s *testing.State) {
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	if _, err := helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable modem")
	}

	verifyHostIPSpeedTest := func(ctx context.Context) error {
		uploadSpeed, downloadSpeed, err := cellular.RunHostIPSpeedTest(ctx, testexec.CommandContext, "/usr/local/bin")
		if err != nil {
			return errors.Wrap(err, "failed speed test")
		}
		if uploadSpeed == 0.0 || downloadSpeed == 0.0 {
			return errors.Errorf("invalid speed upload: %f download: %f", uploadSpeed, downloadSpeed)
		}
		perfValues := perf.NewValues()
		perfValues.Set(perf.Metric{
			Name:      "Upload",
			Unit:      "bps",
			Direction: perf.BiggerIsBetter,
		}, uploadSpeed)
		perfValues.Set(perf.Metric{
			Name:      "Download",
			Unit:      "bps",
			Direction: perf.BiggerIsBetter,
		}, downloadSpeed)
		if err := perfValues.Save(s.OutDir()); err != nil {
			return errors.Wrap(err, "failed saving perf data")
		}
		return nil
	}

	if err := helper.RunTestOnCellularInterface(ctx, verifyHostIPSpeedTest); err != nil {
		s.Fatal("Failed to run test on cellular interface: ", err)
	}
}
