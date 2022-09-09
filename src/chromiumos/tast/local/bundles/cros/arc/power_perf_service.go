// Copyright 2022 The ChromiumOS Authors
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
	s        *testing.ServiceState
	metrics  []perf.TimelineDatasource
	cleanups []func(ctx context.Context)
}

func (c *PowerPerfService) Setup(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if c.cleanups != nil {
		// We didn't clean up from the last time, do it now.
		for _, c := range c.cleanups {
			// Defer to make the cleanups order correct, and also so that if one
			// panics, we still try to clean up the rest.
			defer c(ctx)
		}
		c.cleanups = nil
		return nil, errors.New("call Powercleanups before calling PowerSetup again")
	}

	// Slice of things to clean up if we don't succeed.
	var cleanups []func(ctx context.Context)
	defer func(ctx context.Context) {
		for _, c := range cleanups {
			// NB: deferred calls run in the reverse order
			defer c(ctx)
		}
	}(ctx)

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
	cleanups = append(cleanups, func(ctx context.Context) {
		if err := cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "PowerSetup cleanup failure")
		}
	})

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}

	// Set up Android.
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	cleanups = append(cleanups, func(ctx context.Context) { os.RemoveAll(td) })

	a, err := arc.New(ctx, td)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	cleanups = append(cleanups, func(ctx context.Context) { a.Close(ctx) })

	// Configure DUT for power test.
	sup, cleanupPower := setup.New("power perf")
	cleanups = append(cleanups, func(ctx context.Context) { cleanupPower(ctx) })

	dischargeMode := setup.NoBatteryDischarge
	if batteryPath, err := power.SysfsBatteryPath(ctx); err == nil {
		dischargeMode = setup.ForceBatteryDischarge
		// There is a battery, make sure it's charged before starting the test.
		testing.ContextLog(ctx, "Waiting for battery to charge")
		if err := power.WaitForCharge(ctx, batteryPath, 0.95, 30*time.Minute); err != nil {
			return nil, err
		}
		testing.ContextLog(ctx, "Battery now charged")
	} else if !errors.Is(err, power.ErrNoBattery) {
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

	// Don't cleanups everything
	c.cleanups = cleanups
	cleanups = nil

	return &emptypb.Empty{}, nil
}

func (c *PowerPerfService) StartMeasurement(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if c.metrics != nil {
		return nil, errors.New("already measuring metrics")
	}

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

	return &emptypb.Empty{}, nil
}

func (c *PowerPerfService) StopMeasurement(ctx context.Context, _ *emptypb.Empty) (*perfpb.Values, error) {
	metrics := c.metrics
	c.metrics = nil
	if metrics == nil {
		return nil, errors.New("no metrics to stop measuring")
	}

	p := perf.NewValues()
	if len(metrics) > 0 {
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
	return p.Proto(), nil
}

func (c *PowerPerfService) Cleanup(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if c.metrics != nil {
		testing.ContextLog(ctx, "Warning, PowerPerfService.StartMeasurement but no StopMeasurement")
		c.metrics = nil
	}
	if c.cleanups == nil {
		return nil, errors.New("nothing to clean up")
	}
	for _, c := range c.cleanups {
		defer c(ctx)
	}
	c.cleanups = nil
	return &emptypb.Empty{}, nil
}
