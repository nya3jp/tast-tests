// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	pb "chromiumos/tast/services/cros/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterPowerServiceServer(srv, &PowerService{s: s})
		},
	})
}

// PowerService implements tast.cros.power.PowerService
type PowerService struct { // NOLINT
	s *testing.ServiceState
}

// BatteryCharge tries to charge DUT to the specified level within the provided timeout.
func (c *PowerService) BatteryCharge(ctx context.Context, req *pb.BatteryChargeRequest) (*empty.Empty, error) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get DUT power status")
	}
	if !status.LinePowerConnected {
		return nil, errors.New("BatteryCharge requires power line for charging DUT")
	}

	cls := new(testing.Closers)
	defer cls.CloseAll(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
	defer cancel()

	// Stopping the unnecessary service for faster charging
	for _, job := range []string{"powerd", "update-engine", "vnc", "ui"} {
		cl, err := setup.DisableServiceIfExists(ctx, job)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to stop service %s", job)
		}
		cls.Append(cl)
	}

	// Dimming DUT screen
	cl, err := setup.SetBacklightLux(ctx, 0)
	if err != nil {
		// No need to fail
		testing.ContextLog(ctx, "Failed to put screen brightness to 0: ", err)
	}
	cls.Append(cl)

	// Some calculation on charge capacity, current charge and target charge
	cap := status.BatteryChargeFull
	if req.UseDesignChargeCapacity {
		cap = status.BatteryChargeFullDesign
	}
	if cap == 0.0 {
		return nil, errors.New("unable to determine DUT charge capacity")
	}

	cur, tgt := status.BatteryCharge, 0.0
	if req.PercentTargetCharge != 0.0 {
		tgt = cap * float64(req.PercentTargetCharge) / 100
	} else {
		tgt = cur + cap*float64(req.PercentChargeToAdd)/100
	}

	if tgt > cap {
		tgt = cap // trimming if it exceeds capacity
	}

	testing.ContextLogf(ctx, "Current charge: %f. Target charge: %f", cur, tgt)

	if err := testing.Poll(ctx, func(context.Context) error {
		if status, err = power.GetStatus(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get device power status"))
		}
		if status.BatteryDischarging {
			return testing.PollBreak(errors.Wrap(err, "power line isn't connected, can't charge DUT"))
		}

		testing.ContextLogf(ctx, "Current charge: %f. Charge added: %f", status.BatteryDisplayPercent, (status.BatteryCharge - cur))

		cur = status.BatteryCharge
		if cur < tgt {
			return errors.New("ongoing charging")
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  time.Duration(req.Timeout) * time.Second,
		Interval: 60 * time.Second,
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// BatteryDrain tries to drain DUT to the specified percentage within the provided timeout.
func (c *PowerService) BatteryDrain(ctx context.Context, req *pb.BatteryDrainRequest) (*empty.Empty, error) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get DUT power status")
	}
	if status.LinePowerConnected {
		return nil, errors.New("BatteryDrain requires DUT to be disconnected from power source")
	}

	cls := new(testing.Closers)
	defer cls.CloseAll(ctx)

	cl, err := setup.SetBacklightLux(ctx, 700) // TODO: find the max lux possible
	if err != nil {
		testing.ContextLog(ctx, "Failed to put screen brightness to 700 lux: ", err)
	}
	cls.Append(cl)

	kcl, err := setup.SetKeyboardBrightness(ctx, 100)
	if err != nil {
		testing.ContextLog(ctx, "Failed to maximize brightness of backlit keyboard: ", err)
	}
	cls.Append(kcl)

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	cls.Append(cr.Close)

	const url = "https://crospower.page.link/power_BatteryDrain"
	// Rendering a WebGL website to consume power quickly.
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		testing.ContextLog(ctx, "Failed to render webpage: ", err)
	}
	defer conn.Close()

	cur := status.BatteryCharge
	if err := testing.Poll(ctx, func(context.Context) error {
		if status, err = power.GetStatus(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get device power status"))
		}
		if status.LinePowerConnected {
			return testing.PollBreak(errors.New("BatteryDrain requires DUT to be disconnected from power source"))
		}

		testing.ContextLogf(ctx, "Current battery: %f. Battery drained: %f", status.BatteryDisplayPercent, (cur - status.BatteryCharge))

		cur = status.BatteryCharge
		if status.BatteryDisplayPercent > float64(req.DrainToPercent) {
			return errors.New("ongoing discharging")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  time.Duration(req.Timeout) * time.Second,
		Interval: 60 * time.Second,
	}); err != nil {
		return nil, errors.Wrapf(err, "unable to drain the battery to %f percent in %d seconds", req.DrainToPercent, req.Timeout)
	}

	return &empty.Empty{}, nil
}
