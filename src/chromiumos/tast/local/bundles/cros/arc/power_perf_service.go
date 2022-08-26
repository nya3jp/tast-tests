// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"os"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

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
	s           *testing.ServiceState
	cancelSleep context.CancelFunc
	sleepDone   chan error
	cleanupCtx  context.Context
}

func (c *PowerPerfService) PowerSetup(ctx context.Context, request *arcpb.PowerSetupRequest) (*emptypb.Empty, error) {
	if c.cancelSleep != nil || c.sleepDone != nil {
		return nil, errors.New("call PowerCleanup before calling PowerSetup again")
	}

	testing.ContextLogf(ctx, "stack trace %s", string(debug.Stack()))

	// Slice of things to clean up if we don't succeed.
	var cleanup []func(ctx context.Context)
	defer func() {
		for _, c := range cleanup {
			// Defer to make the cleanup order correct, and also so that if one
			// panics, we still try to clean up the rest.
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
	_, err = power.SysfsBatteryPath(ctx)
	if err == nil {
		dischargeMode = setup.ForceBatteryDischarge
	} else if !errors.Is(power.ErrNoBattery, err) {
		// If it's ErrNoBattery, leave dischargeMode at NoBatteryDischarge.
		return nil, errors.Wrap(err, "failed to determine if there is a battery")
	}

	sup.Add(setup.PowerTest(ctx, tconn,
		setup.PowerTestOptions{Wifi: setup.DisableWifiInterfaces, NightLight: setup.DisableNightLight},
		setup.NewBatteryDischargeFromMode(dischargeMode),
	))
	if err := sup.Check(ctx); err != nil {
		return nil, errors.Wrap(err, "power perf setup failed")
	}

	// Wait until CPU is cooled down and idle.
	_, err = cpu.WaitUntilCoolDown(ctx, idleCoolDownConfig())
	if err != nil {
		return nil, errors.Wrap(err, "CPU failed to cool down")
	}
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return nil, errors.Wrap(err, "CPU failed to idle: ")
	}
	if err := arc.CheckNoDex2Oat(td); err != nil {
		return nil, errors.Wrap(err, "failed to verify dex2oat was not running")
	}

	// ctx is cancelled when this call returns, so if we want to clean up after
	// a timeout we need a new context.Context.
	sleepCtx, cancelSleep := context.WithCancel(context.Background())
	sleepDone := make(chan error)
	cleanupSuccess := cleanup
	// Don't cleanup everything
	cleanup = nil
	c.cancelSleep = cancelSleep
	c.sleepDone = sleepDone

	go func() {
		sleepErr := testing.Sleep(sleepCtx, request.Duration.AsDuration())
		func() {
			// Wrap cleanup in a function call so the deferred cleanup runs
			// before we signal sleepDone.
			cleanupCtx := c.cleanupCtx
			if cleanupCtx == nil {
				// Did not find a context forwarded from PowerCleanup.
				cleanupCtx = context.Background()
			}
			for _, clean := range cleanupSuccess {
				// Defer to make the cleanup order correct, and also so that if
				// one panics, we still try to clean up the rest.
				defer clean(cleanupCtx)
			}
		}()
		if errors.Is(sleepErr, context.Canceled) {
			// PowerCleanup cancelled sleepCtx, so everything is fine
			sleepDone <- nil
		} else if sleepErr != nil {
			sleepDone <- sleepErr
		} else {
			sleepDone <- errors.New("took too long to call PowerCleanup, timeout out")
		}
	}()

	return &emptypb.Empty{}, nil
}

func (c *PowerPerfService) PowerCleanup(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if c.cancelSleep == nil || c.sleepDone == nil {
		return nil, errors.New("call PowerCleanup only after a successful PowerSetup call")
	}
	// Forward our context so that cleanup gets logged properly.
	c.cleanupCtx = ctx

	c.cancelSleep()
	err := <-c.sleepDone
	c.cancelSleep = nil
	c.sleepDone = nil
	c.cleanupCtx = nil
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
