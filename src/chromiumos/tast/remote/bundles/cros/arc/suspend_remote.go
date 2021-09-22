// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SuspendRemote,
		Desc: "Checks the behavior of ARC around suspend/resume",
		Contacts: []string{
			"hikalium@chromium.org",
			"cros-platform-kernel-core@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "android_vm" /*, "virtual_susupend_injection"*/},
		ServiceDeps:  []string{"tast.cros.arc.SuspendRemoteService"},
		Timeout:      30 * time.Minute,
	})
}

type readclocksTimespec struct {
	Seconds     int64 `json:"tv_sec"`
	NanoSeconds int64 `json:"tv_nsec"`
}

type readclocksOutput struct {
	Boot readclocksTimespec `json:"CLOCK_BOOTTIME"`
	Mono readclocksTimespec `json:"CLOCK_MONOTONIC"`
	TSC  int64
}

type clocks struct {
	hostBoot  time.Time
	hostMono  time.Time
	guestBoot time.Time
	guestMono time.Time
}

func readClocks(ctx context.Context, s *testing.State) (clocks, error) {
	var c clocks
	s.Log("Reading Host Clock")
	output, err := s.DUT().Conn().CommandContext(ctx, "/usr/local/libexec/tast/helpers/local/cros/arc.Suspend.readclocks").Output()
	if err != nil {
		s.Fatal("Failed to run cmd: ", err)
	}
	var hostClocks readclocksOutput
	err = json.Unmarshal(output, &hostClocks)
	if err != nil {
		s.Fatal("failed to parse readclocks output: ", err)
	}
	c.hostMono = time.Unix(hostClocks.Mono.Seconds, hostClocks.Mono.NanoSeconds)
	c.hostBoot = time.Unix(hostClocks.Boot.Seconds, hostClocks.Boot.NanoSeconds)
	return c, nil
}

func SuspendRemote(ctx context.Context, s *testing.State) {
	d := s.DUT()

	c0, err := readClocks(ctx, s)
	if err != nil {
		s.Fatal("Failed to read clocks: ", err)
	}
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewSuspendRemoteServiceClient(cl.Conn)
	res, err := service.GetClockValues(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("SuspendRemoteService.GetClockValues returned an error: ", err)
	}
	s.Log("%v", res)

	s.Log("Suspending DUT")
	if err := d.Conn().CommandContext(ctx, "powerd_dbus_suspend", "-wakeup_timeout=10").Run(ssh.DumpLogOnError); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		s.Fatal("Failed to suspend: ", err)
	}

	c1, err := readClocks(ctx, s)
	if err != nil {
		s.Fatal("Failed to read clocks: ", err)
	}

	hostBootDiff := c1.hostBoot.Sub(c0.hostBoot)
	hostMonoDiff := c1.hostMono.Sub(c0.hostMono)
	hostSuspendSeconds := hostBootDiff.Seconds() - hostMonoDiff.Seconds()

	s.Log(hostBootDiff, hostMonoDiff, hostSuspendSeconds)

}
