// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/power"
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
	s   *testing.ServiceState
	obj dbus.BusObject
}

// setBrightnessPercentage sets DUT screen brightness to the specified value and returns a callback to revert to original.
func (c *PowerService) setBrightnessPercentage(ctx context.Context, value float64) (func(context.Context) error, error) {
	const (
		dbusName      = "org.chromium.PowerManager"
		dbusPath      = "/org/chromium/PowerManager"
		dbusInterface = "org.chromium.PowerManager"
	)
	var err error

	if c.obj == nil {
		if _, c.obj, err = dbusutil.Connect(ctx, dbusName, dbusPath); err != nil {
			return nil, errors.Wrap(err, "failed to connect to dbus")
		}
	}

	call := c.obj.CallWithContext(ctx, dbusInterface+".GetScreenBrightnessPercent", 0)
	if call.Err != nil {
		return nil, errors.Wrap(err, "failed to call dbus method GetScreenBrightnessPercent")
	}

	var prev float64
	if err = call.Store(&prev); err != nil {
		return nil, errors.Wrap(err, "failed to store dbus method call response")
	}

	if err = dbusutil.CallProtoMethod(ctx, c.obj, dbusInterface+".SetScreenBrightness",
		&pmpb.SetBacklightBrightnessRequest{
			Percent: &value,
		}, nil); err != nil {
		return nil, errors.Wrapf(err, "unable to alter screen brightness from %f to %f", prev, value)
	}

	return func(ctx context.Context) error {
		if err = dbusutil.CallProtoMethod(ctx, c.obj, dbusInterface+".SetScreenBrightness",
			&pmpb.SetBacklightBrightnessRequest{
				Percent: &prev,
			}, nil); err != nil {
			return errors.Wrapf(err, "failed to reset screen brightness from %f to %f", value, prev)
		}
		return nil
	}, nil
}

// BatteryStatus informs about current battery percentage, charging state etc.
func (c *PowerService) BatteryStatus(ctx context.Context, _ *empty.Empty) (*pb.BatteryStatusResponse, error) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire DUT power status")
	}

	return &pb.BatteryStatusResponse{
		Percentage: status.BatteryDisplayPercent,
		OnAc:       status.LinePowerConnected,
		Charging:   !status.BatteryDischarging,
	}, nil
}

// BatteryCharge charges DUT to the specified percentage.
func (c *PowerService) BatteryCharge(ctx context.Context, req *pb.BatteryChargeRequest) (*empty.Empty, error) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire DUT power status")
	}

	if req.Percentage <= status.BatteryDisplayPercent {
		return &empty.Empty{}, nil
	}
	if err := testing.Poll(ctx, func(context.Context) error {
		// To tackle the micro delay between switching servo role & DUT status update
		status, err = power.GetStatus(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to acquire DUT power status"))
		}
		if !status.LinePowerConnected {
			return errors.New("BatteryCharge requires power line for charging DUT")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  6 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return nil, err
	}

	ctxold := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	restore, err := c.setBrightnessPercentage(ctx, 10)
	if err != nil {
		return nil, err
	}
	defer restore(ctxold)

	if err := testing.Poll(ctx, func(context.Context) error {
		if status, err = power.GetStatus(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get device power status"))
		}
		if status.BatteryDischarging {
			return testing.PollBreak(errors.Wrap(err, "power line isn't connected, can't charge DUT"))
		}

		if status.BatteryDisplayPercent == 100.0 {
			return nil
		}
		if status.BatteryDisplayPercent < req.Percentage {
			return errors.New("charging")
		}
		return nil
	}, &testing.PollOptions{
		Interval: time.Second,
	}); err != nil {
		return nil, errors.Wrapf(err, "unable to charge the battery to %f percent", req.Percentage)
	}

	return &empty.Empty{}, nil
}

// BatteryDrain drains DUT to the specified percentage.
func (c *PowerService) BatteryDrain(ctx context.Context, req *pb.BatteryDrainRequest) (*empty.Empty, error) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire DUT power status")
	}

	ctxold := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	restore, err := c.setBrightnessPercentage(ctx, 100)
	if err != nil {
		return nil, err
	}
	defer restore(ctxold)

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	defer cr.Close(ctxold)

	const url = "https://crospower.page.link/power_BatteryDrain"
	// Rendering a WebGL website to consume power quickly.
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		testing.ContextLog(ctx, "Failed to render webpage: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctxold)

	if err := testing.Poll(ctx, func(context.Context) error {
		if status, err = power.GetStatus(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get device power status"))
		}
		if status.LinePowerConnected {
			return testing.PollBreak(errors.New("BatteryDrain requires DUT to be disconnected from power source"))
		}

		if status.BatteryDisplayPercent > req.Percentage {
			return errors.New("discharging")
		}
		return nil
	}, &testing.PollOptions{
		Interval: time.Second,
	}); err != nil {
		return nil, errors.Wrapf(err, "unable to drain the battery to %f percent", req.Percentage)
	}

	return &empty.Empty{}, nil
}
