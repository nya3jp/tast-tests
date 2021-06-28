// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
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

// BatteryStatus informs about current battery percentage.
func (c *PowerService) BatteryStatus(ctx context.Context, _ *empty.Empty) (*pb.BatteryStatusResponse, error) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get DUT power status")
	}
	return &pb.BatteryStatusResponse{
		Percentage: status.BatteryDisplayPercent,
	}, nil
}

// BatteryCharge tries to charge DUT to the specified level within the provided timeout.
func (c *PowerService) BatteryCharge(ctx context.Context, req *pb.BatteryChargeRequest) (*empty.Empty, error) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get DUT power status")
	}

	if req.Percentage <= status.BatteryDisplayPercent {
		return &empty.Empty{}, nil
	}
	if !status.LinePowerConnected {
		return nil, errors.New("BatteryCharge requires power line for charging DUT")
	}

	ctxold := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 60*time.Second)
	defer cancel()

	// Stopping the list of services that influence power test in a unpredictable way.
	for _, job := range []string{"powerd", "update-engine", "vnc"} {
		cl, err := setup.DisableServiceIfExists(ctx, job)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to stop service %s", job)
		}
		defer cl(ctxold)
	}

	if percent, err := currentBrightnessPercentage(ctx); err != nil {
		// No need to fail, backlight_tool may not be available [dut without display]
		testing.ContextLog(ctx, "Failed to control screen brightness: ", err)
	} else {
		if err := setBrightnessPercentage(ctx, 10.0); err != nil {
			return nil, errors.Wrap(err, "unable to change dut screen brightness")
		}
		defer func(ctx context.Context) {
			if err := setBrightnessPercentage(ctx, percent); err != nil {
				testing.ContextLog(ctx, "Failed to switch back screen brightness: ", err)
			}
		}(ctxold)
	}

	var duration time.Duration
	if t, ok := ctx.Deadline(); ok {
		duration = t.UTC().Sub(time.Now().UTC())
	} else {
		duration = time.Hour
	}

	go periodicLog(ctx, time.Minute)
	if err := testing.Poll(ctx, func(context.Context) error {
		if status, err = power.GetStatus(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get device power status"))
		}
		if status.BatteryDischarging {
			return testing.PollBreak(errors.Wrap(err, "power line isn't connected, can't charge DUT"))
		}

		if status.BatteryDisplayPercent < req.Percentage {
			return errors.New("charging")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  duration,
		Interval: time.Second,
	}); err != nil {
		return nil, errors.Wrapf(err, "unable to charge the battery to %f percent in %s", req.Percentage, duration)
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

	ctxold := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	if percent, err := currentBrightnessPercentage(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to control screen brightness: ", err)
	} else {
		if err := setBrightnessPercentage(ctx, 99.0); err != nil {
			return nil, errors.Wrap(err, "unable to change dut screen brightness")
		}
		defer func(ctx context.Context) {
			if err := setBrightnessPercentage(ctx, percent); err != nil {
				testing.ContextLog(ctx, "Failed to switch back screen brightness: ", err)
			}
		}(ctxold)
	}

	cl, err := setup.SetKeyboardBrightness(ctx, 100)
	if err != nil {
		testing.ContextLog(ctx, "Failed to maximize brightness of backlit keyboard: ", err)
	}
	defer cl(ctxold)

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
	defer conn.CloseTarget(ctxold)
	defer conn.Close()

	var duration time.Duration
	if t, ok := ctx.Deadline(); ok {
		duration = t.UTC().Sub(time.Now().UTC())
	} else {
		duration = time.Hour
	}

	go periodicLog(ctx, time.Minute)
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
		Timeout:  duration,
		Interval: time.Second,
	}); err != nil {
		return nil, errors.Wrapf(err, "unable to drain the battery to %f percent in %s", req.Percentage, duration)
	}

	return &empty.Empty{}, nil
}

func currentBrightnessPercentage(ctx context.Context) (float64, error) {
	out, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness_percent").Output()
	if err != nil {
		return 0, err
	}

	// Sometime backlight_tool prints some log message along with the battery percentage. We are interested in the last line
	splits := strings.Split(strings.Trim(string(out), " \n"), "\n")

	percent, err := strconv.ParseFloat(splits[len(splits)-1], 64)
	if err != nil {
		return 0, err
	}

	return percent, nil
}

func setBrightnessPercentage(ctx context.Context, percentage float64) error {
	arg := fmt.Sprintf("--set_brightness_percent=%f", percentage)
	return testexec.CommandContext(ctx, "backlight_tool", arg).Run()
}

func periodicLog(ctx context.Context, d time.Duration) {
	status, err := power.GetStatus(ctx)
	if err != nil {
		return
	}
	cur := status.BatteryDisplayPercent
	for {
		select {
		case <-time.After(d):
			status, err = power.GetStatus(ctx)
			if err != nil {
				return
			}
			testing.ContextLogf(ctx, "Current charge: %f. Delta: %f", status.BatteryDisplayPercent, math.Abs(status.BatteryDisplayPercent-cur))
			cur = status.BatteryDisplayPercent
		case <-ctx.Done():
			return
		}
	}
}
