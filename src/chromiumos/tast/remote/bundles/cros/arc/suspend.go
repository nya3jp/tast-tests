// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

type testArgsForSuspend struct {
	suspendDurationSeconds          int
	suspendDurationAllowanceSeconds float64
	numTrials                       int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Suspend,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks the behavior of ARC around suspend/resume",
		Contacts: []string{
			"hikalium@chromium.org",
			"cros-platform-kernel-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		SoftwareDeps: []string{
			"chrome",
			"android_vm",
			"virtual_susupend_time_injection",
			"no_qemu", /* TODO(b/209400676): Remove this once the issue on betty is fixed. */
		},
		ServiceDeps: []string{"tast.cros.arc.SuspendService"},
		Timeout:     24 * 60 * time.Minute,
		Params: []testing.Param{{
			Name: "s10c1",
			Val: testArgsForSuspend{
				suspendDurationSeconds:          10,
				suspendDurationAllowanceSeconds: 0.1,
				numTrials:                       1,
			},
		}, {
			Name: "s10c100",
			Val: testArgsForSuspend{
				suspendDurationSeconds:          10,
				suspendDurationAllowanceSeconds: 0.1,
				numTrials:                       100,
			},
		}, {
			Name: "s10c10000",
			Val: testArgsForSuspend{
				suspendDurationSeconds:          10,
				suspendDurationAllowanceSeconds: 0.1,
				numTrials:                       10000,
			},
		}, {
			Name: "s120c5",
			Val: testArgsForSuspend{
				suspendDurationSeconds:          120, /* Longer than CONFIG_RCU_CPU_STALL_TIMEOUT */
				suspendDurationAllowanceSeconds: 0.1,
				numTrials:                       5,
			},
		}, {
			Name: "s600c2",
			Val: testArgsForSuspend{
				suspendDurationSeconds:          600, /* Long enough to trigger watchdog timeouts */
				suspendDurationAllowanceSeconds: 0.1,
				numTrials:                       2,
			},
		}},
	})
}

type clockDiffs struct {
	bootDiff time.Duration
	monoDiff time.Duration
}

type dutClockDiffs struct {
	host clockDiffs
	arc  clockDiffs
}

func readClocks(ctx context.Context, s *testing.State, params *arcpb.SuspendServiceParams) *arcpb.GetClockValuesResponse {
	// Establish an rpc connection again since RPC connections can be disconnected
	// when the DUT is suspended for a long time.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewSuspendServiceClient(cl.Conn)
	res, err := service.GetClockValues(ctx, params)
	if err != nil {
		s.Fatal("SuspendService.GetClockValues returned an error: ", err)
	}
	return res
}

func calcClockDiffs(t0, t1 *arcpb.ClockValues) (clockDiffs, error) {
	t0Boottime, err := ptypes.Duration(t0.ClockBoottime)
	if err != nil {
		return clockDiffs{}, errors.Wrap(err, "failed to convert t0.ClockBoottime")
	}
	t1Boottime, err := ptypes.Duration(t1.ClockBoottime)
	if err != nil {
		return clockDiffs{}, errors.Wrap(err, "failed to convert t1.ClockBoottime")
	}
	t0Monotonic, err := ptypes.Duration(t0.ClockMonotonic)
	if err != nil {
		return clockDiffs{}, errors.Wrap(err, "failed to convert t0.ClockMonotonic")
	}
	t1Monotonic, err := ptypes.Duration(t1.ClockMonotonic)
	if err != nil {
		return clockDiffs{}, errors.Wrap(err, "failed to convert t1.ClockMonotonic")
	}
	return clockDiffs{
		bootDiff: time.Duration(t1Boottime - t0Boottime),
		monoDiff: time.Duration(t1Monotonic - t0Monotonic),
	}, nil
}

func calcDUTClockDiffs(t0, t1 *arcpb.GetClockValuesResponse) (dutClockDiffs, error) {
	host, err := calcClockDiffs(t0.Host, t1.Host)
	if err != nil {
		return dutClockDiffs{}, errors.Wrap(err, "failed to calc host diff")
	}
	arc, err := calcClockDiffs(t0.Arc, t1.Arc)
	if err != nil {
		return dutClockDiffs{}, errors.Wrap(err, "failed to calc arc diff")
	}
	return dutClockDiffs{
		host: host,
		arc:  arc,
	}, nil
}

func suspendDUT(ctx context.Context, s *testing.State, seconds int) {
	s.Logf("Suspending DUT for %d seconds", seconds)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(seconds+30)*time.Second)
	defer cancel()
	if err := s.DUT().Conn().CommandContext(ctx,
		"suspend_stress_test", "-c", "1",
		"--suspend_min", strconv.Itoa(seconds),
		"--suspend_max", strconv.Itoa(seconds),
		"--nopm_print_times").Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("Failed to suspend: ", err)
	}
	s.Log("Resumed")
}

func Suspend(ctx context.Context, s *testing.State) {
	args := s.Param().(testArgsForSuspend)

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	service := arc.NewSuspendServiceClient(cl.Conn)
	params, err := service.Prepare(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("SuspendService.Prepare returned an error: ", err)
	}
	defer func() {
		_, err = service.Finalize(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("SuspendService.Finalize returned an error: ", err)
		}
	}()

	for i := 0; i < args.numTrials; i++ {
		s.Logf("Trial %d/%d", i+1, args.numTrials)

		t0 := readClocks(ctx, s, params)
		suspendDUT(ctx, s, args.suspendDurationSeconds)
		t1 := readClocks(ctx, s, params)

		diff, err := calcDUTClockDiffs(t0, t1)
		if err != nil {
			s.Fatal("Failed to calc clock diffs: ", err)
		}
		hostSuspendDuration := diff.host.bootDiff - diff.host.monoDiff
		s.Log("Host suspended ", hostSuspendDuration)
		arcSuspendDuration := diff.arc.bootDiff - diff.arc.monoDiff
		if math.Abs((arcSuspendDuration - hostSuspendDuration).Seconds()) > args.suspendDurationAllowanceSeconds {
			s.Logf("Suspend time was not injected to ARC properly, got %v, want %v", arcSuspendDuration, hostSuspendDuration)
		}
		s.Logf("OK: %v of suspend time was injected to ARC", arcSuspendDuration)
	}
}
