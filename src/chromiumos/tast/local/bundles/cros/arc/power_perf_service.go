// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/perf/perfpb"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterPowerPerfServiceServer(srv, &PowerPerfService{s: s})
		},
	})
}

type PowerPerfService struct {
	s       *testing.ServiceState
	metrics []perf.TimelineDatasource
	cleanup []func(ctx context.Context)
}

func (c *PowerPerfService) PowerSetup(ctx context.Context, request *arcpb.PowerSetupRequest) (*emptypb.Empty, error) {
	if c.cleanup != nil {
		// We didn't clean up from the last time, do it now.
		for _, c := range c.cleanup {
			// Defer to make the cleanup order correct, and also so that if one
			// panics, we still try to clean up the rest.
			defer c(ctx)
		}
		c.cleanup = nil
		return nil, errors.New("call PowerCleanup before calling PowerSetup again")
	}

	// Slice of things to clean up if we don't succeed.
	var cleanup []func(ctx context.Context)
	defer func() {
		for _, c := range cleanup {
			defer c(ctx)
		}
	}()

	// Set up Chrome.
	opts := []chrome.Option{
		chrome.ARCEnabled(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
		chrome.ExtraArgs("--disable-features=FirmwareUpdaterApp"),
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	cleanup = append(cleanup, func(ctx context.Context) { cr.Close(ctx) })

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}

	// Set up Android.
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	cleanup = append(cleanup, func(ctx context.Context) { os.RemoveAll(td) })

	a, err := arc.New(ctx, td)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	cleanup = append(cleanup, func(ctx context.Context) { a.Close(ctx) })

	// Configure DUT for power test.
	sup, cleanupPower := setup.New("power perf")
	cleanup = append(cleanup, func(ctx context.Context) { cleanupPower(ctx) })

	dischargeMode := setup.NoBatteryDischarge
	if batteryPath, err := power.SysfsBatteryPath(ctx); err == nil {
		dischargeMode = setup.ForceBatteryDischarge
		// There is a battery, make sure it's charged before starting the test.
		testing.ContextLog(ctx, "Waiting for battery to charge")
		if err := power.WaitForCharge(ctx, batteryPath, 0.95, 30*time.Minute); err != nil {
			return nil, err
		}
		testing.ContextLog(ctx, "Battery now charged")
	} else if !errors.Is(power.ErrNoBattery, err) {
		// If it's ErrNoBattery, leave dischargeMode at NoBatteryDischarge.
		return nil, errors.Wrap(err, "failed to determine if there is a battery")
	}

	// Wait until CPU is cooled down and idle.
	_, err = cpu.WaitUntilCoolDown(ctx, cpu.IdleCoolDownConfig())
	if err != nil {
		return nil, errors.Wrap(err, "CPU failed to cool down")
	}
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return nil, errors.Wrap(err, "CPU failed to idle")
	}
	if err := arc.CheckNoDex2Oat(td); err != nil {
		return nil, errors.Wrap(err, "failed to verify dex2oat was not running")
	}

	sup.Add(setup.PowerTest(ctx, tconn,
		setup.PowerTestOptions{Wifi: setup.DisableWifiInterfaces, NightLight: setup.DisableNightLight},
		setup.NewBatteryDischargeFromMode(dischargeMode),
	))
	if err := sup.Check(ctx); err != nil {
		return nil, errors.Wrap(err, "power perf setup failed")
	}

	if err := testing.Sleep(ctx, 90*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to sleep to settle before measurement")
	}

	// Start collecting metrics.
	c.metrics = []perf.TimelineDatasource{
		power.NewCpuidleStateMetrics(),
		power.NewPackageCStatesMetrics(),
		power.NewRAPLPowerMetrics(),
	}
	for _, metric := range c.metrics {
		if err := metric.Setup(ctx, ""); err != nil {
			return nil, errors.Wrap(err, "failed to setup metric")
		}
	}
	for _, metric := range c.metrics {
		if err := metric.Start(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to start metric")
		}
	}

	// ctx is cancelled when this call returns, so if we want to clean up after
	// a timeout we need a new context.Context.

	// Don't cleanup everything
	c.cleanup = cleanup
	cleanup = nil

	return &emptypb.Empty{}, nil
}

func (c *PowerPerfService) PowerCleanup(ctx context.Context, _ *emptypb.Empty) (*perfpb.Values, error) {
	for _, c := range c.cleanup {
		defer c(ctx)
	}
	c.cleanup = nil

	p := perf.NewValues()
	if len(c.metrics) > 0 {
		for _, metric := range c.metrics {
			if err := metric.Snapshot(ctx, p); err != nil {
				return nil, errors.Wrap(err, "failed to snapshot metric")
			}
		}
		for _, metric := range c.metrics {
			if err := metric.Stop(ctx, p); err != nil {
				return nil, errors.Wrap(err, "failed to stop metric")
			}
		}
	}
	c.metrics = nil
	return p.Proto(), nil
}
