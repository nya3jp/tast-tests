// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

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
		Func: Suspend,
		Desc: "Checks the behavior of ARC around suspend/resume",
		Contacts: []string{
			"hikalium@chromium.org",
			"cros-platform-kernel-core@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "android_vm", "virtual_susupend_time_injection"},
		ServiceDeps:  []string{"tast.cros.arc.SuspendService"},
		Timeout:      60 * time.Minute,
		Params: []testing.Param{{
			Name: "s10c100",
			Val: testArgsForSuspend{
				suspendDurationSeconds:          10,
				suspendDurationAllowanceSeconds: 0.1,
				numTrials:                       100,
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
				suspendDurationSeconds:          600, /* Long enough than watchdog timeouts */
				suspendDurationAllowanceSeconds: 0.1,
				numTrials:                       2,
			},
		}},
	})
}

type clockDiffs struct {
	bootDiff time.Duration
	monoDiff time.Duration
	tscDiff  int64
}

type dutClockDiffs struct {
	host clockDiffs
	arc  clockDiffs
}

func readClocks(ctx context.Context, s *testing.State, params *arcpb.SuspendServiceParams) *arcpb.GetClockValuesResponse {
	// Establish an rpc connection again
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
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

func calcClockDiffs(t0, t1 *arcpb.ClockValues) clockDiffs {
	return clockDiffs{
		bootDiff: time.Duration(t1.ClockBoottimeNs-t0.ClockBoottimeNs) * time.Nanosecond,
		monoDiff: time.Duration(t1.ClockMonotonicNs-t0.ClockMonotonicNs) * time.Nanosecond,
		tscDiff:  t1.Tsc - t0.Tsc,
	}
}

func calcDUTClockDiffs(t0, t1 *arcpb.GetClockValuesResponse) dutClockDiffs {
	return dutClockDiffs{
		host: calcClockDiffs(t0.Host, t1.Host),
		arc:  calcClockDiffs(t0.Arc, t1.Arc),
	}
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
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	service := arc.NewSuspendServiceClient(cl.Conn)
	params, err := service.Prepare(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("SuspendService.Prepare returned an error: ", err)
	}

	for i := 0; i < args.numTrials; i++ {
		s.Logf("Trial %d/%d", i+1, args.numTrials)

		t0 := readClocks(ctx, s, params)
		suspendDUT(ctx, s, args.suspendDurationSeconds)
		t1 := readClocks(ctx, s, params)

		diff := calcDUTClockDiffs(t0, t1)
		hostSuspendDuration := diff.host.bootDiff - diff.host.monoDiff
		s.Log("Host suspended", hostSuspendDuration)
		arcSuspendDuration := diff.arc.bootDiff - diff.arc.monoDiff
		if math.Abs((arcSuspendDuration - hostSuspendDuration).Seconds()) > args.suspendDurationAllowanceSeconds {
			s.Fatalf("Suspend time was not injected to ARC properly, got %f, want %f", arcSuspendDuration, hostSuspendDuration)
		}
		s.Logf("OK: %v seconds of suspend time was injected to ARC", arcSuspendDuration)
	}

}
